package audioout_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioout/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/audioout"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testAudioOutName = "audio_out1"
	failAudioOutName = "audio_out2"
)

var (
	errPlayFailed       = errors.New("can't play audio")
	errPropertiesFailed = errors.New("can't get properties")
)

func newServer(logger logging.Logger) (pb.AudioOutServiceServer, *inject.AudioOut, *inject.AudioOut, error) {
	injectAudioOut := &inject.AudioOut{}
	injectAudioOut2 := &inject.AudioOut{}
	audioOuts := map[resource.Name]audioout.AudioOut{
		audioout.Named(testAudioOutName): injectAudioOut,
		audioout.Named(failAudioOutName): injectAudioOut2,
	}
	audioOutSvc, err := resource.NewAPIResourceCollection(audioout.API, audioOuts)
	if err != nil {
		return nil, nil, nil, err
	}
	return audioout.NewRPCServiceServer(audioOutSvc, logger).(pb.AudioOutServiceServer), injectAudioOut, injectAudioOut2, nil
}

func TestServer(t *testing.T) {
	audioOutServer, injectAudioOut, _, err := newServer(logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	audioData := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	audioInfo := &commonpb.AudioInfo{
		Codec:        rutils.CodecPCM16,
		SampleRateHz: 44100,
		NumChannels:  2,
	}

	t.Run("Play", func(t *testing.T) {
		// Test successful play
		injectAudioOut.PlayFunc = func(ctx context.Context, data []byte, info *rutils.AudioInfo, extra map[string]interface{}) error {
			test.That(t, data, test.ShouldResemble, audioData)
			test.That(t, info.Codec, test.ShouldEqual, rutils.CodecPCM16)
			test.That(t, info.SampleRateHz, test.ShouldEqual, 44100)
			test.That(t, info.NumChannels, test.ShouldEqual, 2)
			return nil
		}

		playReq := &pb.PlayRequest{
			Name:      testAudioOutName,
			AudioData: audioData,
			AudioInfo: audioInfo,
		}

		resp, err := audioOutServer.Play(context.Background(), playReq)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)

		// Test play error
		injectAudioOut.PlayFunc = func(ctx context.Context, data []byte, info *rutils.AudioInfo, extra map[string]interface{}) error {
			return errPlayFailed
		}

		_, err = audioOutServer.Play(context.Background(), playReq)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPlayFailed.Error())
	})

	t.Run("Properties", func(t *testing.T) {
		expectedProperties := rutils.Properties{
			SupportedCodecs: []string{rutils.CodecPCM16, rutils.CodecMP3},
			SampleRateHz:    44100,
			NumChannels:     2,
		}

		injectAudioOut.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
			return expectedProperties, nil
		}

		resp, err := audioOutServer.GetProperties(
			context.Background(),
			&commonpb.GetPropertiesRequest{Name: testAudioOutName},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.SupportedCodecs, test.ShouldResemble, expectedProperties.SupportedCodecs)
		test.That(t, resp.SampleRateHz, test.ShouldEqual, expectedProperties.SampleRateHz)
		test.That(t, resp.NumChannels, test.ShouldEqual, expectedProperties.NumChannels)

		// Test properties error
		injectAudioOut.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
			return rutils.Properties{}, errPropertiesFailed
		}
		_, err = audioOutServer.GetProperties(
			context.Background(),
			&commonpb.GetPropertiesRequest{Name: testAudioOutName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())
	})

	t.Run("DoCommand", func(t *testing.T) {
		command := map[string]interface{}{"command": "test", "data": 500}
		injectAudioOut.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			return cmd, nil
		}

		pbCommand, err := protoutils.StructToStructPb(command)
		test.That(t, err, test.ShouldBeNil)
		doCommandRequest := &commonpb.DoCommandRequest{
			Name:    testAudioOutName,
			Command: pbCommand,
		}
		doCommandResponse, err := audioOutServer.DoCommand(context.Background(), doCommandRequest)
		test.That(t, err, test.ShouldBeNil)

		// Make sure we get the same thing back
		respMap := doCommandResponse.Result.AsMap()
		test.That(t, respMap["command"], test.ShouldEqual, "test")
		test.That(t, respMap["data"], test.ShouldEqual, 500)
	})
}
