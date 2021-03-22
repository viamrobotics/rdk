package lidar

import (
	"context"
	"errors"
	"fmt"
	"image"
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
	wsServer.RegisterCommand(WSCommandInfo, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandStart, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandStop, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandClose, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandScan, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandRange, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandBounds, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer.RegisterCommand(WSCommandAngularResolution, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, errors.New("whoops")
	}))
	wsServer2.RegisterCommand(WSCommandInfo, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return map[string]interface{}{"hello": "world"}, nil
	}))
	wsServer2.RegisterCommand(WSCommandStart, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, nil
	}))
	wsServer2.RegisterCommand(WSCommandStop, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, nil
	}))
	wsServer2.RegisterCommand(WSCommandClose, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, nil
	}))
	wsServer2.RegisterCommand(WSCommandScan, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return Measurements{NewMeasurement(2, 40)}, nil
	}))
	wsServer2.RegisterCommand(WSCommandRange, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return 25, nil
	}))
	wsServer2.RegisterCommand(WSCommandBounds, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return image.Point{4, 5}, nil
	}))
	wsServer2.RegisterCommand(WSCommandAngularResolution, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return 5.2, nil
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
	_, err = dev.Info(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = dev.Start(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = dev.Stop(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Scan(context.Background(), ScanOptions{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Range(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.Bounds(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	_, err = dev.AngularResolution(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	err = dev.Close(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	dev, err = NewWSDevice(context.Background(), fmt.Sprintf("ws://127.0.0.1:%d/2", port))
	test.That(t, err, test.ShouldBeNil)
	info, err := dev.Info(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info, test.ShouldResemble, map[string]interface{}{"hello": "world"})
	err = dev.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = dev.Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	scan, err := dev.Scan(context.Background(), ScanOptions{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, scan, test.ShouldResemble, Measurements{NewMeasurement(2, 40)})
	devRange, err := dev.Range(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, devRange, test.ShouldEqual, 25)
	bounds, err := dev.Bounds(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bounds, test.ShouldResemble, image.Point{4, 5})
	angRes, err := dev.AngularResolution(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angRes, test.ShouldEqual, 5.2)
	err = dev.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
