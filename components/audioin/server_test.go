package audioin_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioin/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testAudioInName = "audio_in1"
	failAudioInName = "audio_in2"
)

var (
	errGetAudioFailed   = errors.New("can't get audio")
	errPropertiesFailed = errors.New("can't get properties")
	errSendFailed       = errors.New("send fail")
)

// Mock streaming server for getAudio RPC.
type getAudioServer struct {
	grpc.ServerStream
	ctx       context.Context
	audioChan chan *pb.GetAudioResponse
	fail      bool
}

func (x *getAudioServer) Context() context.Context {
	return x.ctx
}

func (x *getAudioServer) Send(m *pb.GetAudioResponse) error {
	if x.fail {
		return errSendFailed
	}
	if x.audioChan == nil {
		return nil
	}
	x.audioChan <- m
	return nil
}

func newServer(logger logging.Logger) (pb.AudioInServiceServer, *inject.AudioIn, *inject.AudioIn, error) {
	injectAudioIn := &inject.AudioIn{}
	injectAudioIn2 := &inject.AudioIn{}
	audioIns := map[resource.Name]audioin.AudioIn{
		audioin.Named(testAudioInName): injectAudioIn,
		audioin.Named(failAudioInName): injectAudioIn2,
	}
	audioInSvc, err := resource.NewAPIResourceCollection(audioin.API, audioIns)
	if err != nil {
		return nil, nil, nil, err
	}
	return audioin.NewRPCServiceServer(audioInSvc, logger).(pb.AudioInServiceServer), injectAudioIn, injectAudioIn2, nil
}

func TestServer(t *testing.T) {
	audioInServer, injectAudioIn, _, err := newServer(logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	getAudioRequest := &pb.GetAudioRequest{
		Name:            testAudioInName,
		Codec:           rutils.CodecPCM16,
		DurationSeconds: 5.0,
	}

	t.Run("GetAudio", func(t *testing.T) {
		// Mock audio chunks to return
		mockChunk1 := &audioin.AudioChunk{
			AudioData: []byte{1, 2, 3, 4},
			Sequence:  1,
		}
		mockChunk2 := &audioin.AudioChunk{
			AudioData: []byte{5, 6, 7, 8},
			Sequence:  2,
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

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := make(chan *pb.GetAudioResponse, 2)
		s := &getAudioServer{
			ctx:       cancelCtx,
			audioChan: ch,
			fail:      false,
		}

		workers := goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
			err := audioInServer.GetAudio(getAudioRequest, s)
			test.That(t, err, test.ShouldBeNil)
		})

		// Test that the channel returns the expected chunks
		resp1 := <-s.audioChan
		test.That(t, resp1.Audio.AudioData, test.ShouldResemble, []byte{1, 2, 3, 4})
		test.That(t, resp1.Audio.Sequence, test.ShouldEqual, 1)

		resp2 := <-s.audioChan
		test.That(t, resp2.Audio.AudioData, test.ShouldResemble, []byte{5, 6, 7, 8})
		test.That(t, resp2.Audio.Sequence, test.ShouldEqual, 2)

		workers.Stop()

		// Test a failing getAudio request
		injectAudioIn.GetAudioFunc = func(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64,
			extra map[string]interface{}) (chan *audioin.AudioChunk, error,
		) {
			return nil, errGetAudioFailed
		}

		s2 := &getAudioServer{
			ctx:       context.Background(),
			audioChan: nil,
			fail:      false,
		}
		err = audioInServer.GetAudio(getAudioRequest, s2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetAudioFailed.Error())
	})

	t.Run("Properties", func(t *testing.T) {
		injectAudioIn.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
			return rutils.Properties{}, nil
		}

		resp, err := audioInServer.GetProperties(
			context.Background(),
			&commonpb.GetPropertiesRequest{Name: testAudioInName},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)

		injectAudioIn.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
			return rutils.Properties{}, errPropertiesFailed
		}
		_, err = audioInServer.GetProperties(
			context.Background(),
			&commonpb.GetPropertiesRequest{Name: testAudioInName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())
	})

	t.Run("DoCommand", func(t *testing.T) {
		command := map[string]interface{}{"command": "test", "data": 500}
		injectAudioIn.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			return cmd, nil
		}

		pbCommand, err := protoutils.StructToStructPb(command)
		test.That(t, err, test.ShouldBeNil)
		doCommandRequest := &commonpb.DoCommandRequest{
			Name:    testAudioInName,
			Command: pbCommand,
		}
		doCommandResponse, err := audioInServer.DoCommand(context.Background(), doCommandRequest)
		test.That(t, err, test.ShouldBeNil)

		// Make sure we get the same thing back
		respMap := doCommandResponse.Result.AsMap()
		test.That(t, respMap["command"], test.ShouldEqual, "test")
		test.That(t, respMap["data"], test.ShouldEqual, 500)
	})
}
