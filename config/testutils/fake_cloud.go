// Package testutils helpers for testing the config retrievial.
package testutils

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// FakeCredentialPayLoad the hardcoded payload for all devices.
const FakeCredentialPayLoad = "some-secret"

// FakeCloudServer fake implementation of the Viam Cloud RobotService.
type FakeCloudServer struct {
	pb.UnimplementedRobotServiceServer

	rpcServer rpc.Server
	listener  net.Listener
	exitWg    sync.WaitGroup

	deviceConfigs map[string]*configAndCerts

	failOnConfigAndCerts bool

	mu sync.Mutex
}

type configAndCerts struct {
	cfg   *pb.RobotConfig
	certs *pb.CertificateResponse
}

// NewFakeCloudServer creates and starts a new grpc server for the Viam Cloud.
func NewFakeCloudServer(ctx context.Context, logger logging.Logger) (*FakeCloudServer, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	if err != nil {
		return nil, err
	}

	server := &FakeCloudServer{
		listener:      listener,
		deviceConfigs: map[string]*configAndCerts{},
	}

	server.rpcServer, err = rpc.NewServer(logger.AsZap(),
		rpc.WithDisableMulticastDNS(),
		rpc.WithAuthHandler(rutils.CredentialsTypeRobotSecret, rpc.AuthHandlerFunc(
			server.robotSecretAuthenticate,
		)),
		rpc.WithEntityDataLoader(rutils.CredentialsTypeRobotSecret, rpc.EntityDataLoaderFunc(
			server.robotSecretEntityDataLoad,
		)),
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{Enable: false}))
	if err != nil {
		return nil, err
	}

	err = server.rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		server,
		pb.RegisterRobotServiceHandlerFromEndpoint,
	)
	if err != nil {
		return nil, err
	}

	server.exitWg.Add(1)
	utils.PanicCapturingGo(func() {
		defer server.exitWg.Done()

		err := server.rpcServer.Serve(server.listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warnf("Error shutting down grpc server", "error", err)
		}
	})

	return server, nil
}

// FailOnConfigAndCerts if `failure` is true the server will return an Internal error on
// all certficate and config requests.
func (s *FakeCloudServer) FailOnConfigAndCerts(failure bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOnConfigAndCerts = failure
}

// Addr returns the listeners address.
func (s *FakeCloudServer) Addr() net.Addr {
	return s.listener.Addr()
}

// Shutdown will stop the server.
func (s *FakeCloudServer) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.rpcServer.Stop()
	if err != nil {
		return err
	}

	s.exitWg.Wait()

	return nil
}

// Clear resets the fake servers state, does not restart the server.
func (s *FakeCloudServer) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deviceConfigs = map[string]*configAndCerts{}
}

// StoreDeviceConfig store config and cert data for the device id.
func (s *FakeCloudServer) StoreDeviceConfig(id string, cfg *pb.RobotConfig, cert *pb.CertificateResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deviceConfigs[id] = &configAndCerts{cfg: cfg, certs: cert}
}

// Config impl.
func (s *FakeCloudServer) Config(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failOnConfigAndCerts {
		return nil, status.Error(codes.Internal, "oops failure")
	}

	d, ok := s.deviceConfigs[req.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "config for device not found")
	}

	return &pb.ConfigResponse{Config: d.cfg}, nil
}

// Certificate impl.
func (s *FakeCloudServer) Certificate(ctx context.Context, req *pb.CertificateRequest) (*pb.CertificateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failOnConfigAndCerts {
		return nil, status.Error(codes.Internal, "oops failure")
	}

	d, ok := s.deviceConfigs[req.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "cert for device not found")
	}

	return d.certs, nil
}

// Log impl.
func (s *FakeCloudServer) Log(ctx context.Context, req *pb.LogRequest) (*pb.LogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Log not implemented")
}

// NeedsRestart impl.
func (s *FakeCloudServer) NeedsRestart(ctx context.Context, req *pb.NeedsRestartRequest) (*pb.NeedsRestartResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method NeedsRestart not implemented")
}

func (s *FakeCloudServer) robotSecretAuthenticate(ctx context.Context, entity, payload string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.deviceConfigs[entity]
	if !ok {
		return nil, errors.New("failed to auth device not found in fake server")
	}

	if payload != FakeCredentialPayLoad {
		return nil, errors.New("failed to auth device payload does not match")
	}

	return map[string]string{}, nil
}

func (s *FakeCloudServer) robotSecretEntityDataLoad(ctx context.Context, claims rpc.Claims) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.deviceConfigs[claims.Entity()]
	if !ok {
		return nil, errors.New("failed to verify entity in fake server")
	}

	return map[string]string{}, nil
}
