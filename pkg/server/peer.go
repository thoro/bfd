package server

import (
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/gofrs/uuid"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/packet/bfd"
	"github.com/golang/glog"
)

const (
	// 20 seconds defined as micro seconds
	OFFLINE_TIMEOUT = 20 * 1000 * 1000
)

var ErrInvalidAddress = errors.New("Invalid address passed")
var ErrInvalidPort = errors.New("Invalid port passed, should be between 1 and 65535")
var ErrSessionAdminDown = errors.New("Peer is state is admin down")
var ErrNotImplemented = errors.New("Function not implemented")

type Peer struct {
	sync.RWMutex

	uuid []byte

	Name       string
	Address    *net.UDPAddr
	SourcePort int

	// Set up interval
	Interval uint32

	local  *PeerState
	remote *PeerState

	AuthType             bfd.AuthenticationType // 0 = no authentication
	ReceivedAuthSequence uint32
	XmitAuthSeq          uint32 // needs to be initialized with random 32 bit value
	AuthSequenceKnown    uint32 // reset to 0 if no packets are received in 2 * DetectionTime (Interval * Multiplier)
	PollActive           bool
	IsMultiHop           bool

	// control channels
	conn       Connection  // sending udp connection
	ticker     *time.Timer // timer for control packets
	expiry     *time.Timer // timer for expiry of the session
	lastPacket time.Time
	control    chan bool
	updater    chan *mgmtOp

	watchers []*watcher
}

type mgmtOp struct {
	errCh chan error
	f     func() error
}

func NewPeer(address net.IP, port int) (*Peer, error) {

	if address == nil {
		return nil, ErrInvalidAddress
	}

	if port < 1 || port > 65535 {
		return nil, ErrInvalidPort
	}

	id, _ := uuid.NewV4()
	uuidBytes := id.Bytes()

	p := &Peer{
		uuid: uuidBytes,
		Address: &net.UDPAddr{
			IP:   address,
			Port: port,
		},

		// Initialize with a long time - will be updated by the config
		ticker: time.NewTimer(time.Duration(5) * time.Hour),
		expiry: time.NewTimer(time.Duration(5) * time.Hour),

		updater: make(chan *mgmtOp, 8),
	}

	// Setup an initial state
	p.local = &PeerState{
		sessionState:          bfd.Down,
		discriminator:         0,
		desiredMinTxInterval:  1000000,
		requiredMinRxInterval: 0,
		detectMultiplier:      1,
	}

	p.remote = &PeerState{
		sessionState:          bfd.Down,
		requiredMinRxInterval: 1,
	}

	return p, nil
}

func (p *Peer) NewPacket(poll bfd.Bool, final bfd.Bool) *bfd.ControlPacket {
	local := p.GetLocal()
	remote := p.GetRemote()

	return &bfd.ControlPacket{
		Version:                 1,
		State:                   local.sessionState,
		Poll:                    poll,
		Final:                   final,
		Demand:                  bfd.No,
		DetectMultiplier:        local.detectMultiplier,
		MyDiscriminator:         local.discriminator,
		YourDiscriminator:       remote.discriminator,
		DesiredMinTxInterval:    local.desiredMinTxInterval,
		RequiredMinRxInterval:   local.requiredMinRxInterval,
		RequiredMinEchoInterval: 0,
	}
}

func (p *Peer) Send(packet *bfd.ControlPacket) error {
	b, err := packet.MarshalBinary()

	_, err = p.conn.Write(b)

	if err != nil {
		glog.Infof("Error on write: %s", err)
	}

	return err
}

func (p *Peer) GetLocal() *PeerState {
	p.RLock()
	defer p.RUnlock()

	return p.local
}

func (p *Peer) GetRemote() *PeerState {
	p.RLock()
	defer p.RUnlock()

	return p.remote
}

func (p *Peer) GetUuid() []byte {
	return p.uuid
}

func (p *Peer) GetAuthenticationType() bfd.AuthenticationType {
	p.RLock()
	defer p.RUnlock()

	return p.AuthType
}

func (p *Peer) SetDesiredMinTxInterval(desiredMinTx uint32) {
	state := p.GetLocal()

	/*
		If bfd.DesiredMinTxInterval is increased and bfd.SessionState is Up,
		the actual transmission interval used MUST NOT change until the Poll
		Sequence described above has terminated.  This is to ensure that the
		remote system updates its Detection Time before the transmission
		interval increases.
	*/
	if desiredMinTx > state.desiredMinTxInterval && state.sessionState == bfd.Up {
		// delayed apply
		panic("Not implemented")
	} else {
		p.ApplyLocalState([]PeerStateUpdate{setDesiredMinTxInterval(desiredMinTx)})
	}
}

