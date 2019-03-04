package server

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/golang/glog"
	"golang.org/x/net/ipv4"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/packet/bfd"
)

/*

https://tools.ietf.org/html/rfc5880
https://tools.ietf.org/html/rfc5881

*/

const (
	BFD_PORT = 3784
)

type BfdServer struct {
	sync.RWMutex

	Sessions map[uint32]*Peer

	conns map[string]*listener

	inbound  chan packet
	outbound chan packet

	control chan bool
	dialUDP func(network string, laddr, raddr *net.UDPAddr) (*net.UDPConn, error)
}

var ErrInvalidDetectionMultiplierSupplied = errors.New("Invalid Detection Multiplier supplied")
var ErrPeerNotFound = errors.New("Unable to find peer")
var ErrInvalidVersion = errors.New("Invalid Version")
var ErrInvalidDetectMultiplier = errors.New("Invalid detect multiplier")
var ErrInvalidMultiPoint = errors.New("Invalid Multipoint value")
var ErrInvalidMyDiscriminator = errors.New("Invalid MyDiscriminator value")
var ErrInvalidYourDiscriminator = errors.New("Invalid YourDiscriminator (=0) for state")
var ErrInvalidPacket = errors.New("Discarded Packet: Packet invalid")
var ErrYourDiscriminatorNotFound = errors.New("Discarded Packet: YourDiscriminator not found")
var ErrInvalidTTL = errors.New("Invalid TTL received")
var ErrInvalidIP = errors.New("Invalid IP passed")

type packet struct {
	addr   *net.UDPAddr
	packet *bfd.ControlPacket
}

type listener struct {
	conn    Connection
	control chan bool
}

func max(v1, v2 uint32) uint32 {
	if v1 < v2 {
		return v2
	}

	return v1
}

func min(v1, v2 uint32) uint32 {
	if v1 < v2 {
		return v1
	}

	return v2
}

func NewBfdServer() *BfdServer {
	s := &BfdServer{
		dialUDP:  net.DialUDP,
		Sessions: make(map[uint32]*Peer, 0),
		inbound:  make(chan packet, 5),
		outbound: make(chan packet, 5),
		control:  make(chan bool, 1),
		conns:    make(map[string]*listener, 0),
	}

	return s
}

func (s *BfdServer) AddPeer(api_peer *api.Peer) (*Peer, error) {
	var address string
	var err error

	if api_peer.DetectMultiplier == 0 {
		return nil, ErrInvalidDetectionMultiplierSupplied
	}

	// create a random descriptor
	discriminator := rand.Uint32()
	//  49152 through 65535
	sourcePort := 49152 + int(rand.Intn(65535-49152))

	port := BFD_PORT

	if strings.Contains(api_peer.Address, ":") {
		parts := strings.Split(api_peer.Address, ":")
		address = parts[0]

		port, err = strconv.Atoi(parts[1])

		if err != nil {
			return nil, err
		}
	} else {
		address = api_peer.Address
	}

	peer, err := NewPeer(net.ParseIP(address), port)

	if err != nil {
		return nil, err
	}

	peer.Lock()
	peer.Name = api_peer.Name
	peer.SourcePort = sourcePort
	peer.Interval = api_peer.DesiredMinTxInterval
	peer.local = &PeerState{
		sessionState:          bfd.Down,
		discriminator:         discriminator,
		desiredMinTxInterval:  1000000,
		requiredMinRxInterval: uint32(api_peer.RequiredMinRxInterval * 1000),
		detectMultiplier:      uint8(api_peer.DetectMultiplier),
	}

	peer.remote = &PeerState{
		sessionState:          bfd.Down,
		requiredMinRxInterval: 1,
	}

	conn, err := s.dialUDP("udp", &net.UDPAddr{Port: peer.SourcePort}, peer.Address)

	if err != nil {
		return nil, err
	}

	ttlConn := ipv4.NewConn(conn)
	ttlConn.SetTTL(255)

	peer.conn = conn
	peer.Unlock()

	peer.scheduleExpiry(OFFLINE_TIMEOUT)
	peer.scheduleSend(peer.local.desiredMinTxInterval)

	s.Lock()
	s.Sessions[discriminator] = peer
	s.Unlock()

	peer.Start()

	return peer, nil
}

func (s *BfdServer) GetPeerByUuid(uuid []byte) (*Peer, error) {
	s.RLock()
	defer s.RUnlock()

	for _, peer := range s.Sessions {
		if bytes.Equal(peer.uuid, uuid) {
			return peer, nil
		}
	}

	return nil, ErrPeerNotFound
}

func (s *BfdServer) ListPeer(ctx context.Context, cb func([]byte, *api.Peer) error) error {
	s.RLock()
	defer s.RUnlock()

	for _, peer := range s.Sessions {
		local := peer.GetLocal()

		peer.RLock()
		api_peer := &api.Peer{
			Name:                  peer.Name,
			Address:               peer.Address.String(),
			DesiredMinTxInterval:  local.GetDesiredMinTxInterval(),
			RequiredMinRxInterval: local.GetRequiredMinRxInterval(),
			DetectMultiplier:      uint32(local.GetDetectMultiplier()),
			IsMultiHop:            peer.IsMultiHop,
		}
		peer.RUnlock()

		err := cb(peer.uuid, api_peer)

		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}

	return nil
}

