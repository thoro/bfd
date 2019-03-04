package server

import (
	"errors"
	"net"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type BfdServerApi interface {
	Serve() error
	Shutdown()
	AddPeer(*api.Peer) (*Peer, error)
	GetPeerByUuid([]byte) (*Peer, error)
	DeletePeer([]byte) error
	ListPeer(context.Context, func([]byte, *api.Peer) error) error
	MonitorPeer(context.Context, []byte, func(*api.PeerStateResponse) error) error
}

var ErrAddressNotChangeable = errors.New("Unable to change peer address")
var ErrMultiphopNotChangeable = errors.New("Unable to change multi hop")

type BfdApiServer struct {
	bfdServer  BfdServerApi
	grpcServer *grpc.Server
}

func NewBfdApiServer(server BfdServerApi, grpc *grpc.Server) *BfdApiServer {
	srv := &BfdApiServer{
		bfdServer:  server,
		grpcServer: grpc,
	}

	if grpc != nil {
		api.RegisterBfdApiServer(grpc, srv)
	}

	return srv
}

func (a *BfdApiServer) ServeApi(address string) error {
	lis, err := net.Listen("tcp", address)

	if err != nil {
		return err
	}

	a.grpcServer.Serve(lis)

	return err
}

func (a *BfdApiServer) StopApi() {
	a.grpcServer.Stop()
}

func (a *BfdApiServer) Start(ctx context.Context, req *api.StartRequest) (*empty.Empty, error) {

	err := a.bfdServer.Serve()

	return &empty.Empty{}, err
}

func (a *BfdApiServer) Stop(ctx context.Context, req *api.StopRequest) (*empty.Empty, error) {

	a.bfdServer.Shutdown()

	return &empty.Empty{}, nil
}

func (a *BfdApiServer) AddPeer(ctx context.Context, req *api.AddPeerRequest) (*api.AddPeerResponse, error) {
	_, err := a.bfdServer.AddPeer(req.Peer)

	if err != nil {
		return nil, err
	}

	return &api.AddPeerResponse{}, nil
}

func (a *BfdApiServer) UpdatePeer(ctx context.Context, req *api.UpdatePeerRequest) (*empty.Empty, error) {

	peer, err := a.bfdServer.GetPeerByUuid(req.Uuid)

	if err != nil {
		return nil, err
	}

	if req.Peer.Address != "" && req.Peer.Address != peer.Address.String() {
		return nil, ErrAddressNotChangeable
	}

	if req.Peer.IsMultiHop != false && req.Peer.IsMultiHop != peer.IsMultiHop {
		return nil, ErrMultiphopNotChangeable
	}

	local := peer.GetLocal()

	if req.Peer.DesiredMinTxInterval != 0 && req.Peer.DesiredMinTxInterval != local.GetDesiredMinTxInterval() {
		peer.SetDesiredMinTxInterval(req.Peer.DesiredMinTxInterval)
	}

	if req.Peer.RequiredMinRxInterval != 0 && req.Peer.RequiredMinRxInterval != local.GetRequiredMinRxInterval() {
		peer.SetRequiredMinRxInterval(req.Peer.RequiredMinRxInterval)
	}

	if req.Peer.DetectMultiplier != 0 && uint8(req.Peer.DetectMultiplier) != local.GetDetectMultiplier() {
		peer.SetDetectMultiplier(uint8(req.Peer.DetectMultiplier))
	}

	return &empty.Empty{}, nil
}

func (a *BfdApiServer) DeletePeer(ctx context.Context, req *api.DeletePeerRequest) (*empty.Empty, error) {
	return &empty.Empty{}, a.bfdServer.DeletePeer(req.Uuid)
}

func (a *BfdApiServer) ListPeer(req *api.ListPeerRequest, stream api.BfdApi_ListPeerServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var err error

	return a.bfdServer.ListPeer(ctx, func(uuid []byte, peer *api.Peer) error {
		// wrap in func with callback
		err = stream.Send(&api.ListPeerResponse{
			Uuid: uuid,
			Peer: peer,
		})

		if err != nil {
			cancel()
			return err
		}

		return nil
	})
}

func (a *BfdApiServer) GetPeerState(ctx context.Context, req *api.GetPeerStateRequest) (*api.PeerStateResponse, error) {
	peer, err := a.bfdServer.GetPeerByUuid(req.Uuid)

	if err != nil {
		return nil, err
	}

	return &api.PeerStateResponse{
		Local:  peer.GetLocal().ToApi(),
		Remote: peer.GetRemote().ToApi(),
	}, nil
}

func (a *BfdApiServer) MonitorPeer(req *api.MonitorPeerRequest, stream api.BfdApi_MonitorPeerServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	var err error

	return a.bfdServer.MonitorPeer(ctx, req.Uuid, func(state *api.PeerStateResponse) error {

		// wrap in func with callback
		err = stream.Send(state)

		if err != nil {
			cancel()
			return err
		}

		return nil
	})
}

func (a *BfdApiServer) DisablePeer(ctx context.Context, req *api.DisablePeerRequest) (*empty.Empty, error) {
	peer, err := a.bfdServer.GetPeerByUuid(req.Uuid)

	if err != nil {
		return nil, err
	}

	peer.Disable()

	return &empty.Empty{}, nil
}

func (a *BfdApiServer) EnablePeer(ctx context.Context, req *api.EnablePeerRequest) (*empty.Empty, error) {
	peer, err := a.bfdServer.GetPeerByUuid(req.Uuid)

	if err != nil {
		return nil, err
	}

	peer.Enable()

	return &empty.Empty{}, nil
}
