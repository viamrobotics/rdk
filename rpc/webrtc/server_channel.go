package rpcwebrtc

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
)

// A ServerChannel reflects the server end of a gRPC connection serviced over
// a WebRTC data channel.
type ServerChannel struct {
	*baseChannel
	mu      sync.Mutex
	server  *Server
	streams map[uint64]*ServerStream
}

// NewServerChannel wraps the given WebRTC data channel to be used as the server end
// of a gRPC connection.
func NewServerChannel(
	server *Server,
	peerConn *webrtc.PeerConnection,
	dataChannel *webrtc.DataChannel,
	logger golog.Logger,
) *ServerChannel {
	base := newBaseChannel(
		server.ctx,
		peerConn,
		dataChannel,
		func() { server.removePeer(peerConn) },
		logger,
	)
	ch := &ServerChannel{
		baseChannel: base,
		server:      server,
		streams:     make(map[uint64]*ServerStream),
	}
	dataChannel.OnMessage(ch.onChannelMessage)
	return ch
}

func (ch *ServerChannel) writeHeaders(stream *webrtcpb.Stream, headers *webrtcpb.ResponseHeaders) error {
	return ch.baseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Headers{
			Headers: headers,
		},
	})
}

func (ch *ServerChannel) writeMessage(stream *webrtcpb.Stream, msg *webrtcpb.ResponseMessage) error {
	return ch.baseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Message{
			Message: msg,
		},
	})
}

func (ch *ServerChannel) writeTrailers(stream *webrtcpb.Stream, trailers *webrtcpb.ResponseTrailers) error {
	return ch.baseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Trailers{
			Trailers: trailers,
		},
	})
}

func (ch *ServerChannel) removeStreamByID(id uint64) {
	ch.mu.Lock()
	delete(ch.streams, id)
	ch.mu.Unlock()
}

func (ch *ServerChannel) onChannelMessage(msg webrtc.DataChannelMessage) {
	req := &webrtcpb.Request{}
	err := proto.Unmarshal(msg.Data, req)
	if err != nil {
		ch.baseChannel.logger.Errorw("error unmarshaling message; discarding", "error", err)
		return
	}
	stream := req.GetStream()
	if stream == nil {
		ch.baseChannel.logger.Error("no stream, discard request")
		return
	}

	id := stream.Id
	logger := ch.baseChannel.logger.With("id", id)

	ch.mu.Lock()
	serverStream, ok := ch.streams[id]
	if !ok {
		if len(ch.streams) == MaxStreamCount {
			ch.baseChannel.logger.Error(errMaxStreams)
			return
		}
		// peek headers for timeout
		headers, ok := req.Type.(*webrtcpb.Request_Headers)
		if !ok || headers.Headers == nil {
			ch.baseChannel.logger.Errorf("expected headers as first message but got %T, discard request", req.Type)
			ch.mu.Unlock()
			return
		}

		handlerCtx := metadata.NewIncomingContext(ch.ctx, metadataFromProto(headers.Headers.Metadata))
		timeout := headers.Headers.Timeout.AsDuration()
		var cancelCtx func()
		if timeout == 0 {
			cancelCtx = func() {}
		} else {
			handlerCtx, cancelCtx = context.WithTimeout(handlerCtx, timeout)
		}

		serverStream = newServerStream(handlerCtx, cancelCtx, ch, stream, ch.removeStreamByID, logger)
		ch.streams[id] = serverStream
	}
	ch.mu.Unlock()

	serverStream.onRequest(req)
}