func (s *BfdServer) MonitorPeer(ctx context.Context, uuid []byte, cb func(*api.PeerStateResponse) error) error {
	peer, err := s.GetPeerByUuid(uuid)

	if err != nil {
		return err
	}

	watcher := peer.Watch()
	defer watcher.Stop()

	for {
		select {
		case state := <-watcher.Event():
			cb(state)
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *BfdServer) DeletePeer(uuid []byte) error {
	peer, err := s.GetPeerByUuid(uuid)

	if err != nil {
		return err
	}

	delete(s.Sessions, peer.GetLocal().GetDiscriminator())

	peer.Shutdown()

	return nil
}

/*

Packets = udp

dst port = 3784
src port = 49152 - 65535

*/

func (s *BfdServer) Listen(address string) error {
	port := BFD_PORT

	// parse our address and determine if a port is passed
	host, str_port, err := net.SplitHostPort(address)

	if err != nil {
		return err
	}

	ip := net.ParseIP(host)

	if ip == nil {
		return ErrInvalidIP
	}

	if str_port != "" {
		port, err = strconv.Atoi(str_port)

		if err != nil || port < 1 || port > 65536 {
			return ErrInvalidPort
		}
	}

	// now start a separate server for this address
	addr := &net.UDPAddr{
		IP:   ip,
		Port: port,
	}

	conn, err := net.ListenUDP("udp", addr)

	if err != nil {
		return err
	}

	// Setup so that we receive the TTL of incoming packets
	file, err := conn.File()

	if err != nil {
		return err
	}

	syscall.SetsockoptInt(int(file.Fd()), syscall.IPPROTO_IP, syscall.IP_RECVTTL, 1)

	file.Close()

	s.conns[address] = &listener{
		conn,
		make(chan bool, 1),
	}

	go s.handleIncomingPackets(conn)

	return nil
}

func (s *BfdServer) Serve() error {
	go s.handleIncomingBfdPacket()

	if len(s.conns) == 0 {
		err := s.Listen("0.0.0.0:" + strconv.Itoa(BFD_PORT))

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *BfdServer) Shutdown() {
	s.control <- true

	for _, peer := range s.Sessions {
		peer.Shutdown()
	}
}

func (s *BfdServer) handleIncomingBfdPacket() {
	for {
		select {
		case pkt := <-s.inbound:
			err := s.handlePacket(pkt)

			if err != nil {
				glog.Infof("%s", err.Error())
			}
		case <-s.control:
			glog.Infof("Shutdown Incoming")
			return
		}
	}
}

func (s *BfdServer) handlePacket(pkt packet) error {
	var peer *Peer
	var ok bool

	p := pkt.packet

	if err := checkPacket(p); err != nil {
		return ErrInvalidPacket
	}

	/*
		If the Your Discriminator field is nonzero,
		it MUST be used to select the session with which this BFD packet is associated.
		If no session is found, the packet MUST be discarded.
	*/

	if p.YourDiscriminator != 0 {
		s.RLock()
		peer, ok = s.Sessions[p.YourDiscriminator]
		s.RUnlock()
		if !ok {
			return ErrYourDiscriminatorNotFound
		}
	} else {
		/*
			If the Your Discriminator field is zero, the session MUST be
			selected based on some combination of other fields, possibly
			including source addressing information, the My Discriminator
			field, and the interface over which the packet was received.  The
			exact method of selection is application specific and is thus
			outside the scope of this specification.  If a matching session is
			not found, a new session MAY be created, or the packet MAY be
			discarded.  This choice is outside the scope of this
			specification.
		*/
		s.RLock()
		for _, lp := range s.Sessions {
			if lp.Address.IP.Equal(pkt.addr.IP) {
				peer = lp
				break
			}
		}
		s.RUnlock()
	}

	if peer == nil {
		return ErrPeerNotFound
	}

	return peer.handlePacket(p)
}

func checkPacket(p *bfd.ControlPacket) error {
	// If the version number is not correct (1), the packet MUST be discarded.
	if p.Version != 1 {
		return ErrInvalidVersion
	}

	/*
		If the Length field is less than the minimum correct value (24 if
		the A bit is clear, or 26 if the A bit is set), the packet MUST be
		discarded.

		If the Length field is greater than the payload of the
		encapsulating protocol, the packet MUST be discarded.

		=> handled with a length check on parse
	*/

	// If the Detect Mult field is zero, the packet MUST be discarded.
	if p.DetectMultiplier == 0 {
		return ErrInvalidDetectMultiplier
	}

	// If the Multipoint (M) bit is nonzero, the packet MUST be discarded.
	if p.Multipoint == bfd.Yes {
		return ErrInvalidMultiPoint
	}

	// If the My Discriminator field is zero, the packet MUST be discarded.
	if p.MyDiscriminator == 0 {
		return ErrInvalidMyDiscriminator
	}

	/*
		If the Your Discriminator field is zero and the State field is not
		Down or AdminDown, the packet MUST be discarded.
	*/

	if p.YourDiscriminator == 0 && (p.State != bfd.Down && p.State != bfd.AdminDown) {
		return ErrInvalidYourDiscriminator
	}

	return nil
}

func (s *BfdServer) handleIncomingPackets(conn Connection) {
	b := make([]byte, 256)
	oob := make([]byte, 256)

	for {
		err := s.readIncomingPacket(conn, b, oob)

		if err != nil {
			glog.Errorf("%v", err)
		}

		select {
		case <-s.control:
			return
		default:
		}
	}
}

func (s *BfdServer) readIncomingPacket(conn Connection, b, oob []byte) error {
	n, _, _, addr, err := conn.ReadMsgUDP(b, oob)

	if err != nil {
		return err
	}

	ttl := oob[16]

	if ttl != 255 {
		return ErrInvalidTTL
	}

	pkt := &bfd.ControlPacket{}

	err = pkt.UnmarshalBinary(b[:n])

	if err != nil {
		return err
	}

	s.inbound <- packet{
		addr,
		pkt,
	}

	return nil
}
