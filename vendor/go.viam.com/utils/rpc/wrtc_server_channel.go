package rpc

import (
	"context"
	"strings"
	"sync"

	"github.com/viamrobotics/webrtc/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"go.viam.com/utils"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

// A webrtcServerChannel reflects the server end of a gRPC connection serviced over
// a WebRTC data channel.
type webrtcServerChannel struct {
	*webrtcBaseChannel
	mu sync.Mutex
	// TODO(GOUT-11): Handle auth; authAudience is an approximation of the authenticated
	// entity due to the lack of the signaling protocol indicating to the answerer who
	// the entity. There is no reason to extend the protocol right now since we intend
	// to support some for of authentication in the presence of untrusted signalers.
	authAudience string
	server       *webrtcServer
	streams      map[uint64]*webrtcServerStream
}

// newWebRTCServerChannel wraps the given WebRTC data channel to be used as the server end
// of a gRPC connection.
func newWebRTCServerChannel(
	server *webrtcServer,
	peerConn *webrtc.PeerConnection,
	dataChannel *webrtc.DataChannel,
	authAudience []string,
	logger utils.ZapCompatibleLogger,
) *webrtcServerChannel {
	base := newBaseChannel(
		server.workers.Context(),
		peerConn,
		dataChannel,
		server,
		nil,
		logger,
	)
	ch := &webrtcServerChannel{
		authAudience:      strings.Join(authAudience, ":"),
		webrtcBaseChannel: base,
		server:            server,
		streams:           make(map[uint64]*webrtcServerStream),
	}
	dataChannel.OnMessage(ch.onChannelMessage)
	return ch
}

func (ch *webrtcServerChannel) writeHeaders(stream *webrtcpb.Stream, headers *webrtcpb.ResponseHeaders) error {
	return ch.webrtcBaseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Headers{
			Headers: headers,
		},
	})
}

func (ch *webrtcServerChannel) writeMessage(stream *webrtcpb.Stream, msg *webrtcpb.ResponseMessage) error {
	return ch.webrtcBaseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Message{
			Message: msg,
		},
	})
}

func (ch *webrtcServerChannel) writeTrailers(stream *webrtcpb.Stream, trailers *webrtcpb.ResponseTrailers) error {
	return ch.webrtcBaseChannel.write(&webrtcpb.Response{
		Stream: stream,
		Type: &webrtcpb.Response_Trailers{
			Trailers: trailers,
		},
	})
}

func (ch *webrtcServerChannel) removeStreamByID(id uint64) {
	ch.mu.Lock()
	delete(ch.streams, id)
	ch.mu.Unlock()
}

func (ch *webrtcServerChannel) onChannelMessage(msg webrtc.DataChannelMessage) {
	req := &webrtcpb.Request{}
	err := proto.Unmarshal(msg.Data, req)
	if err != nil {
		ch.webrtcBaseChannel.logger.Errorw("error unmarshaling message; discarding", "error", err)
		return
	}
	stream := req.GetStream()
	if stream == nil {
		ch.webrtcBaseChannel.logger.Error("no stream, discard request")
		return
	}

	id := stream.GetId()
	logger := utils.AddFieldsToLogger(ch.webrtcBaseChannel.logger, "id", id)

	ch.mu.Lock()
	serverStream, ok := ch.streams[id]
	if !ok {
		if len(ch.streams) == WebRTCMaxStreamCount {
			logger.Error(errWebRTCMaxStreams)
			ch.mu.Unlock()
			return
		}
		// peek headers for timeout
		headers, ok := req.GetType().(*webrtcpb.Request_Headers)
		if !ok || headers.Headers == nil {
			logger.Debugf("expected headers as first message but got %T, discard request", req.GetType())
			ch.mu.Unlock()
			return
		}

		handlerCtx := metadata.NewIncomingContext(ch.ctx, metadataFromProto(headers.Headers.GetMetadata()))
		timeout := headers.Headers.GetTimeout().AsDuration()
		var cancelCtx func()
		if timeout == 0 {
			handlerCtx, cancelCtx = context.WithCancel(handlerCtx)
		} else {
			handlerCtx, cancelCtx = context.WithTimeout(handlerCtx, timeout)
		}
		handlerCtx = ContextWithPeerConnection(handlerCtx, ch.peerConn)

		// TODO(GOUT-11): Handle auth; right now we assume successful auth to the signaler
		// implies that auth should be allowed here, which is not 100% true.
		// TODO(RSDK-890): use the correct entity (sub), not the audience (hosts)
		handlerCtx = ContextWithAuthEntity(handlerCtx, EntityInfo{Entity: ch.authAudience})

		serverStream = newWebRTCServerStream(handlerCtx, cancelCtx, headers.Headers.GetMethod(), ch, stream, ch.removeStreamByID, logger)
		ch.streams[id] = serverStream
	}
	ch.mu.Unlock()

	serverStream.onRequest(req)
}