func (p *Peer) SetRequiredMinRxInterval(requiredMinRx uint32) {
	state := p.GetLocal()

	/*
		If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up,
		the previous value of bfd.RequiredMinRxInterval MUST be used when
		calculating the Detection Time for the remote system until the Poll
		Sequence described above has terminated.  This is to ensure that the
		remote system is transmitting packets at the higher rate (and those
		packets are being received) prior to the Detection Time being
		reduced.
	*/
	if requiredMinRx < state.requiredMinRxInterval && state.sessionState == bfd.Up {
		// delayed apply
		panic("Not implemented")
	} else {
		p.ApplyLocalState([]PeerStateUpdate{setRequiredMinRxInterval(requiredMinRx)})
	}
}

func (p *Peer) SetDetectMultiplier(detectMultiplier uint8) {
	p.ApplyLocalState([]PeerStateUpdate{setDetectMultiplier(detectMultiplier)})
}

func (p *Peer) mgmt(f func() error) error {
	ch := make(chan error)

	p.updater <- &mgmtOp{
		f:     f,
		errCh: ch,
	}

	return <-ch
}

func (p *Peer) NotifyWatchers(state *api.PeerStateResponse) {
	for _, watcher := range p.watchers {
		watcher.Notify(state)
	}
}

func (p *Peer) Watch() *watcher {
	w := NewWatcher()
	w.peer = p

	p.Lock()
	p.watchers = append(p.watchers, w)
	p.Unlock()

	go w.loop()

	return w
}

func (p *Peer) ApplyLocalState(updates []PeerStateUpdate) {
	sessionStateUpdated := false

	p.mgmt(func() error {
		p.Lock()
		defer p.Unlock()
		old_state := p.local
		p.local = p.local.Clone(updates)

		if old_state.sessionState != p.local.sessionState {
			sessionStateUpdated = true

			if p.local.sessionState == bfd.Up {
				// Update the desiredMinTxInterval
				p.local = p.local.Clone([]PeerStateUpdate{setDesiredMinTxInterval(p.Interval)})
			}
		}

		return nil
	})

	if sessionStateUpdated {
		p.NotifyWatchers(&api.PeerStateResponse{
			Local:  p.GetLocal().ToApi(),
			Remote: p.GetRemote().ToApi(),
		})
	}
}

func (p *Peer) ApplyRemoteState(updates []PeerStateUpdate) {
	p.mgmt(func() error {
		p.Lock()
		defer p.Unlock()
		p.remote = p.remote.Clone(updates)

		return nil
	})
}

func (p *Peer) Enable() {
	local := p.GetLocal()

	if local.sessionState == bfd.AdminDown {
		p.ApplyLocalState([]PeerStateUpdate{setSessionState(bfd.Down)})
	}
}

func (p *Peer) Disable() {
	local := p.GetLocal()

	if local.sessionState != bfd.AdminDown {
		p.ApplyLocalState([]PeerStateUpdate{setSessionState(bfd.AdminDown)})
	}
}

func (p *Peer) Start() {
	p.control = make(chan bool, 1)

	go p.handleUpdates()
	go p.handle()
}

func (p *Peer) Shutdown() {
	close(p.control)
}

func (p *Peer) scheduleExpiry(interval uint32) {
	p.Lock()
	p.expiry.Reset(time.Duration(interval) * time.Microsecond)
	p.Unlock()
}

// scheduleSend schedules the next send operation
func (p *Peer) scheduleSend(interval uint32) {
	p.Lock()
	p.ticker.Reset(time.Duration(interval) * time.Microsecond)
	p.Unlock()
}

// handleUpdates handles all updates that need to be applied to the state of a peer
func (p *Peer) handleUpdates() {
	for {
		select {
		case mgmtOp := <-p.updater:
			mgmtOp.errCh <- mgmtOp.f()
		case <-p.control:
			return
		}
	}
}

