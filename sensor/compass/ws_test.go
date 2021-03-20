package compass

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/edaniels/test"
	"github.com/edaniels/wsapi"
)

func TestWSDevice(t *testing.T) {
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
	wsServer.RegisterCommand(WSCommandHeading, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer2.RegisterCommand(WSCommandHeading, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
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

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewWSDevice(cancelCtx, fmt.Sprintf("ws://127.0.0.1:%d/1", port))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	dev, err := NewWSDevice(context.Background(), fmt.Sprintf("ws://127.0.0.1:%d/1", port))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dev.StartCalibration(context.Background()), test.ShouldBeNil)
	test.That(t, dev.StopCalibration(context.Background()), test.ShouldBeNil)
	_, err = dev.Heading(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Readings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dev.Close(context.Background()), test.ShouldBeNil)

	dev, err = NewWSDevice(context.Background(), fmt.Sprintf("ws://127.0.0.1:%d/2", port))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dev.StartCalibration(context.Background()), test.ShouldBeNil)
	test.That(t, dev.StopCalibration(context.Background()), test.ShouldBeNil)
	heading, err := dev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 23.45)
	readings, err := dev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{23.45})
	test.That(t, dev.Close(context.Background()), test.ShouldBeNil)
}
