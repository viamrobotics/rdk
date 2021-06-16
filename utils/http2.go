package utils

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewPlainTextHTTP2Server returns an http.Server capable of handling HTTP/2
// over plaintext via h2c for the given handler.
func NewPlainTextHTTP2Server(handler http.Handler) (*http.Server, error) {
	http2Server, err := NewHTTP2Server()
	if err != nil {
		return nil, err
	}
	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        h2c.NewHandler(handler, http2Server.HTTP2),
	}
	httpServer.RegisterOnShutdown(func() {
		UncheckedErrorFunc(http2Server.Close)
	})
	return httpServer, nil
}

// NewHTTP2Server returns an HTTP/2 server. The returned struct contains the
// http2.Server itself as well as a http.Server that can be used to serve
// TLS based connections and is also used to gracefully shutdown the HTTP/2
// server itself since it does not provide a proper shutdown method.
func NewHTTP2Server() (*HTTP2Server, error) {
	var http1Server http.Server
	var http2Server http2.Server
	if err := http2.ConfigureServer(&http1Server, &http2Server); err != nil {
		return nil, err
	}
	return &HTTP2Server{&http1Server, &http2Server}, nil
}

// HTTP2Server provides dual access to HTTP/2 via a preconfigured HTTP/1 server
// and a direct access HTTP/2 server.
type HTTP2Server struct {
	HTTP1 *http.Server
	HTTP2 *http2.Server
}

// Close shuts down the HTTP/1 server which in turn triggers the HTTP/2 server to
// shutdown (albeit not immediately).
func (srv *HTTP2Server) Close() error {
	return srv.HTTP1.Shutdown(context.Background())
}
