package server

import (
	"errors"
	"net"
	"testing"

	"github.com/Thoro/bfd/pkg/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type uuidPeer struct {
	uuid []byte
	peer *api.Peer
}

type fakeApiServer struct {
	err error

	peer *Peer

	listChannel    chan uuidPeer
	monitorChannel chan *api.PeerStateResponse
}

func NewFakeApiServer() *fakeApiServer {
	fake := &fakeApiServer{
		listChannel:    make(chan uuidPeer, 8),
		monitorChannel: make(chan *api.PeerStateResponse, 8),
	}

	return fake
}

var ErrFake = errors.New("Fake Error")

func (s *fakeApiServer) Serve() error {
	return s.err
}

func (s *fakeApiServer) Shutdown() {

}

func (s *fakeApiServer) AddPeer(*api.Peer) (*Peer, error) {
	return nil, s.err
}

func (s *fakeApiServer) GetPeerByUuid([]byte) (*Peer, error) {
	if s.peer != nil {
		return s.peer, nil
	}

	return nil, s.err
}

func (s *fakeApiServer) DeletePeer([]byte) error {
	return nil
}

func (s *fakeApiServer) ListPeer(ctx context.Context, cb func([]byte, *api.Peer) error) error {
	for p := range s.listChannel {
		err := cb(p.uuid, p.peer)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *fakeApiServer) MonitorPeer(ctx context.Context, uuid []byte, cb func(*api.PeerStateResponse) error) error {

	for p := range s.monitorChannel {
		err := cb(p)
		if err != nil {
			return err
		}
	}

	return s.err
}

type fakeSendList struct {
	grpc.ServerStream
	responses chan *api.ListPeerResponse
	sendError error
}

func newFakeSendList() *fakeSendList {
	return &fakeSendList{
		responses: make(chan *api.ListPeerResponse, 8),
	}
}

func (s *fakeSendList) Send(d *api.ListPeerResponse) error {
	s.responses <- d

	return s.sendError
}

type fakeSendMonitor struct {
	grpc.ServerStream
	responses chan *api.PeerStateResponse
	sendError error
}

func newFakeSendMonitor() *fakeSendMonitor {
	return &fakeSendMonitor{
		responses: make(chan *api.PeerStateResponse, 8),
	}
}

func (s *fakeSendMonitor) Send(d *api.PeerStateResponse) error {
	s.responses <- d

	return s.sendError
}

func (s *fakeSendMonitor) Context() context.Context {
	return context.Background()
}

func TestGrpcServe(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake
	go server.ServeApi("0.0.0.0:53021")

	server.StopApi()
}

func TestGrpcStart(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake
	_, err := server.Start(context.Background(), &api.StartRequest{})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcStop(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.Stop(context.Background(), &api.StopRequest{})

	if err != nil {
		t.Fail()
	}

}

func TestGrpcAddPeerWithErr(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: &api.Peer{
			Address: "127.0.0.1",
		},
	})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcAddPeer(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	_, err := server.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: &api.Peer{
			Address: "127.0.0.1",
		},
	})

	if err != nil {
		t.Fail()
	}
}

func TestGrpcUpdatePeerError1(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
		Uuid: []byte{0, 0, 0},
		Peer: &api.Peer{},
	})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcUpdatePeerError2(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)

	fake.peer.Start()

	_, err := server.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
		Uuid: []byte{0, 0, 0},
		Peer: &api.Peer{
			Address: "192.168.1.1",
		},
	})

	fake.peer.Shutdown()

	if err != ErrAddressNotChangeable {
		t.Fail()
	}
}

func TestGrpcUpdatePeerError3(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)

	fake.peer.Start()

	_, err := server.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
		Uuid: []byte{0, 0, 0},
		Peer: &api.Peer{
			IsMultiHop: true,
		},
	})

	fake.peer.Shutdown()

	if err != ErrMultiphopNotChangeable {
		t.Fail()
	}
}

func TestGrpcUpdatePeerSuccess(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)

	fake.peer.Start()

	_, err := server.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
		Uuid: []byte{0, 0, 0},
		Peer: &api.Peer{
			DesiredMinTxInterval:  500,
			RequiredMinRxInterval: 300,
			DetectMultiplier:      3,
		},
	})

	fake.peer.Shutdown()

	if err != nil {
		t.Fail()
	}
}

