package server

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/packet/bfd"
)

func TestMax(t *testing.T) {
	if max(5, 6) != 6 {
		t.Fail()
	}

	if max(6, 5) != 6 {
		t.Fail()
	}
}

func TestMin(t *testing.T) {
	if min(5, 6) != 5 {
		t.Fail()
	}

	if min(6, 5) != 5 {
		t.Fail()
	}
}

func TestAddPeerInvalidDetectMultiplier(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		DetectMultiplier: 0,
	})

	if err != ErrInvalidDetectionMultiplierSupplied {
		t.Fail()
	}
}

func TestAddPeerValidAddressInvalidPortString(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1:asdf",
		DetectMultiplier: 1,
	})

	if err.(*strconv.NumError).Err != strconv.ErrSyntax {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestAddPeerValidAddressInvalidPortNumeric(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1:80000",
		DetectMultiplier: 1,
	})

	if err != ErrInvalidPort {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestAddPeerInvalidAddress(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		Address:          "300.300.300.300",
		DetectMultiplier: 1,
	})

	if err != ErrInvalidAddress {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestAddPeerValidAddressWithPort(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	p, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1:4000",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.Address.String() != "127.0.0.1:4000" {
		t.Fail()
	}
}

func TestAddPeerValidAddressWithoutPort(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	p, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	if p.Address.String() != "127.0.0.1:3784" {
		t.Fail()
	}
}

func FakeDialUdp(network string, laddr, raddr *net.UDPAddr) (*net.UDPConn, error) {
	return nil, errors.New("Fake error for testing")
}

func TestAddPeerDialUDPError(t *testing.T) {
	server := NewBfdServer()

	server.dialUDP = FakeDialUdp

	_, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	server.dialUDP = net.DialUDP

	if err == nil {
		t.Errorf("Expected fake error")
		t.Fail()
	}
}

func TestGetPeerByUuid(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	p, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("Unable to prepare peer: %v", err)
		t.Fail()
		return
	}

	peer, err := server.GetPeerByUuid(p.GetUuid())

	if err != nil || p != peer {
		t.Fail()
	}

	_, err = server.GetPeerByUuid([]byte{})

	if err != ErrPeerNotFound {
		t.Fail()
	}
}

func TestGetListPeer(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	_, err = server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("Unable to prepare peer: %v", err)
		t.Fail()
		return
	}

	count := 0

	server.ListPeer(
		context.Background(),
		func(uuid []byte, peer *api.Peer) error {
			count++
			return nil
		},
	)

	if count != 2 {
		t.Fail()
	}
}

func TestGetListPeerDone(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	_, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	_, err = server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("Unable to prepare peer: %v", err)
		t.Fail()
		return
	}

	count := 0

	context, _ := context.WithTimeout(context.Background(), time.Duration(0))

	server.ListPeer(
		context,
		func(uuid []byte, peer *api.Peer) error {
			count++

			return nil
		},
	)

	if count != 1 {
		t.Fail()
	}
}

func TestMonitorPeer(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	p, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	if err != nil {
		t.Errorf("Unable to prepare peer: %v", err)
		t.Fail()
		return
	}

	context, cancelFunc := context.WithCancel(context.Background())

	updates := make(chan *api.PeerStateResponse, 1)
	done := make(chan bool, 1)
	ready := make(chan bool, 1)

	go func() {
		ready <- true

		err := server.MonitorPeer(
			context,
			p.GetUuid(),
			func(state *api.PeerStateResponse) error {
				updates <- state
				return nil
			},
		)

		if err != nil {
			t.Errorf("%v", err)
			t.Fail()
		}

		done <- true
	}()

	<-ready

	p.ApplyLocalState([]PeerStateUpdate{setSessionState(bfd.Up)})

	timeout := time.NewTimer(time.Duration(200) * time.Millisecond)

	select {
	case <-updates:
	case <-timeout.C:
		t.Error("Update never received")
	}

	cancelFunc()

	timeout = time.NewTimer(time.Duration(200) * time.Millisecond)

	select {
	case <-done:
	case <-timeout.C:
		t.Error("Done timed out")
	}
}

func TestMonitorPeerInvalidUuid(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.MonitorPeer(context.Background(), []byte{}, func(state *api.PeerStateResponse) error { return nil })

	if err != ErrPeerNotFound {
		t.Fail()
	}
}

func TestDeletePeer(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	p, err := server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	err = server.DeletePeer(p.GetUuid())

	if err != nil {
		t.Fail()
	}

	err = server.DeletePeer(p.GetUuid())

	if err != ErrPeerNotFound {
		t.Fail()
	}

}

func TestDeletePeerInvalidUuid(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.DeletePeer([]byte{})

	if err != ErrPeerNotFound {
		t.Fail()
	}
}

func TestServe(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.Serve()

	if err != nil {
		t.Fail()
	}

	srv2 := NewBfdServer()
	defer srv2.Shutdown()

	err = srv2.Serve()

	if err.Error() != "listen udp 0.0.0.0:3784: bind: address already in use" {
		t.Log(err.Error())
		t.Fail()
	}
}

func TestServeInvalidIP(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.Listen("888.888.888.888:3784")

	if err != ErrInvalidIP {
		t.Log(err.Error())
		t.Fail()
	}
}

func TestServeInvalidPort(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.Listen("127.0.0.1:adf")

	if err != ErrInvalidPort {
		t.Log(err.Error())
		t.Fail()
	}
}

