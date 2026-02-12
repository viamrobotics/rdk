package audioout_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/audioout"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func setupAudioOutService(t *testing.T, injectAudioOut *inject.AudioOut) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	audioOutSvc, err := resource.NewAPIResourceCollection(
		audioout.API, map[resource.Name]audioout.AudioOut{audioout.Named(testAudioOutName): injectAudioOut})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[audioout.AudioOut](audioout.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, audioOutSvc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestFailingAudioOutClient(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectAudioOut := &inject.AudioOut{}

	listener, cleanup := setupAudioOutService(t, injectAudioOut)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, context.Canceled)
}

func TestWorkingAudioOutClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioOut := &inject.AudioOut{}

	listener, cleanup := setupAudioOutService(t, injectAudioOut)
	defer cleanup()

	testWorkingClient := func(t *testing.T, client audioout.AudioOut) {
		t.Helper()

		expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
		var actualExtra map[string]interface{}

		// DoCommand
		injectAudioOut.DoFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		// Play
		audioData := []byte{1, 2, 3, 4, 5, 6, 7, 8}
		audioInfo := &rutils.AudioInfo{
			Codec:        rutils.CodecPCM16,
			SampleRateHz: 44100,
			NumChannels:  2,
		}

		injectAudioOut.PlayFunc = func(ctx context.Context, data []byte, info *rutils.AudioInfo, extra map[string]interface{}) error {
			actualExtra = extra
			test.That(t, data, test.ShouldResemble, audioData)
			test.That(t, info.Codec, test.ShouldEqual, rutils.CodecPCM16)
			test.That(t, info.SampleRateHz, test.ShouldEqual, 44100)
			test.That(t, info.NumChannels, test.ShouldEqual, 2)
			return nil
		}

		err = client.Play(context.Background(), audioData, audioInfo, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)

		actualExtra = nil

		// Properties
		expectedProperties := rutils.Properties{
			SupportedCodecs: []string{rutils.CodecPCM16, rutils.CodecMP3},
			SampleRateHz:    44100,
			NumChannels:     2,
		}

		injectAudioOut.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
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
		client, err := audioout.NewClientFromConn(ctx, conn, "", audioout.Named(testAudioOutName), logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestAudioOutClientPlayError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioOut := &inject.AudioOut{}

	listener, cleanup := setupAudioOutService(t, injectAudioOut)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	client, err := audioout.NewClientFromConn(ctx, conn, "", audioout.Named(testAudioOutName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Test Play error
	injectAudioOut.PlayFunc = func(ctx context.Context, data []byte, info *rutils.AudioInfo, extra map[string]interface{}) error {
		return errPlayFailed
	}

	audioData := []byte{1, 2, 3, 4}
	audioInfo := &rutils.AudioInfo{
		Codec:        rutils.CodecPCM16,
		SampleRateHz: 44100,
		NumChannels:  2,
	}

	err = client.Play(ctx, audioData, audioInfo, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPlayFailed.Error())
}

func TestAudioOutClientPropertiesError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectAudioOut := &inject.AudioOut{}

	listener, cleanup := setupAudioOutService(t, injectAudioOut)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	client, err := audioout.NewClientFromConn(ctx, conn, "", audioout.Named(testAudioOutName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Test Properties error
	injectAudioOut.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
		return rutils.Properties{}, errPropertiesFailed
	}

	_, err = client.Properties(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())
}
