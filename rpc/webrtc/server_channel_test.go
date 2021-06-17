package rpcwebrtc

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/test"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	"go.viam.com/core/testutils"
)

func TestServerChannel(t *testing.T) {
	testutils.SkipUnlessInternet(t)
	logger := golog.NewTestLogger(t)
	pc1, pc2, dc1, dc2 := setupPeers(t)

	clientCh := NewClientChannel(pc1, dc1, logger)
	defer func() {
		test.That(t, clientCh.Close(), test.ShouldBeNil)
	}()

	server := NewServer(logger)
	// use signaling server just as some random service to test against.
	// It helps that it is in our package.
	signalServer := NewSignalingServer()
	server.RegisterService(
		&webrtcpb.SignalingService_ServiceDesc,
		signalServer,
	)

	serverCh := NewServerChannel(server, pc2, dc2, logger)
	defer func() {
		test.That(t, serverCh.Close(), test.ShouldBeNil)
	}()

	<-clientCh.Ready()
	<-serverCh.Ready()

	someStatus, _ := status.FromError(errors.New("ouch"))

	// bad data
	test.That(t, clientCh.write(someStatus.Proto()), test.ShouldBeNil)  // unexpected proto
	test.That(t, clientCh.write(&webrtcpb.Request{}), test.ShouldBeNil) // bad request
	test.That(t, clientCh.writeMessage(&webrtcpb.Stream{                // message before headers
		Id: 1,
	}, &webrtcpb.RequestMessage{}), test.ShouldBeNil)

	var expectedMessagesMu sync.Mutex
	expectedMessages := []*webrtcpb.Response{
		{
			Stream: &webrtcpb.Stream{Id: 1},
			Type: &webrtcpb.Response_Headers{
				Headers: &webrtcpb.ResponseHeaders{},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 1},
			Type: &webrtcpb.Response_Trailers{
				Trailers: &webrtcpb.ResponseTrailers{
					Status: status.New(codes.Unimplemented, codes.Unimplemented.String()).Proto(),
				},
			},
		},
	}
	messagesRead := make(chan struct{})
	clientCh.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		expectedMessagesMu.Lock()
		defer expectedMessagesMu.Unlock()
		req := &webrtcpb.Response{}
		test.That(t, proto.Unmarshal(msg.Data, req), test.ShouldBeNil)
		logger.Debugw("got message", "actual", req)
		test.That(t, expectedMessages, test.ShouldNotBeEmpty)
		expected := expectedMessages[0]
		logger.Debugw("comparing", "expected", expected, "actual", req)
		test.That(t, proto.Equal(expected, req), test.ShouldBeTrue)
		expectedMessages = expectedMessages[1:]
		if len(expectedMessages) == 0 {
			close(messagesRead)
		}
	})

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{ // no method
		Id: 1,
	}, &webrtcpb.RequestHeaders{}), test.ShouldBeNil)

	<-messagesRead

	expectedMessagesMu.Lock()
	expectedMessages = []*webrtcpb.Response{
		{
			Stream: &webrtcpb.Stream{Id: 2},
			Type: &webrtcpb.Response_Headers{
				Headers: &webrtcpb.ResponseHeaders{},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 2},
			Type: &webrtcpb.Response_Trailers{
				Trailers: &webrtcpb.ResponseTrailers{
					Status: status.New(codes.InvalidArgument, "headers already received").Proto(),
				},
			},
		},
	}
	messagesRead = make(chan struct{})
	expectedMessagesMu.Unlock()

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{
		Id: 2,
	}, &webrtcpb.RequestHeaders{
		Method: "/proto.rpc.webrtc.v1.SignalingService/Call",
	}), test.ShouldBeNil)

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{
		Id: 2,
	}, &webrtcpb.RequestHeaders{
		Method: "/proto.rpc.webrtc.v1.SignalingService/Call",
	}), test.ShouldBeNil)

	<-messagesRead

	expectedMessagesMu.Lock()
	expectedMessages = []*webrtcpb.Response{
		{
			Stream: &webrtcpb.Stream{Id: 3},
			Type: &webrtcpb.Response_Headers{
				Headers: &webrtcpb.ResponseHeaders{},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 3},
			Type: &webrtcpb.Response_Trailers{
				Trailers: &webrtcpb.ResponseTrailers{
					Status: status.New(codes.Unknown, "EOF").Proto(),
				},
			},
		},
	}
	messagesRead = make(chan struct{})
	expectedMessagesMu.Unlock()

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{
		Id: 3,
	}, &webrtcpb.RequestHeaders{
		Method: "/proto.rpc.webrtc.v1.SignalingService/Call",
		Metadata: metadataToProto(metadata.MD{
			"rpc-host": []string{"yeehaw"},
		}),
	}), test.ShouldBeNil)

	test.That(t, clientCh.writeMessage(&webrtcpb.Stream{
		Id: 3,
	}, &webrtcpb.RequestMessage{
		Eos: true,
	}), test.ShouldBeNil)

	<-messagesRead

	respMd, err := proto.Marshal(&webrtcpb.CallResponse{Sdp: "world"})
	test.That(t, err, test.ShouldBeNil)

	expectedMessagesMu.Lock()
	expectedMessages = []*webrtcpb.Response{
		{
			Stream: &webrtcpb.Stream{Id: 4},
			Type: &webrtcpb.Response_Headers{
				Headers: &webrtcpb.ResponseHeaders{},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 4},
			Type: &webrtcpb.Response_Message{
				Message: &webrtcpb.ResponseMessage{
					PacketMessage: &webrtcpb.PacketMessage{
						Data: respMd,
						Eom:  true,
					},
				},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 4},
			Type: &webrtcpb.Response_Trailers{
				Trailers: &webrtcpb.ResponseTrailers{
					Status: ErrorToStatus(nil).Proto(),
				},
			},
		},
	}
	messagesRead = make(chan struct{})
	expectedMessagesMu.Unlock()

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{
		Id: 4,
	}, &webrtcpb.RequestHeaders{
		Method: "/proto.rpc.webrtc.v1.SignalingService/Call",
		Metadata: metadataToProto(metadata.MD{
			"rpc-host": []string{"yeehaw"},
		}),
	}), test.ShouldBeNil)

	reqMd, err := proto.Marshal(&webrtcpb.CallRequest{Sdp: "hello"})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, clientCh.writeMessage(&webrtcpb.Stream{
		Id: 4,
	}, &webrtcpb.RequestMessage{
		HasMessage: true,
		PacketMessage: &webrtcpb.PacketMessage{
			Data: reqMd,
			Eom:  true,
		},
		Eos: true,
	}), test.ShouldBeNil)

	offer, err := signalServer.callQueue.RecvOffer(context.Background(), "yeehaw")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, offer.Respond(context.Background(), CallAnswer{SDP: "world"}), test.ShouldBeNil)

	<-messagesRead

	expectedMessagesMu.Lock()
	expectedMessages = []*webrtcpb.Response{
		{
			Stream: &webrtcpb.Stream{Id: 5},
			Type: &webrtcpb.Response_Headers{
				Headers: &webrtcpb.ResponseHeaders{},
			},
		},
		{
			Stream: &webrtcpb.Stream{Id: 5},
			Type: &webrtcpb.Response_Trailers{
				Trailers: &webrtcpb.ResponseTrailers{
					Status: ErrorToStatus(errors.New("ohno")).Proto(),
				},
			},
		},
	}
	messagesRead = make(chan struct{})
	expectedMessagesMu.Unlock()

	test.That(t, clientCh.writeHeaders(&webrtcpb.Stream{
		Id: 5,
	}, &webrtcpb.RequestHeaders{
		Method: "/proto.rpc.webrtc.v1.SignalingService/Call",
		Metadata: metadataToProto(metadata.MD{
			"rpc-host": []string{"yeehaw"},
		}),
	}), test.ShouldBeNil)

	test.That(t, clientCh.writeMessage(&webrtcpb.Stream{
		Id: 5,
	}, &webrtcpb.RequestMessage{
		HasMessage: true,
		PacketMessage: &webrtcpb.PacketMessage{
			Data: reqMd,
			Eom:  true,
		},
		Eos: true,
	}), test.ShouldBeNil)

	offer, err = signalServer.callQueue.RecvOffer(context.Background(), "yeehaw")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, offer.Respond(context.Background(), CallAnswer{Err: errors.New("ohno")}), test.ShouldBeNil)

	<-messagesRead
}
