// Package server implement an echo server used for testing.
package server

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"

	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
)

// RPCEntityInfo prevents a package cycle. DO NOT set this to anything other
// than the real thing.
type RPCEntityInfo struct {
	Entity string
	Data   interface{}
}

// Server implements a simple echo service.
type Server struct {
	mu sync.Mutex
	echopb.UnimplementedEchoServiceServer
	fail       bool
	authorized bool

	// prevents a package cycle. DO NOT set this to anything other
	// than the real thing.
	MustContextAuthEntity func(ctx context.Context) RPCEntityInfo

	expectedAuthEntity     string
	expectedAuthEntityData interface{}
}

// SetFail instructs the server to fail at certain points in its execution.
func (srv *Server) SetFail(fail bool) {
	srv.mu.Lock()
	srv.fail = fail
	srv.mu.Unlock()
}

// SetAuthorized instructs the server to check authorization at certain points.
func (srv *Server) SetAuthorized(authorized bool) {
	srv.mu.Lock()
	srv.authorized = authorized
	srv.mu.Unlock()
}

// SetExpectedAuthEntity sets the expected auth entity.
func (srv *Server) SetExpectedAuthEntity(entity string) {
	srv.mu.Lock()
	srv.expectedAuthEntity = entity
	srv.mu.Unlock()
}

// SetExpectedAuthEntityData sets the expected auth entity data.
func (srv *Server) SetExpectedAuthEntityData(data interface{}) {
	srv.mu.Lock()
	srv.expectedAuthEntityData = data
	srv.mu.Unlock()
}

// Echo responds back with the same message.
func (srv *Server) Echo(ctx context.Context, req *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.fail {
		return nil, errors.New("whoops")
	}
	if srv.authorized {
		entity := srv.MustContextAuthEntity(ctx)
		if srv.expectedAuthEntity == "" {
			if entity.Entity == "" {
				return nil, errors.New("empty entity")
			}
		} else if entity.Entity != srv.expectedAuthEntity {
			return nil, errors.Errorf("expected auth entity %q; got %q", srv.expectedAuthEntity, entity.Data)
		}

		if srv.expectedAuthEntityData != nil && entity.Data != srv.expectedAuthEntityData {
			return nil, errors.Errorf("expected auth entity data %q; got %q", srv.expectedAuthEntityData, entity.Data)
		}
	}
	return &echopb.EchoResponse{Message: req.GetMessage()}, nil
}

// EchoMultiple responds back with the same message one character at a time.
func (srv *Server) EchoMultiple(req *echopb.EchoMultipleRequest, server echopb.EchoService_EchoMultipleServer) error {
	cnt := len(req.GetMessage())
	for i := 0; i < cnt; i++ {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		default:
		}
		if err := server.Send(&echopb.EchoMultipleResponse{Message: req.GetMessage()[i : i+1]}); err != nil {
			return err
		}
	}
	return nil
}

// EchoBiDi responds back with the same message one character at a time for each message sent to it.
func (srv *Server) EchoBiDi(server echopb.EchoService_EchoBiDiServer) error {
	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		default:
		}
		req, err := server.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		cnt := len(req.GetMessage())
		for i := 0; i < cnt; i++ {
			select {
			case <-server.Context().Done():
				return server.Context().Err()
			default:
			}
			if err := server.Send(&echopb.EchoBiDiResponse{Message: req.GetMessage()[i : i+1]}); err != nil {
				return err
			}
		}
	}
}
