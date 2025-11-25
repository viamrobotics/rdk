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
	"go.viam.com/rdk/resource"
	webstream "go.viam.com/rdk/robot/web/stream"
)

// Reconfigure updates the stream server audio and video streams with the new resources.
func (svc *webService) Reconfigure(ctx context.Context, _ resource.Dependencies, _ resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.isRunning {
		return nil
	}
	return svc.streamServer.AddNewStreams(svc.cancelCtx)
}

func (svc *webService) closeStreamServer() {
	// streamServer is called by svc.stopWeb, which is called by both Stop and Close in the shutdown process.
	if svc.streamServer != nil {
		if err := svc.streamServer.Close(); err != nil {
			svc.logger.Errorw("error closing stream server", "error", err)
		}

		// RSDK-10570: Nil out the stream server such that we recreate it on a `runWeb` call. Recreating
		// the stream server is important for passing in a fresh `svc.cancelCtx` that's in an alive
		// state. The stream server checks that context, for example, when handling the AddStream API
		// call.
		svc.streamServer = nil
	}
}

func (svc *webService) initStreamServer(ctx context.Context, srv rpc.Server) error {
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