func TestGrpcDeletePeer(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	_, err := server.DeletePeer(context.Background(), &api.DeletePeerRequest{
		Uuid: []byte{0, 0, 0},
	})

	if err != nil {
		t.Fail()
	}
}

func TestGrpcListPeerSingleResult(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	peer := uuidPeer{
		uuid: []byte{},
	}

	fakeResponse := newFakeSendList()

	fake.listChannel <- peer
	close(fake.listChannel)

	err := server.ListPeer(&api.ListPeerRequest{}, fakeResponse)

	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	select {
	case <-fakeResponse.responses:
	default:
		t.Errorf("Did not receive a peer via list peers")
		t.Fail()
	}
}

func TestGrpcListPeerNoResult(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	close(fake.listChannel)

	fakeResponse := newFakeSendList()

	err := server.ListPeer(&api.ListPeerRequest{}, fakeResponse)

	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestGrpcListPeerSendError(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	peer := uuidPeer{
		uuid: []byte{},
	}

	fakeResponse := newFakeSendList()
	fakeResponse.sendError = ErrFake

	fake.listChannel <- peer
	close(fake.listChannel)

	err := server.ListPeer(&api.ListPeerRequest{}, fakeResponse)

	if err != ErrFake {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestGrpcGetPeerStateError(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.GetPeerState(context.Background(), &api.GetPeerStateRequest{
		Uuid: []byte{0, 0, 0},
	})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcGetPeerStateSuccess(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)
	fake.err = ErrFake

	_, err := server.GetPeerState(context.Background(), &api.GetPeerStateRequest{
		Uuid: []byte{0, 0, 0},
	})

	if err != nil {
		t.Fail()
	}
}

func TestGrpcMonitorPeerSendError(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	resp := &api.PeerStateResponse{}

	fakeResponse := newFakeSendMonitor()
	fakeResponse.sendError = ErrFake

	fake.monitorChannel <- resp
	close(fake.monitorChannel)

	err := server.MonitorPeer(&api.MonitorPeerRequest{
		Uuid: []byte{0, 0, 0},
	}, fakeResponse)

	if err != ErrFake {
		t.Errorf("%v", err)
		t.Fail()
	}
}

func TestGrpcMonitorPeerSucess(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	resp := &api.PeerStateResponse{}

	fakeResponse := newFakeSendMonitor()

	fake.monitorChannel <- resp
	close(fake.monitorChannel)

	err := server.MonitorPeer(&api.MonitorPeerRequest{
		Uuid: []byte{0, 0, 0},
	}, fakeResponse)

	if err != nil {
		t.Fail()
	}

	select {
	case <-fakeResponse.responses:
	default:
		t.Errorf("Did not receive a peer via list peers")
		t.Fail()
	}
}

func TestGrpcDisablePeerError(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.DisablePeer(context.Background(), &api.DisablePeerRequest{
		Uuid: []byte{0, 0, 0},
	})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcDisablePeerSuccess(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)

	fake.peer.Start()

	_, err := server.DisablePeer(context.Background(), &api.DisablePeerRequest{
		Uuid: []byte{0, 0, 0},
	})

	fake.peer.Shutdown()

	if err != nil {
		t.Fail()
	}
}

func TestGrpcEnablePeerError(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.err = ErrFake

	_, err := server.EnablePeer(context.Background(), &api.EnablePeerRequest{
		Uuid: []byte{0, 0, 0},
	})

	if err != ErrFake {
		t.Fail()
	}
}

func TestGrpcEnablePeerSuccess(t *testing.T) {
	fake := NewFakeApiServer()
	server := NewBfdApiServer(fake, grpc.NewServer())

	fake.peer, _ = NewPeer(net.ParseIP("127.0.0.1"), 16200)

	fake.peer.Start()

	_, err := server.EnablePeer(context.Background(), &api.EnablePeerRequest{
		Uuid: []byte{0, 0, 0},
	})

	fake.peer.Shutdown()

	if err != nil {
		t.Fail()
	}
}
