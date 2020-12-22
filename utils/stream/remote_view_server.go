package stream

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/echolabsinc/robotcore/utils/log"
)

type RemoteViewServer interface {
	Run()
	Stop(ctx context.Context) error
}

type remoteViewServer struct {
	port       int
	remoteView RemoteView
	httpServer *http.Server
	running    bool
	logger     log.Logger
}

func NewRemoteViewServer(port int, view RemoteView, logger log.Logger) RemoteViewServer {
	return &remoteViewServer{port: port, remoteView: view, logger: logger}
}

func (rvs *remoteViewServer) Run() {
	if rvs.running {
		panic("already running")
	}
	rvs.running = true
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", rvs.port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	rvs.httpServer = httpServer
	mux := http.NewServeMux()
	httpServer.Handler = mux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(rvs.remoteView.SinglePageHTML())); err != nil {
			rvs.logger.Error(err)
			return
		}
	})
	handler := rvs.remoteView.Handler()
	mux.HandleFunc("/"+handler.Name, handler.Func)

	go func() {
		rvs.logger.Infow("listening", "port", rvs.port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func (rvs *remoteViewServer) Stop(ctx context.Context) error {
	rvs.remoteView.Stop()
	return rvs.httpServer.Shutdown(ctx)
}
