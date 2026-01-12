package audioin_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/audioin"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func setupAudioInService(t *testing.T, injectAudioIn *inject.AudioIn) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	audioInSvc, err := resource.NewAPIResourceCollection(
		audioin.API, map[resource.Name]audioin.AudioIn{audioin.Named(testAudioInName): injectAudioIn})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[audioin.AudioIn](audioin.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, audioInSvc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestFailingAudioInClient(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectAudioIn := &inject.AudioIn{}

	listener, cleanup := setupAudioInService(t, injectAudioIn)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, context.Canceled)
}

func TestWorkingAudioInClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioIn := &inject.AudioIn{}

	listener, cleanup := setupAudioInService(t, injectAudioIn)
	defer cleanup()

	testWorkingClient := func(t *testing.T, client audioin.AudioIn) {
		t.Helper()

		expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
		var actualExtra map[string]interface{}

		// DoCommand
		injectAudioIn.DoFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		// GetAudio
		mockChunk1 := &audioin.AudioChunk{
			AudioData:                 []byte{1, 2, 3, 4},
			Sequence:                  1,
			StartTimestampNanoseconds: 1000000,
			EndTimestampNanoseconds:   2000000,
		}
		mockChunk2 := &audioin.AudioChunk{
			AudioData:                 []byte{5, 6, 7, 8},
			Sequence:                  2,
			StartTimestampNanoseconds: 2000000,
			EndTimestampNanoseconds:   3000000,
		}

		injectAudioIn.GetAudioFunc = func(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64,
			extra map[string]interface{}) (chan *audioin.AudioChunk, error,
		) {
			ch := make(chan *audioin.AudioChunk, 2)
			ch <- mockChunk1
			ch <- mockChunk2
			close(ch)
			return ch, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		audioCh, err := client.GetAudio(ctx, rutils.CodecPCM16, 5.0, 0, expectedExtra)
		test.That(t, err, test.ShouldBeNil)

		// Read first chunk
		chunk1, ok := <-audioCh
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, chunk1.AudioData, test.ShouldResemble, []byte{1, 2, 3, 4})
		test.That(t, chunk1.Sequence, test.ShouldEqual, 1)
		test.That(t, chunk1.StartTimestampNanoseconds, test.ShouldEqual, 1000000)
		test.That(t, chunk1.EndTimestampNanoseconds, test.ShouldEqual, 2000000)

		// Read second chunk
		chunk2, ok := <-audioCh
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, chunk2.AudioData, test.ShouldResemble, []byte{5, 6, 7, 8})
		test.That(t, chunk2.Sequence, test.ShouldEqual, 2)
		test.That(t, chunk2.StartTimestampNanoseconds, test.ShouldEqual, 2000000)
		test.That(t, chunk2.EndTimestampNanoseconds, test.ShouldEqual, 3000000)

		// Channel should be closed after all chunks are read
		_, ok = <-audioCh
		test.That(t, ok, test.ShouldBeFalse)

		actualExtra = nil

		// Properties
		expectedProperties := rutils.Properties{
			SupportedCodecs: []string{rutils.CodecPCM16, rutils.CodecMP3},
			SampleRateHz:    44100,
			NumChannels:     2,
		}

		injectAudioIn.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
			actualExtra = extra
			return expectedProperties, nil
		}

		properties, err := client.Properties(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		test.That(t, properties.SupportedCodecs, test.ShouldResemble, expectedProperties.SupportedCodecs)
		test.That(t, properties.SampleRateHz, test.ShouldEqual, expectedProperties.SampleRateHz)
		test.That(t, properties.NumChannels, test.ShouldEqual, expectedProperties.NumChannels)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}

	t.Run("New client from connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := audioin.NewClientFromConn(ctx, conn, "", audioin.Named(testAudioInName), logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestAudioInClientGetAudioError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioIn := &inject.AudioIn{}

	listener, cleanup := setupAudioInService(t, injectAudioIn)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	client, err := audioin.NewClientFromConn(ctx, conn, "", audioin.Named(testAudioInName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Test GetAudio error
	injectAudioIn.GetAudioFunc = func(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64,
		extra map[string]interface{}) (chan *audioin.AudioChunk, error,
	) {
		return nil, errGetAudioFailed
	}

	_, err = client.GetAudio(ctx, rutils.CodecPCM16, 5.0, 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errGetAudioFailed.Error())
}

func TestAudioInClientPropertiesError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioIn := &inject.AudioIn{}

	listener, cleanup := setupAudioInService(t, injectAudioIn)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	client, err := audioin.NewClientFromConn(ctx, conn, "", audioin.Named(testAudioInName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Test Properties error
	injectAudioIn.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
		return rutils.Properties{}, errPropertiesFailed
	}

	_, err = client.Properties(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())
}
