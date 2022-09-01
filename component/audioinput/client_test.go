package audioinput_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	audioinput "go.viam.com/rdk/component/audioinput"
	"go.viam.com/rdk/component/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	componentpb "go.viam.com/rdk/proto/api/component/audioinput/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	audioData := &wave.Float32Interleaved{
		Data: []float32{
			0.1, -0.5, 0.2, -0.6, 0.3, -0.7, 0.4, -0.8, 0.5, -0.9, 0.6, -1.0, 0.7, -1.1, 0.8, -1.2,
		},
		Size: wave.ChunkInfo{8, 2, 48000},
	}

	injectAudioInput := &inject.AudioInput{}

	// good audio input
	injectAudioInput.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.AudioStream, error) {
		return gostream.NewEmbeddedAudioStreamFromReader(gostream.AudioReaderFunc(func(ctx context.Context) (wave.Audio, func(), error) {
			return audioData, func() {}, nil
		})), nil
	}
	expectedProps := prop.Audio{
		ChannelCount:  1,
		SampleRate:    2,
		IsBigEndian:   true,
		IsInterleaved: true,
		Latency:       5,
	}
	injectAudioInput.MediaPropertiesFunc = func(ctx context.Context) (prop.Audio, error) {
		return expectedProps, nil
	}
	// bad audio input
	injectAudioInput2 := &inject.AudioInput{}
	injectAudioInput2.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.AudioStream, error) {
		return nil, errors.New("can't generate stream")
	}

	resources := map[resource.Name]interface{}{
		audioinput.Named(testAudioInputName): injectAudioInput,
		audioinput.Named(failAudioInputName): injectAudioInput2,
	}
	audioInputSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(audioinput.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, audioInputSvc)

	injectAudioInput.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, audioInputSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("audio input client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		audioInput1Client := audioinput.NewClientFromConn(context.Background(), conn, testAudioInputName, logger)
		chunk, _, err := gostream.ReadAudio(context.Background(), audioInput1Client)
		test.That(t, err, test.ShouldBeNil)

		audioData := wave.NewFloat32Interleaved(chunk.ChunkInfo())
		// convert
		for i := 0; i < chunk.ChunkInfo().Len; i++ {
			for j := 0; j < chunk.ChunkInfo().Channels; j++ {
				audioData.Set(i, j, chunk.At(i, j))
			}
		}

		test.That(t, chunk, test.ShouldResemble, audioData)

		props, err := audioInput1Client.MediaProperties(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props, test.ShouldResemble, expectedProps)

		// Do
		resp, err := audioInput1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		test.That(t, utils.TryClose(context.Background(), audioInput1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("audio input client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failAudioInputName, logger)
		audioInput2Client, ok := client.(audioinput.AudioInput)
		test.That(t, ok, test.ShouldBeTrue)

		_, _, err = gostream.ReadAudio(context.Background(), audioInput2Client)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate stream")

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectAudioInput := &inject.AudioInput{}

	audioInputSvc, err := subtype.New(map[resource.Name]interface{}{audioinput.Named(testAudioInputName): injectAudioInput})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterAudioInputServiceServer(gServer, audioinput.NewServer(audioInputSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := audioinput.NewClientFromConn(ctx, conn1, testAudioInputName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := audioinput.NewClientFromConn(ctx, conn2, testAudioInputName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}