func (peer *Peer) handle() {

	for {
		select {
		case <-peer.ticker.C:
			local := peer.GetLocal()
			remote := peer.GetRemote()

			if remote.requiredMinRxInterval > 0 {
				// send a packet
				packet := peer.NewPacket(bfd.No, bfd.No)
				peer.Send(packet)
			}

			if local.sessionState != bfd.Up {
				// we need to unlock since otherwise we got a deadlock here
				peer.SetDesiredMinTxInterval(1000000)
				local = local.Clone([]PeerStateUpdate{setDesiredMinTxInterval(1000000)})
			}

			/*
				RFC5880 6.8.7
				The periodic transmission of BFD Control packets MUST be jittered on
				a per-packet basis by up to 25%, that is, the interval MUST be
				reduced by a random value of 0 to 25%, in order to avoid self-
				synchronization with other systems on the same subnetwork.  Thus, the
				average interval between packets will be roughly 12.5% less than that
				negotiated.
			*/

			preSendInterval := max(local.desiredMinTxInterval, remote.requiredMinRxInterval)
			sendInterval := preSendInterval - (preSendInterval * uint32(rand.Intn(25)) / 100)

			/*
				RFC5880 6.8.7
				If bfd.DetectMult is equal to 1, the interval between transmitted BFD
				Control packets MUST be no more than 90% of the negotiated
				transmission interval, and MUST be no less than 75% of the negotiated
				transmission interval.  This is to ensure that, on the remote system,
				the calculated Detection Time does not pass prior to the receipt of
				the next BFD Control packet.
			*/
			if local.detectMultiplier == 1 {
				sendInterval = min(sendInterval, preSendInterval*90/100)
			}

			peer.scheduleSend(sendInterval)

		// expiry placed here, since it's 1 goroutine per neighbor
		case <-peer.expiry.C:
			/*
				If Demand mode is not active, and a period of time equal to the
				Detection Time passes without receiving a BFD Control packet from the
				remote system, and bfd.SessionState is Init or Up, the session has
				gone down -- the local system MUST set bfd.SessionState to Down and
				bfd.LocalDiag to 1 (Control Detection Time Expired).
			*/
			local := peer.GetLocal()

			// glog.Infof("Expired: %v", time.Since(peer.lastPacket))

			if local.sessionState == bfd.Init || local.sessionState == bfd.Up {
				/*
						So long as the local system continues to transmit BFD Control
					    packets, the remote system is obligated to obey the value carried in
					    Required Min RX Interval.  If the remote system does not receive any
					    BFD Control packets for a Detection Time, it SHOULD reset
					    bfd.RemoteMinRxInterval to its initial value of 1 (per section 6.8.1,
					    since it is no longer required to maintain previous session state)
					    and then can transmit at its own rate.
				*/

				peer.ApplyLocalState([]PeerStateUpdate{
					setDiagnosticCode(bfd.ControlDetectionTimeExpired),
					setSessionState(bfd.Down),
				})

				peer.ApplyRemoteState([]PeerStateUpdate{
					setRequiredMinRxInterval(1),
				})
			}

		case <-peer.control:
			glog.Infof("Shutdown Peer")
			return
		}
	}
}

