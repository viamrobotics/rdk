package gostream

import (
	"context"
	"errors"
	"fmt"
	"github.com/pion/webrtc/v3"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"goji.io"
	"goji.io/pat"

	streampb "go.viam.com/rdk/gostream/proto/stream/v1"
)

// A StandaloneStreamServer is a convenience helper for solely streaming a series
// streams. Streams can be added over time for future new connections.
type StandaloneStreamServer interface {
	// AddStream adds the given stream for new connections to see.
	AddStream(stream Stream) error
	// Start starts the server and waits for new connections.
	Start(ctx context.Context) error
	// Stop stops the server and stops the underlying streams.
	Stop(ctx context.Context) error
}

type standaloneStreamServer struct {
	port                    int
	streamServer            StreamServer
	rpcServer               rpc.Server
	httpServer              *http.Server
	started                 bool
	opts                    StandaloneStreamServerOptions
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// NewStandaloneStreamServer returns a server that will run on the given port and initially starts
// with the given streams.
func NewStandaloneStreamServer(
	port int,
	logger golog.Logger,
	opts []StandaloneStreamServerOption,
	streams ...Stream,
) (StandaloneStreamServer, error) {
	var sOpts StandaloneStreamServerOptions
	for _, opt := range opts {
		opt.apply(&sOpts)
	}

	streamServer, err := NewStreamServer(streams...)
	if err != nil {
		return nil, err
	}
	return &standaloneStreamServer{
		port:         port,
		streamServer: streamServer,
		opts:         sOpts,
		logger:       logger,
	}, nil
}

// ErrServerAlreadyStarted happens when the server has already been started.
var ErrServerAlreadyStarted = errors.New("already started")

func (ss *standaloneStreamServer) AddStream(stream Stream) error {
	return ss.streamServer.AddStream(stream)
}

func (ss *standaloneStreamServer) Start(ctx context.Context) error {
	if ss.started {
		return ErrServerAlreadyStarted
	}
	ss.started = true

	humanAddress := fmt.Sprintf("localhost:%d", ss.port)
	listener, secure, err := utils.NewPossiblySecureTCPListenerFromFile(humanAddress, "", "")
	if err != nil {
		return err
	}
	var serverOpts []rpc.ServerOption
	webrtcOpts := rpc.WebRTCServerOptions{
		Enable: true,
	}
	if ss.opts.onPeerAdded != nil {
		webrtcOpts.OnPeerAdded = ss.opts.onPeerAdded
	}
	if ss.opts.onPeerRemoved != nil {
		webrtcOpts.OnPeerRemoved = ss.opts.onPeerRemoved
	}
	serverOpts = append(serverOpts, rpc.WithWebRTCServerOptions(webrtcOpts))
	serverOpts = append(serverOpts, rpc.WithUnauthenticated(), rpc.WithInstanceNames("local"))

	rpcServer, err := rpc.NewServer(ss.logger, serverOpts...)
	if err != nil {
		return err
	}
	ss.rpcServer = rpcServer

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		ss.streamServer.ServiceServer(),
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	//nolint:dogsled
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		return fmt.Errorf("error locating current file: %w", err)
	}
	t, err := template.New("foo").Funcs(template.FuncMap{
		//nolint:gosec
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		//nolint:gosec
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).ParseGlob(fmt.Sprintf("%s/*.html", filepath.Join(thisDirPath, "templates")))
	if err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}
	indexT := t.Lookup("index.html")

	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/"), func(w http.ResponseWriter, r *http.Request) {
		if err := indexT.Execute(w, struct {
			AllowSendAudio bool
		}{
			ss.opts.allowReceive,
		}); err != nil {
			panic(err)
		}
	})
	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static", http.FileServer(http.Dir(filepath.Join(thisDirPath, "frontend/dist")))))
	mux.Handle(pat.New("/*"), rpcServer.GRPCHandler())

	httpServer, err := utils.NewPlainTextHTTP2Server(mux)
	if err != nil {
		return err
	}
	httpServer.Addr = listener.Addr().String()
	ss.httpServer = httpServer

	var scheme string
	if secure {
		scheme = "https"
	} else {
		scheme = "http"
	}

	ss.activeBackgroundWorkers.Add(2)
	utils.PanicCapturingGo(func() {
		defer ss.activeBackgroundWorkers.Done()
		if err := rpcServer.Start(); err != nil {
			ss.logger.Errorw("error starting rpc server", "error", err)
		}
	})
	utils.PanicCapturingGo(func() {
		defer ss.activeBackgroundWorkers.Done()
		ss.logger.Infow("serving", "url", fmt.Sprintf("%s://%s", scheme, humanAddress))
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ss.logger.Errorw("error serving", "error", err)
		}
	})
	return nil
}

func (ss *standaloneStreamServer) Stop(ctx context.Context) (err error) {
	defer ss.activeBackgroundWorkers.Wait()
	defer func() {
		err = multierr.Combine(err, ss.rpcServer.Stop())
	}()
	defer func() {
		err = multierr.Combine(err, ss.httpServer.Shutdown(ctx))
	}()
	return ss.streamServer.Close()
}

// StandaloneStreamServerOptions configures a StandaloneStreamServer.
type StandaloneStreamServerOptions struct {
	// onPeerAdded is called when a new peer connection is added.
	onPeerAdded func(pc *webrtc.PeerConnection)
	// onPeerRemoved is called when an existing peer connection is removed.
	onPeerRemoved func(pc *webrtc.PeerConnection)
	// allowReceive sets whether or not this stream server wants to receive
	// media.
	allowReceive bool
}

// StandaloneStreamServerOption configures how we set up the server.
// Cribbed from https://github.com/grpc/grpc-go/blob/aff571cc86e6e7e740130dbbb32a9741558db805/dialoptions.go#L41
type StandaloneStreamServerOption interface {
	apply(*StandaloneStreamServerOptions)
}

// funcOption wraps a function that modifies StandaloneStreamServerOptions into an
// implementation of the Option interface.
type funcOption struct {
	f func(*StandaloneStreamServerOptions)
}

func (fdo *funcOption) apply(do *StandaloneStreamServerOptions) {
	fdo.f(do)
}

func newFuncOption(f func(*StandaloneStreamServerOptions)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithStandaloneOnPeerAdded returns an Option which sets a function to call
// when a new peer connection is added.
func WithStandaloneOnPeerAdded(f func(pc *webrtc.PeerConnection)) StandaloneStreamServerOption {
	return newFuncOption(func(o *StandaloneStreamServerOptions) {
		o.onPeerAdded = f
	})
}

// WithStandaloneOnPeerRemoved returns an Option which sets a function to call
// when an existing peer connection is remvoed.
func WithStandaloneOnPeerRemoved(f func(pc *webrtc.PeerConnection)) StandaloneStreamServerOption {
	return newFuncOption(func(o *StandaloneStreamServerOptions) {
		o.onPeerRemoved = f
	})
}

// WithStandaloneAllowReceive returns an Option which sets whether or not this
// stream server wants to receive media.
func WithStandaloneAllowReceive(allowReceive bool) StandaloneStreamServerOption {
	return newFuncOption(func(o *StandaloneStreamServerOptions) {
		o.allowReceive = allowReceive
	})
}
