//go:build !no_cgo || android

package web

import (
	"bytes"
	"context"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	weboptions "go.viam.com/rdk/robot/web/options"
	webstream "go.viam.com/rdk/robot/web/stream"
)

// StreamServer manages streams and displays.
type StreamServer struct {
	// Server serves streams
	Server *webstream.Server
	// HasStreams is true if service has streams that require a WebRTC connection.
	HasStreams bool
}

// New returns a new web service for the given robot.
func New(r robot.Robot, logger logging.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		Named:        InternalServiceName.AsNamed(),
		r:            r,
		logger:       logger,
		rpcServer:    nil,
		streamServer: nil,
		services:     map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		opts:         wOpts,
	}
	return webSvc
}

type webService struct {
	resource.Named

	mu           sync.Mutex
	r            robot.Robot
	rpcServer    rpc.Server
	modServer    rpc.Server
	streamServer *StreamServer
	services     map[resource.API]resource.APIResourceCollection[resource.Resource]
	opts         options
	addr         string
	modAddr      string
	logger       logging.Logger
	cancelCtx    context.Context
	cancelFunc   func()
	isRunning    bool
	webWorkers   sync.WaitGroup
	modWorkers   sync.WaitGroup
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
	return svc.streamServer.Server.AddNewStreams(svc.cancelCtx)
}

func (svc *webService) closeStreamServer() {
	if svc.streamServer.Server != nil {
		if err := svc.streamServer.Server.Close(); err != nil {
			svc.logger.Errorw("error closing stream server", "error", err)
		}
	}
}

func (svc *webService) initStreamServer(ctx context.Context, options *weboptions.Options) error {
	// Check to make sure stream config option is set in the webservice.
	var streamConfig gostream.StreamConfig
	if svc.opts.streamConfig != nil {
		streamConfig = *svc.opts.streamConfig
	} else {
		svc.logger.Warn("streamConfig is nil, using empty config")
	}
	server := webstream.NewServer(svc.r, streamConfig, svc.logger)
	svc.streamServer = &StreamServer{server, false}
	if err := svc.streamServer.Server.AddNewStreams(svc.cancelCtx); err != nil {
		return err
	}
	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer.Server,
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}
	if svc.streamServer.HasStreams {
		// force WebRTC template rendering
		options.PreferWebRTC = true
	}
	return nil
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