func (peer *Peer) handlePacket(packet *bfd.ControlPacket) error {
	/*
		If the A bit is set and no authentication is in use (bfd.AuthType is zero),
		the packet MUST be discarded.

		If the A bit is clear and authentication is in use (bfd.AuthType is nonzero),
		the packet MUST be discarded.
	*/

	if peer.GetAuthenticationType() != packet.GetAuthenticationType() {
		return bfd.ErrInvalidAuthenticationType
	}

	/*
		If the A bit is set, the packet MUST be authenticated under the
		rules of section 6.7, based on the authentication type in use
		(bfd.AuthType).  This may cause the packet to be discarded.
	*/
	if packet.GetAuthenticationType() != bfd.Reserved {
		// Do authentication
		return ErrNotImplemented
	}

	local := peer.GetLocal()
	remote := peer.GetRemote()

	// local updates
	lu := make([]PeerStateUpdate, 0)
	// remote updates
	ru := make([]PeerStateUpdate, 0)

	// Set bfd.RemoteState to the value of the State (Sta) field.
	ru = append(ru, setSessionState(packet.State))

	// Set bfd.RemoteDemandMode to the value of the Demand (D) bit.
	ru = append(ru, setDemandMode(packet.Demand == bfd.Yes))

	// Set bfd.RemoteDiscr to the value of My Discriminator.
	ru = append(ru, setDiscriminator(packet.MyDiscriminator))

	// Set bfd.RemoteMinRxInterval to the value of Required Min RX Interval.
	if remote.requiredMinRxInterval > packet.RequiredMinRxInterval {
		// define a new timeout for the ticker
		// maybe define a custom timer with the "end time"
		// or define a timer with 0 (for immediate change)

		// RFC5880 6.8.3
		// If this interval has already passed
		// since the last transmission (because the new interval is
		// significantly shorter), the local system MUST send the next periodic
		// BFD Control packet as soon as practicable.

		// TODO something?
		// peer.scheduleSend(peer.desiredMinTxInterval)
	}

	ru = append(ru, setRequiredMinRxInterval(packet.RequiredMinRxInterval))
	ru = append(ru, setDetectMultiplier(packet.DetectMultiplier))

	// If the Required Min Echo RX Interval field is zero, the transmission of Echo packets, if any, MUST cease.
	// => handle in sending code (Locks?!)

	/*
		If a Poll Sequence is being transmitted by the local system and
		the Final (F) bit in the received packet is set, the Poll Sequence
		MUST be terminated.
	*/
	if peer.PollActive && packet.Final == bfd.Yes {
		peer.Lock()
		peer.PollActive = false
		peer.Unlock()
	}

	// Update the transmit interval as described in section 6.8.2.
	// Update the Detection Time as described in section 6.8.4.

	// If bfd.SessionState is AdminDown
	//     Discard the packet

	if local.sessionState == bfd.AdminDown {
		return ErrSessionAdminDown
	}

	/*
		If received state is AdminDown
		  If bfd.SessionState is not Down
			  Set bfd.LocalDiag to 3 (Neighbor signaled
				  session down)
			  Set bfd.SessionState to Down

		Else
	*/
	if packet.State == bfd.AdminDown {
		if local.sessionState != bfd.Down {
			lu = append(lu, setDiagnosticCode(bfd.NeighborSignaledSessionDown))
			lu = append(lu, setSessionState(bfd.Down))
		}
	} else {
		/*
			If bfd.SessionState is Down
			  If received State is Down
				  Set bfd.SessionState to Init
			  Else if received State is Init
				  Set bfd.SessionState to Up
			Else if bfd.SessionState is Init
				If received State is Init or Up
					Set bfd.SessionState to Up

			Else (bfd.SessionState is Up)
				If received State is Down
					Set bfd.LocalDiag to 3 (Neighbor signaled
						session down)
					Set bfd.SessionState to Down
		*/
		if local.sessionState == bfd.Down {
			if packet.State == bfd.Down {
				lu = append(lu, setDiagnosticCode(bfd.NoDiagnostic))
				lu = append(lu, setSessionState(bfd.Init))
			} else if packet.State == bfd.Init {
				lu = append(lu, setDiagnosticCode(bfd.NoDiagnostic))
				lu = append(lu, setSessionState(bfd.Up))
			}
		} else if local.sessionState == bfd.Init {
			if packet.State == bfd.Init || packet.State == bfd.Up {
				lu = append(lu, setDiagnosticCode(bfd.NoDiagnostic))
				lu = append(lu, setSessionState(bfd.Up))
			}
		} else {
			if packet.State == bfd.Down {
				lu = append(lu, setDiagnosticCode(bfd.NeighborSignaledSessionDown))
				lu = append(lu, setSessionState(bfd.Down))
			}
		}
	}

	// Apply any changes locally and then on the peer
	local = local.Clone(lu)
	remote = remote.Clone(ru)

	peer.ApplyLocalState(lu)
	peer.ApplyRemoteState(ru)

	// Check to see if Demand mode should become active or not (see section 6.6).
	// => not supported for now

	/*
	  If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and
	  bfd.RemoteSessionState is Up, Demand mode is active on the remote
	  system and the local system MUST cease the periodic transmission
	  of BFD Control packets (see section 6.8.7).
	  => not supported for now
	*/

	/*
	  If bfd.RemoteDemandMode is 0, or bfd.SessionState is not Up, or
	  bfd.RemoteSessionState is not Up, Demand mode is not active on the
	  remote system and the local system MUST send periodic BFD Control
	  packets (see section 6.8.7).
	*/

	/*
	  If the Poll (P) bit is set, send a BFD Control packet to the
	  remote system with the Poll (P) bit clear, and the Final (F) bit
	  set (see section 6.8.7).
	*/
	if packet.Poll == bfd.Yes {
		// send final packet
		packet := peer.NewPacket(bfd.No, bfd.Yes)
		peer.Send(packet)
	}

	/*
	  If the packet was not discarded, it has been received for purposes
	  of the Detection Time expiration rules in section 6.8.4.
	*/

	/*
		In Asynchronous mode, the Detection Time calculated in the local
		system is equal to the value of Detect Mult received from the remote
		system, multiplied by the agreed transmit interval of the remote
		system (the greater of bfd.RequiredMinRxInterval and the last
		received Desired Min TX Interval).  The Detect Mult value is (roughly
		speaking, due to jitter) the number of packets that have to be missed
		in a row to declare the session to be down.
	*/
	negotiatedRx := max(local.requiredMinRxInterval, packet.DesiredMinTxInterval)

	// if we are up, extend the expiry to the correct detection time
	// else we need to clear it
	if local.sessionState == bfd.Up {
		detectionTime := negotiatedRx * uint32(remote.detectMultiplier)
		peer.scheduleExpiry(detectionTime)
	} else {
		peer.scheduleExpiry(OFFLINE_TIMEOUT)
	}

	return nil
}