func TestServeMissingPort(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.Listen("127.0.0.1")

	if err.Error() != "address 127.0.0.1: missing port in address" {
		t.Log(err.Error())
		t.Fail()
	}
}

func TestShutdown(t *testing.T) {
	server := NewBfdServer()
	server.Shutdown()
}

func TestHandleIncomingBfdPacket(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	server.inbound <- packet{
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 15662,
		},
		&bfd.ControlPacket{},
	}

	server.Serve()
}

func TestHandleInvalidPacket(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.handlePacket(packet{
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 15662,
		},
		&bfd.ControlPacket{},
	})

	if err != ErrInvalidPacket {
		t.Fail()
	}
}

func TestHandlePacketDiscriminatorNotFound(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.handlePacket(packet{
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 15662,
		},
		&bfd.ControlPacket{
			Version:           1,
			DetectMultiplier:  3,
			YourDiscriminator: 55,
			MyDiscriminator:   60,
			State:             bfd.Up,
		},
	})

	if err != ErrYourDiscriminatorNotFound {
		t.Fail()
	}
}

func TestHandlePacketNoDiscriminator(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	err := server.handlePacket(packet{
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 15662,
		},
		&bfd.ControlPacket{
			Version:          1,
			DetectMultiplier: 3,
			MyDiscriminator:  60,
			State:            bfd.Down,
		},
	})

	if err != ErrPeerNotFound {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandlePacketSuccessNoDiscriminatorWithIP(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	server.AddPeer(&api.Peer{
		Address:          "127.0.0.1",
		DetectMultiplier: 1,
	})

	err := server.handlePacket(packet{
		&net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 15662,
		},
		&bfd.ControlPacket{
			Version:          1,
			DetectMultiplier: 3,
			MyDiscriminator:  60,
			State:            bfd.Down,
		},
	})

	if err != nil {
		t.Fail()
	}
}

func TestHandleIncomingPacketsUdpError(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	fake := &FakeConn{}

	fake.err = errors.New("Fake error")

	b := make([]byte, 256)
	oob := make([]byte, 256)

	err := server.readIncomingPacket(fake, b, oob)

	if err != fake.err {
		t.Fail()
	}

}

func TestHandleIncomingPacketsWrongTTL(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	fake := &FakeConn{}

	fake.n = 40
	fake.oobn = 16
	fake.oob = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254}

	b := make([]byte, 256)
	oob := make([]byte, 256)

	err := server.readIncomingPacket(fake, b, oob)

	if err != ErrInvalidTTL {
		t.Fail()
	}
}

func TestHandleIncomingPacketsUnmarshalError(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	fake := &FakeConn{}

	fake.n = 40
	fake.oobn = 16
	fake.oob = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255}
	fake.data = []byte{255, 255}

	b := make([]byte, 256)
	oob := make([]byte, 256)

	err := server.readIncomingPacket(fake, b, oob)

	if err != bfd.ErrInvalidPacketLength {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestHandleIncomingPacketsSuccess(t *testing.T) {
	server := NewBfdServer()
	defer server.Shutdown()

	fake := &FakeConn{}

	fake.n = 40
	fake.oobn = 16
	fake.oob = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255}
	fake.data, _ = (&bfd.ControlPacket{
		Version: 1,
	}).MarshalBinary()

	b := make([]byte, 256)
	oob := make([]byte, 256)

	err := server.readIncomingPacket(fake, b, oob)

	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestCheckPacketInvalidVersion(t *testing.T) {
	err := checkPacket(&bfd.ControlPacket{
		Version: 2,
	})

	if err != ErrInvalidVersion {
		t.Fail()
	}
}

func TestCheckPacketInvalidMultiplier(t *testing.T) {
	err := checkPacket(&bfd.ControlPacket{
		Version:          1,
		DetectMultiplier: 0,
	})

	if err != ErrInvalidDetectMultiplier {
		t.Fail()
	}
}

func TestCheckPacketInvalidMultipoint(t *testing.T) {

	err := checkPacket(&bfd.ControlPacket{
		Version:          1,
		DetectMultiplier: 1,
		Multipoint:       bfd.Yes,
	})

	if err != ErrInvalidMultiPoint {
		t.Fail()
	}
}

func TestCheckPacketInvalidMyDiscriminator(t *testing.T) {
	err := checkPacket(&bfd.ControlPacket{
		Version:          1,
		DetectMultiplier: 1,
		Multipoint:       bfd.No,
		MyDiscriminator:  0,
	})

	if err != ErrInvalidMyDiscriminator {
		t.Fail()
	}
}

func TestCheckPacketInvalidYourDiscriminator(t *testing.T) {

	err := checkPacket(&bfd.ControlPacket{
		Version:           1,
		DetectMultiplier:  1,
		Multipoint:        bfd.No,
		MyDiscriminator:   2343,
		YourDiscriminator: 0,
		State:             bfd.Up,
	})

	if err != ErrInvalidYourDiscriminator {
		t.Fail()
	}
}

func TestCheckPacketValid(t *testing.T) {

	err := checkPacket(&bfd.ControlPacket{
		Version:           1,
		DetectMultiplier:  1,
		Multipoint:        bfd.No,
		MyDiscriminator:   2343,
		YourDiscriminator: 0,
		State:             bfd.Down,
	})

	if err != nil {
		t.Fail()
	}
}
