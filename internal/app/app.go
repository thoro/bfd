package app

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"
	"context"
	"strconv"

	"google.golang.org/grpc"
	"github.com/golang/glog"
	"github.com/go-yaml/yaml"
	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/server"
	"github.com/Thoro/bfd/pkg/config"
)

type BfdApp struct {
	srv *server.BfdServer
	grpc *grpc.Server
	api  *server.BfdApiServer
}

func NewBfdApp() *BfdApp {
	return &BfdApp{}
}

func (s *BfdApp) LoadConfig(path string) error {
	// Read our yaml config file
	data, err := ioutil.ReadFile(path)

	if err != nil {
		return errors.New(fmt.Sprintf("Error reading config file: %s", err.Error()))
	}

	conf := &config.Config{}
	err = yaml.Unmarshal([]byte(data), conf)

	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing data as yaml: %s", err.Error()))
	}

	glog.Infof("%v", conf)

	for _, ip := range conf.Listen {
		s.srv.Listen(ip)
	}

	// Update the live config
	for ip, settings := range conf.Peers {
		peer, err := s.srv.AddPeer(&api.Peer{
			Name: settings.Name,
			Address: ip,
			DesiredMinTxInterval: uint32(settings.Interval),
			RequiredMinRxInterval: uint32(settings.Interval),
			DetectMultiplier: uint32(settings.DetectionMultiplier),
		})

		if err != nil {
			glog.Errorf("Error adding peer: %s", err)
			continue
		}

		go s.ListenStateUpdates(peer)
	}

	return nil
}

func (s *BfdApp) ListenStateUpdates(peer *server.Peer) {
	downCounter := 0

	s.srv.MonitorPeer(
		context.Background(),
		peer.GetUuid(),
		func(state *api.PeerStateResponse) error {
			switch state.Local.State {
				case api.SessionState_DOWN:
					downCounter++
			}

			glog.Infof("[%s] State is %s (Down: %d)", peer.Address.String(), state.String(), downCounter)

			return nil
		},
	)
}

func (s *BfdApp) NewGrpcServer() *grpc.Server {
	maxSize := 256 << 20

	grpcOpts := []grpc.ServerOption{grpc.MaxRecvMsgSize(maxSize), grpc.MaxSendMsgSize(maxSize)}

	/*
	if opts.TLS {
		creds, err := credentials.NewServerTLSFromFile(opts.TLSCertFile, opts.TLSKeyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}
	*/

	return grpc.NewServer(grpcOpts...)
}

func (s *BfdApp) Start() {
	// init our default random source
	rand.Seed(time.Now().UnixNano())

	s.srv = server.NewBfdServer()

	s.grpc = s.NewGrpcServer()

	s.api = server.NewBfdApiServer(s.srv, s.grpc)

	go s.api.ServeApi("127.0.0.1:" + strconv.Itoa(api.GRPC_PORT))

	/*if err != nil {
		glog.Errorf("Error starting server: %s", err.Error())
		return
	}*/

	err := s.srv.Serve()

	if err != nil {
		glog.Errorf("Error starting server: %s", err.Error())
		return
	}

	glog.Infof("Started Server")
}

func (s *BfdApp) Shutdown() {
	s.srv.Shutdown()
	glog.Infof("Shutdown Server")
}

