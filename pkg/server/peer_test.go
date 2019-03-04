package server

import (
	"errors"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
	//	"fmt"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/packet/bfd"
)

func Setup(t *testing.T) *Peer {
	p, err := NewPeer(net.ParseIP("2ac9::22"), 3278)

	if err != nil {
		t.Fail()
	}

	return p
}

func TestNewPeerInvalidAddress(t *testing.T) {
	_, err := NewPeer(nil, 3278)

	if err != ErrInvalidAddress {
		t.Fail()
	}
}

func TestNewPeerInvalidPort(t *testing.T) {
	_, err := NewPeer(net.ParseIP("127.0.0.1"), 1237000)

	if err != ErrInvalidPort {
		t.Fail()
	}
}

func TestNewPeer(t *testing.T) {
	p, err := NewPeer(net.ParseIP("127.0.0.1"), 3278)

	if err != nil {
		t.Fail()
	}

	if !reflect.DeepEqual(p.uuid, p.GetUuid()) {
		t.Fail()
	}
}

func TestNewPeerIPV6(t *testing.T) {
	_, err := NewPeer(net.ParseIP("2ac9::22"), 3278)

	if err != nil {
		t.Fail()
	}
}

func TestNewPacket(t *testing.T) {
	p, err := NewPeer(net.ParseIP("2ac9::22"), 3278)

	if err != nil {
		t.Fail()
	}

	p.NewPacket(bfd.Yes, bfd.Yes)
}

type FakeConn struct {
	file  *os.File
	n     int
	oobn  int
	flags int
	addr  *net.UDPAddr
	err   error

	data []byte
	oob  []byte

	lastData []byte
}

func (f *FakeConn) File() (*os.File, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.file, nil
}

func (f *FakeConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	if f.err != nil {
		return 0, 0, 0, nil, f.err
	}

	copy(b, f.data)
	copy(oob, f.oob)

	return len(f.data), len(f.oob), f.flags, f.addr, nil
}

func (f *FakeConn) Write(b []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}

	f.lastData = b

	return len(b), nil
}

func TestSendPacket(t *testing.T) {
	p := Setup(t)

	fake := &FakeConn{}
	p.conn = fake

	err := p.Send(p.NewPacket(bfd.No, bfd.No))

	if err != nil {
		t.Fail()
	}

	fake.err = errors.New("Fake error")
	err = p.Send(p.NewPacket(bfd.No, bfd.No))

	if err != fake.err {
		t.Fail()
	}
}

func TestGetAuthenticationType(t *testing.T) {
	p := Setup(t)

	if p.GetAuthenticationType() != p.AuthType {
		t.Fail()
	}
}

func TestSetDesiredMinTxInterval(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	p.Start()
	p.SetDesiredMinTxInterval(50)

	p.local.sessionState = bfd.Up
	p.SetDesiredMinTxInterval(50)

	p.SetDesiredMinTxInterval(40)

	// should panic
	p.SetDesiredMinTxInterval(50)

	t.Fail()
}

func TestSetRequiredMinRxInterval(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	p.Start()

	p.SetRequiredMinRxInterval(100)

	p.local.sessionState = bfd.Up

	p.SetRequiredMinRxInterval(150)

	// should panic
	p.SetRequiredMinRxInterval(50)

	t.Fail()
}

func TestSetDetectMultiplier(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.SetDetectMultiplier(5)

	p.SetDetectMultiplier(2)

	p.SetDetectMultiplier(10)
}

func TestApplyLocalState(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.ApplyLocalState([]PeerStateUpdate{setSessionState(bfd.Up)})
}

func TestApplyRemoteState(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.ApplyRemoteState([]PeerStateUpdate{setSessionState(bfd.Up)})
}

func TestApplyLocalStateWithWatchers(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	watcher := p.Watch()

	p.ApplyLocalState([]PeerStateUpdate{setSessionState(bfd.Up)})

	<-watcher.Event()
}

func TestPeerEnable(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.AdminDown

	p.Enable()

	if p.local.sessionState != bfd.Down {
		t.Fail()
	}
}

func TestPeerDisable(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Up

	p.Disable()

	if p.local.sessionState != bfd.AdminDown {
		t.Fail()
	}
}

func TestPeerScheduleExpiry(t *testing.T) {
	p := Setup(t)

	p.scheduleExpiry(0)

	expiry := time.NewTimer(time.Duration(100) * time.Millisecond)

	select {
	case <-p.expiry.C:
	case <-expiry.C:
		t.Fail()
	}
}

func TestPeerScheduleSend(t *testing.T) {
	p := Setup(t)

	p.scheduleSend(0)

	expiry := time.NewTimer(time.Duration(100) * time.Millisecond)

	select {
	case <-p.ticker.C:
	case <-expiry.C:
		t.Fail()
	}
}

