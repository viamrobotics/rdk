package stream

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type RemoteViewServer interface {
	Run(ctx context.Context) error
}

type remoteViewServer struct {
	port       int
	remoteView RemoteView
}

func NewRemoteViewServer(port int, view RemoteView) RemoteViewServer {
	return &remoteViewServer{port, view}
}

func (rvs *remoteViewServer) Run(ctx context.Context) error {
	// TODO(erd): refactor to listener thingy func
	// Wait for the offer to be submitted
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", rvs.port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	mux := http.NewServeMux()
	httpServer.Handler = mux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(rvs.remoteView.SinglePageHTML()))
	})
	handler := rvs.remoteView.Handler()
	mux.HandleFunc("/"+handler.Name, handler.Func)

	go func() {
		println("listening...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	return nil
}
