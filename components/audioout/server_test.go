package audioout_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioout/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

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
	errPlayStreamFailed = errors.New("can't play stream")
	errPropertiesFailed = errors.New("can't get properties")
	errGetStatusFailed  = errors.New("can't get status")
	errRecvFailed       = errors.New("recv fail")
)

// Mock client-streaming server for PlayStream RPC. Pops messages from the msgs
// slice; returns recvErr if set, otherwise io.EOF when the slice is empty.
type playStreamServer struct {
	grpc.ServerStream
	ctx     context.Context
	msgs    []*pb.PlayStreamRequest
	recvErr error
	resp    *pb.PlayStreamResponse
}

func (s *playStreamServer) Context() context.Context { return s.ctx }

func (s *playStreamServer) Recv() (*pb.PlayStreamRequest, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	m := s.msgs[0]
	s.msgs = s.msgs[1:]
	return m, nil
}

func (s *playStreamServer) SendAndClose(r *pb.PlayStreamResponse) error {
	s.resp = r
	return nil
}

func initMsg(name string, info *commonpb.AudioInfo) *pb.PlayStreamRequest {
	return &pb.PlayStreamRequest{
		Payload: &pb.PlayStreamRequest_Init{
			Init: &pb.PlayStreamInit{Name: name, AudioInfo: info},
		},
	}
}

func chunkMsg(b []byte) *pb.PlayStreamRequest {
	return &pb.PlayStreamRequest{
		Payload: &pb.PlayStreamRequest_AudioChunk{AudioChunk: b},
	}
}

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

	t.Run("PlayStream", func(t *testing.T) {
		chunk1 := []byte{1, 2, 3, 4}
		chunk2 := []byte{5, 6, 7, 8}

		t.Run("success", func(t *testing.T) {
			var receivedInfo *rutils.AudioInfo
			var receivedChunks [][]byte
			injectAudioOut.PlayStreamFunc = func(ctx context.Context, info *rutils.AudioInfo,
				chunks <-chan []byte, extra map[string]interface{},
			) error {
				receivedInfo = info
				for c := range chunks {
					receivedChunks = append(receivedChunks, c)
				}
				return nil
			}

			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &playStreamServer{
				ctx: cancelCtx,
				msgs: []*pb.PlayStreamRequest{
					initMsg(testAudioOutName, audioInfo),
					chunkMsg(chunk1),
					chunkMsg(chunk2),
				},
			}

			err := audioOutServer.PlayStream(s)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, s.resp, test.ShouldNotBeNil)
			test.That(t, receivedInfo, test.ShouldNotBeNil)
			test.That(t, receivedInfo.Codec, test.ShouldEqual, rutils.CodecPCM16)
			test.That(t, receivedInfo.SampleRateHz, test.ShouldEqual, 44100)
			test.That(t, receivedInfo.NumChannels, test.ShouldEqual, 2)
			test.That(t, receivedChunks, test.ShouldResemble, [][]byte{chunk1, chunk2})
		})

		t.Run("first message not init", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &playStreamServer{
				ctx:  cancelCtx,
				msgs: []*pb.PlayStreamRequest{chunkMsg(chunk1)},
			}
			err := audioOutServer.PlayStream(s)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "PlayStreamInit")
		})

		t.Run("resource not found", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &playStreamServer{
				ctx:  cancelCtx,
				msgs: []*pb.PlayStreamRequest{initMsg("does_not_exist", audioInfo)},
			}
			err := audioOutServer.PlayStream(s)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
		})

		t.Run("recv error on first message", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &playStreamServer{
				ctx:     cancelCtx,
				recvErr: errRecvFailed,
			}
			err := audioOutServer.PlayStream(s)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errRecvFailed.Error())
		})

		t.Run("playstream impl error does not deadlock", func(t *testing.T) {
			injectAudioOut.PlayStreamFunc = func(ctx context.Context, info *rutils.AudioInfo,
				chunks <-chan []byte, extra map[string]interface{},
			) error {
				return errPlayStreamFailed
			}

			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &playStreamServer{
				ctx: cancelCtx,
				msgs: []*pb.PlayStreamRequest{
					initMsg(testAudioOutName, audioInfo),
					chunkMsg(chunk1),
					chunkMsg(chunk2),
				},
			}

			done := make(chan error, 1)
			go func() { done <- audioOutServer.PlayStream(s) }()

			select {
			case err := <-done:
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, errPlayStreamFailed.Error())
			case <-time.After(2 * time.Second):
				t.Fatal("PlayStream did not return — deadlock regression")
			}
		})

		injectAudioOut.PlayStreamFunc = nil
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

	t.Run("GetStatus", func(t *testing.T) {
		_, err := audioOutServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: "audio_out3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		resp, err := audioOutServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testAudioOutName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Result.AsMap(), test.ShouldBeEmpty)

		expectedStatus := map[string]interface{}{"key": "value", "count": float64(42)}
		injectAudioOut.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return expectedStatus, nil
		}
		resp, err = audioOutServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testAudioOutName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Result.AsMap(), test.ShouldResemble, expectedStatus)

		injectAudioOut.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, errGetStatusFailed
		}
		_, err = audioOutServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testAudioOutName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetStatusFailed.Error())
		injectAudioOut.StatusFunc = nil
	})
}
