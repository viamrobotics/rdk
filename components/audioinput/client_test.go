//go:build !no_media

package audioinput_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/viamrobotics/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/audioinput"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testAudioInputName = "audio1"
	failAudioInputName = "audio2"
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

	resources := map[resource.Name]audioinput.AudioInput{
		audioinput.Named(testAudioInputName): injectAudioInput,
		audioinput.Named(failAudioInputName): injectAudioInput2,
	}
	audioInputSvc, err := resource.NewAPIResourceCollection(audioinput.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[audioinput.AudioInput](audioinput.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, audioInputSvc), test.ShouldBeNil)

	injectAudioInput.DoFunc = testutils.EchoFunc

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
		audioInput1Client, err := audioinput.NewClientFromConn(context.Background(), conn, "", audioinput.Named(testAudioInputName), logger)
		test.That(t, err, test.ShouldBeNil)
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

		// DoCommand
		resp, err := audioInput1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, audioInput1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("audio input client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", audioinput.Named(failAudioInputName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = gostream.ReadAudio(context.Background(), client2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate stream")

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
