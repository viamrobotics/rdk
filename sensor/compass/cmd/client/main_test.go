package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"github.com/edaniels/wsapi"
)

func TestMain(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	port := listener.Addr().(*net.TCPAddr).Port
	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	wsServer := wsapi.NewServer()
	wsServer2 := wsapi.NewServer()
	wsServer.RegisterCommand(compass.WSCommandHeading, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer2.RegisterCommand(compass.WSCommandHeading, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return 23.45, nil
	}))
	mux := http.NewServeMux()
	mux.Handle("/1", wsServer.HTTPHandler())
	mux.Handle("/2", wsServer2.HTTPHandler())
	httpServer.Handler = mux
	go func() {
		httpServer.Serve(listener)
	}()
	defer httpServer.Close()

	assignLogger := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		wsServer.SetLogger(logger)
		wsServer2.SetLogger(logger)
	}
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "Usage of", assignLogger, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", assignLogger, nil, nil},

		// reading
		{"bad address", []string{"--device=blah://127.0.0.1:4444"}, "scheme", assignLogger, nil, nil},
		{"bad heading", []string{fmt.Sprintf("--device=ws://127.0.0.1:%d/1", port)}, "", assignLogger, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("failed").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("stats").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"normal heading", []string{fmt.Sprintf("--device=ws://127.0.0.1:%d/2", port)}, "", assignLogger, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterField(zap.Float64("data", 23.45)).All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("stats").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