func TestPeerHandleExpiry(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	fake := &FakeConn{}
	p.conn = fake

	expiry := time.NewTimer(time.Duration(100) * time.Millisecond)

	p.Start()

	watcher := p.Watch()
	defer watcher.Stop()

	p.local.sessionState = bfd.Up

	p.scheduleSend(2000000)
	p.scheduleExpiry(0)

	select {
	case ev := <-watcher.Event():

		if ev.Local.State != api.SessionState_DOWN {
			t.Fail()
		}
	case <-expiry.C:
		t.Fail()
	}
}

func TestPeerHandleSend(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	fake := &FakeConn{}
	p.conn = fake

	p.Start()

	watcher := p.Watch()
	defer watcher.Stop()

	p.local.sessionState = bfd.Down
	p.local.detectMultiplier = 1
	p.remote.requiredMinRxInterval = 20

	p.scheduleSend(0)
	// p.scheduleExpiry(20000000)

	expiry := time.NewTimer(time.Duration(10) * time.Millisecond)

	select {
	case <-expiry.C:
		if fake.lastData != nil {
			// we know that data was sent out
		} else {
			t.Fail()
		}
	}
}

type FakeHeader struct{}

func (s *FakeHeader) IsValid(key []byte, packet []byte) bool {
	return true
}

func (s *FakeHeader) GetAuthenticationType() bfd.AuthenticationType {
	return bfd.AuthenticationType(10)
}

func (s *FakeHeader) UnmarshalBinary(buf []byte) error {
	return ErrNotImplemented
}

func (s *FakeHeader) MarshalBinary() ([]byte, error) {
	return nil, ErrNotImplemented
}

func TestPeerHandlePacketWithWrongAuthenticationType(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.AuthType = bfd.KeyedMD5

	pkt := &bfd.ControlPacket{}

	if p.handlePacket(pkt) != bfd.ErrInvalidAuthenticationType {
		t.Fail()
	}
}

func TestPeerHandlePacketNotImplementedAuthenticationHeader(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.AuthType = bfd.AuthenticationType(10)

	pkt := &bfd.ControlPacket{
		AuthenticationHeader: &FakeHeader{},
	}

	if err := p.handlePacket(pkt); err != ErrNotImplemented {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketOfflineTimeout(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.AuthType = bfd.Reserved

	pkt := &bfd.ControlPacket{}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketUp(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Up

	pkt := &bfd.ControlPacket{
		State: bfd.Up,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketAdminDown(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.AdminDown

	pkt := &bfd.ControlPacket{
		State: bfd.Up,
	}

	if err := p.handlePacket(pkt); err != ErrSessionAdminDown {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketInit(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Init

	pkt := &bfd.ControlPacket{
		State: bfd.Up,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketDownAndDown(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Down

	pkt := &bfd.ControlPacket{
		State: bfd.Down,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.local.sessionState != bfd.Init || p.local.diagnosticCode != bfd.NoDiagnostic {
		t.Fail()
	}
}

func TestHandlePacketDownAndInit(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Down

	pkt := &bfd.ControlPacket{
		State: bfd.Init,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.local.sessionState != bfd.Up || p.local.diagnosticCode != bfd.NoDiagnostic {
		t.Fail()
	}
}

func TestHandlePacketDownAndUp(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Down

	pkt := &bfd.ControlPacket{
		State: bfd.Up,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.local.sessionState != bfd.Down || p.local.diagnosticCode != bfd.NoDiagnostic {
		t.Fail()
	}
}

func TestHandlePacketUpAndDown(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Up

	pkt := &bfd.ControlPacket{
		State: bfd.Down,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.local.sessionState != bfd.Down || p.local.diagnosticCode != bfd.NeighborSignaledSessionDown {
		t.Fail()
	}
}

func TestHandlePacketUpAndAdminDown(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.local.sessionState = bfd.Up

	pkt := &bfd.ControlPacket{
		State: bfd.AdminDown,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.local.sessionState != bfd.Down || p.local.diagnosticCode != bfd.NeighborSignaledSessionDown {
		t.Fail()
	}
}

func TestHandlePacketPollInit(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	fake := &FakeConn{}
	p.conn = fake

	pkt := &bfd.ControlPacket{
		Poll: bfd.Yes,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketPollActive(t *testing.T) {
	p := Setup(t)
	defer p.Shutdown()

	p.Start()

	p.PollActive = true

	pkt := &bfd.ControlPacket{
		Final: bfd.Yes,
	}

	if err := p.handlePacket(pkt); err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.PollActive {
		t.Fail()
	}
}
