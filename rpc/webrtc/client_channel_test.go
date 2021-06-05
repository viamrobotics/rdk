package rpcwebrtc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	pbstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/test"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	"go.viam.com/core/testutils"
)

func TestClientChannel(t *testing.T) {
	testutils.SkipUnlessInternet(t)
	logger := golog.NewTestLogger(t)
	pc1, pc2, dc1, dc2 := setupPeers(t)

	clientCh := NewClientChannel(pc1, dc1, logger)
	defer func() {
		test.That(t, clientCh.Close(), test.ShouldBeNil)
	}()
	serverCh := newBaseChannel(context.Background(), pc2, dc2, nil, logger)
	defer func() {
		test.That(t, serverCh.Close(), test.ShouldBeNil)
	}()

	<-clientCh.Ready()
	<-serverCh.Ready()

	someStatus, _ := status.FromError(errors.New("ouch"))

	someStatusMd, err := proto.Marshal(someStatus.Proto())
	test.That(t, err, test.ShouldBeNil)

	someOtherStatus, _ := status.FromError(errors.New("ouchie"))

	someOtherStatusMd, err := proto.Marshal(someOtherStatus.Proto())
	test.That(t, err, test.ShouldBeNil)

	var expectedMessagesMu sync.Mutex
	expectedMessages := []*webrtcpb.Request{
		{
			Stream: &webrtcpb.Stream{Id: 1},
			Type: &webrtcpb.Request_Headers{
				Headers: &webrtcpb.RequestHeaders{
					Method:  "thing",
					Timeout: durationpb.New(0),
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 1},
			Type: &webrtcpb.Request_Message{
				Message: &webrtcpb.RequestMessage{
					HasMessage: true,
					PacketMessage: &webrtcpb.PacketMessage{
						Data: someStatusMd,
						Eom:  true,
					},
					Eos: true,
				},
			},
		},
	}

	expectedTrailer := metadata.MD{}

	var rejected sync.WaitGroup
	var hasTimeout, errorAfterMessage, trailersOnly, rejectAll bool
	idCounter := uint64(1)
	serverCh.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		expectedMessagesMu.Lock()
		defer expectedMessagesMu.Unlock()
		if rejectAll {
			rejected.Done()
			return
		}
		test.That(t, serverCh.write(someStatus.Proto()), test.ShouldBeNil) // unexpected proto
		req := &webrtcpb.Request{}
		test.That(t, proto.Unmarshal(msg.Data, req), test.ShouldBeNil)
		test.That(t, expectedMessages, test.ShouldNotBeEmpty)
		expected := expectedMessages[0]
		if hasTimeout {
			if header, ok := req.Type.(*webrtcpb.Request_Headers); ok {
				test.That(t, header.Headers.Timeout.AsDuration(), test.ShouldNotBeZeroValue)
				header.Headers.Timeout = nil
			}
		}
		logger.Debugw("comparing", "expected", expected, "actual", req)
		test.That(t, proto.Equal(expected, req), test.ShouldBeTrue)
		expectedMessages = expectedMessages[1:]
		if len(expectedMessages) == 0 {
			if !trailersOnly {
				test.That(t, serverCh.write(&webrtcpb.Response{
					Stream: &webrtcpb.Stream{Id: idCounter},
					Type:   &webrtcpb.Response_Headers{},
				}), test.ShouldBeNil)
			}

			if !trailersOnly && !errorAfterMessage {
				test.That(t, serverCh.write(&webrtcpb.Response{
					Stream: &webrtcpb.Stream{Id: idCounter},
					Type: &webrtcpb.Response_Message{
						Message: &webrtcpb.ResponseMessage{
							PacketMessage: &webrtcpb.PacketMessage{
								Data: someOtherStatusMd,
								Eom:  true,
							},
						},
					},
				}), test.ShouldBeNil)
			}

			respStatus := ErrorToStatus(nil)
			if errorAfterMessage {
				respStatus = status.New(codes.InvalidArgument, "whoops")
			}
			test.That(t, serverCh.write(&webrtcpb.Response{
				Stream: &webrtcpb.Stream{Id: idCounter},
				Type: &webrtcpb.Response_Trailers{
					Trailers: &webrtcpb.ResponseTrailers{
						Status:   respStatus.Proto(),
						Metadata: metadataToProto(expectedTrailer),
					},
				},
			}), test.ShouldBeNil)
			idCounter++

			// Ignore bad streams
			test.That(t, serverCh.write(&webrtcpb.Response{
				Stream: &webrtcpb.Stream{Id: 1000},
				Type:   &webrtcpb.Response_Headers{},
			}), test.ShouldBeNil)
			test.That(t, serverCh.write(&webrtcpb.Response{
				Stream: &webrtcpb.Stream{Id: 1000},
				Type: &webrtcpb.Response_Message{
					Message: &webrtcpb.ResponseMessage{
						PacketMessage: &webrtcpb.PacketMessage{
							Data: someOtherStatusMd,
							Eom:  true,
						},
					},
				},
			}), test.ShouldBeNil)
			test.That(t, serverCh.write(&webrtcpb.Response{
				Stream: &webrtcpb.Stream{Id: 1000},
				Type: &webrtcpb.Response_Trailers{
					Trailers: &webrtcpb.ResponseTrailers{
						Status:   ErrorToStatus(nil).Proto(),
						Metadata: metadataToProto(expectedTrailer),
					},
				},
			}), test.ShouldBeNil)
		}
	})

	var respStatus pbstatus.Status
	test.That(t, clientCh.Invoke(context.Background(), "thing", someStatus.Proto(), &respStatus), test.ShouldBeNil)
	test.That(t, status.FromProto(&respStatus).Code(), test.ShouldEqual, someOtherStatus.Code())
	test.That(t, status.FromProto(&respStatus).Message(), test.ShouldEqual, someOtherStatus.Message())
	test.That(t, status.FromProto(&respStatus).Details(), test.ShouldResemble, someOtherStatus.Details())

	expectedMessagesMu.Lock()
	hasTimeout = true
	expectedMessages = []*webrtcpb.Request{
		{
			Stream: &webrtcpb.Stream{Id: 2},
			Type: &webrtcpb.Request_Headers{
				Headers: &webrtcpb.RequestHeaders{
					Method: "thing",
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 2},
			Type: &webrtcpb.Request_Message{
				Message: &webrtcpb.RequestMessage{
					HasMessage: true,
					PacketMessage: &webrtcpb.PacketMessage{
						Data: someStatusMd,
						Eom:  true,
					},
					Eos: true,
				},
			},
		},
	}
	expectedMessagesMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	test.That(t, clientCh.Invoke(ctx, "thing", someStatus.Proto(), &respStatus), test.ShouldBeNil)
	test.That(t, status.FromProto(&respStatus).Code(), test.ShouldEqual, someOtherStatus.Code())
	test.That(t, status.FromProto(&respStatus).Message(), test.ShouldEqual, someOtherStatus.Message())
	test.That(t, status.FromProto(&respStatus).Details(), test.ShouldResemble, someOtherStatus.Details())

	expectedMessagesMu.Lock()
	hasTimeout = false
	expectedMessages = []*webrtcpb.Request{
		{
			Stream: &webrtcpb.Stream{Id: 3},
			Type: &webrtcpb.Request_Headers{
				Headers: &webrtcpb.RequestHeaders{
					Method:  "thing",
					Timeout: durationpb.New(0),
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 3},
			Type: &webrtcpb.Request_Message{
				Message: &webrtcpb.RequestMessage{
					HasMessage: true,
					PacketMessage: &webrtcpb.PacketMessage{
						Data: someStatusMd,
						Eom:  true,
					},
					Eos: true,
				},
			},
		},
	}
	expectedMessagesMu.Unlock()

	test.That(t, clientCh.Invoke(context.Background(), "thing", someStatus.Proto(), &respStatus), test.ShouldBeNil)
	test.That(t, status.FromProto(&respStatus).Code(), test.ShouldEqual, someOtherStatus.Code())
	test.That(t, status.FromProto(&respStatus).Message(), test.ShouldEqual, someOtherStatus.Message())
	test.That(t, status.FromProto(&respStatus).Details(), test.ShouldResemble, someOtherStatus.Details())

	expectedMessagesMu.Lock()
	errorAfterMessage = true
	expectedMessages = []*webrtcpb.Request{
		{
			Stream: &webrtcpb.Stream{Id: 4},
			Type: &webrtcpb.Request_Headers{
				Headers: &webrtcpb.RequestHeaders{
					Method:  "thing",
					Timeout: durationpb.New(0),
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 4},
			Type: &webrtcpb.Request_Message{
				Message: &webrtcpb.RequestMessage{
					HasMessage: true,
					PacketMessage: &webrtcpb.PacketMessage{
						Data: someStatusMd,
						Eom:  true,
					},
					Eos: true,
				},
			},
		},
	}
	expectedMessagesMu.Unlock()

	err = clientCh.Invoke(context.Background(), "thing", someStatus.Proto(), &respStatus)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Convert(err).Code(), test.ShouldEqual, codes.InvalidArgument)
	test.That(t, status.Convert(err).Message(), test.ShouldEqual, "whoops")

	expectedMessagesMu.Lock()
	trailersOnly = true
	errorAfterMessage = true
	expectedMessages = []*webrtcpb.Request{
		{
			Stream: &webrtcpb.Stream{Id: 5},
			Type: &webrtcpb.Request_Headers{
				Headers: &webrtcpb.RequestHeaders{
					Method:  "thing",
					Timeout: durationpb.New(0),
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 5},
			Type: &webrtcpb.Request_Message{
				Message: &webrtcpb.RequestMessage{
					HasMessage: true,
					PacketMessage: &webrtcpb.PacketMessage{
						Data: someStatusMd,
						Eom:  true,
					},
					Eos: true,
				},
			},
		},
	}
	expectedMessagesMu.Unlock()

	err = clientCh.Invoke(context.Background(), "thing", someStatus.Proto(), &respStatus)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Convert(err).Code(), test.ShouldEqual, codes.InvalidArgument)
	test.That(t, status.Convert(err).Message(), test.ShouldEqual, "whoops")

	expectedMessagesMu.Lock()
	rejectAll = true
	expectedMessagesMu.Unlock()

	rejected.Add(2)
	clientErr := make(chan error)
	go func() {
		clientErr <- clientCh.Invoke(context.Background(), "thing", someStatus.Proto(), &respStatus)
	}()
	rejected.Wait()
	test.That(t, clientCh.Close(), test.ShouldBeNil)
	test.That(t, <-clientErr, test.ShouldEqual, context.Canceled)
}
