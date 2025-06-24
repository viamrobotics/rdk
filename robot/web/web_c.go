//go:build !no_cgo || android

package web

import (
	"bytes"
	"context"
	"net/http"

	"github.com/pkg/errors"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	webstream "go.viam.com/rdk/robot/web/stream"
)

// New returns a new web service for the given robot.
func New(r robot.Robot, logger logging.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		Named:              InternalServiceName.AsNamed(),
		r:                  r,
		logger:             logger,
		rpcServer:          nil,
		streamServer:       nil,
		services:           map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		modPeerConnTracker: grpc.NewModPeerConnTracker(),
		opts:               wOpts,
	}
	return webSvc
}

// Reconfigure pulls resources and updates the stream server audio and video streams with the new resources.
func (svc *webService) Reconfigure(ctx context.Context, deps resource.Dependencies, _ resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if err := svc.updateResources(deps); err != nil {
		return err
	}
	if !svc.isRunning {
		return nil
	}
	return svc.streamServer.AddNewStreams(svc.cancelCtx)
}

func (svc *webService) closeStreamServer() {
	if err := svc.streamServer.Close(); err != nil {
		svc.logger.Errorw("error closing stream server", "error", err)
	}

	// RSDK-10570: Nil out the stream server such that we recreate it on a `runWeb` call. Recreating
	// the stream server is important for passing in a fresh `svc.cancelCtx` that's in an alive
	// state. The stream server checks that context, for example, when handling the AddStream API
	// call.
	svc.streamServer = nil
}

func (svc *webService) initStreamServer(ctx context.Context) error {
	// The webService depends on the stream server in addition to modules. We relax expectations on
	// what will be started first and allow for any order.
	if svc.streamServer == nil {
		var streamConfig gostream.StreamConfig
		if svc.opts.streamConfig != nil {
			streamConfig = *svc.opts.streamConfig
		} else {
			svc.logger.Warn("streamConfig is nil, using empty config")
		}
		svc.streamServer = webstream.NewServer(svc.r, streamConfig, svc.logger)
	}

	if err := svc.streamServer.AddNewStreams(svc.cancelCtx); err != nil {
		return err
	}

	// Register the stream server + APIs with the outward facing gRPC server.
	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer,
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}
	return nil
}

func (svc *webService) initStreamServerForModule(ctx context.Context, srv rpc.Server) error {
	// Module's can depend on the stream server, in addition to the general "client facing" RPC
	// server. We relax expectations on what will be started first and allow for any order.
	if svc.streamServer == nil {
		var streamConfig gostream.StreamConfig
		if svc.opts.streamConfig != nil {
			streamConfig = *svc.opts.streamConfig
		} else {
			svc.logger.Warn("streamConfig is nil, using empty config")
		}
		svc.streamServer = webstream.NewServer(svc.r, streamConfig, svc.logger)
	}

	if err := svc.streamServer.AddNewStreams(svc.cancelCtx); err != nil {
		return err
	}

	// Register the stream server + APIs with the gRPC server for modules.
	return srv.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer,
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	)
}

type filterXML struct {
	called bool
	w      http.ResponseWriter
}

func (fxml *filterXML) Write(bs []byte) (int, error) {
	if fxml.called {
		return 0, errors.New("cannot write more than once")
	}
	lines := bytes.Split(bs, []byte("\n"))
	// HACK: these lines are XML Document Type Definition strings
	lines = lines[6:]
	bs = bytes.Join(lines, []byte("\n"))
	n, err := fxml.w.Write(bs)
	if err == nil {
		fxml.called = true
	}
	return n, err
}
