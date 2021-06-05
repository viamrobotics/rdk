package rpcwebrtc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
)

var (
	// MaxStreamCount is the max number of streams a channel can have.
	MaxStreamCount = 256
	errMaxStreams  = errors.New("stream limit hit")
)

// A ClientChannel reflects the client end of a gRPC connection serviced over
// a WebRTC data channel.
type ClientChannel struct {
	*baseChannel
	mu              sync.Mutex
	streamIDCounter uint64
	streams         map[uint64]activeClienStream
}

type activeClienStream struct {
	cs     *ClientStream
	cancel func()
}

// NewClientChannel wraps the given WebRTC data channel to be used as the client end
// of a gRPC connection.
func NewClientChannel(
	peerConn *webrtc.PeerConnection,
	dataChannel *webrtc.DataChannel,
	logger golog.Logger,
) *ClientChannel {
	base := newBaseChannel(
		context.Background(),
		peerConn,
		dataChannel,
		nil,
		logger,
	)
	ch := &ClientChannel{
		baseChannel: base,
		streams:     map[uint64]activeClienStream{},
	}
	dataChannel.OnMessage(ch.onChannelMessage)
	return ch
}

// Close closes all streams and the underlying channel.
func (ch *ClientChannel) Close() error {
	ch.mu.Lock()
	for _, s := range ch.streams {
		s.cancel()
	}
	ch.mu.Unlock()
	return ch.baseChannel.Close()
}

// Invoke sends the RPC request on the wire and returns after response is
// received.  This is typically called by generated code.
//
// All errors returned by Invoke are compatible with the status package.
func (ch *ClientChannel) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	clientStream, err := ch.newStream(ctx, ch.nextStreamID())
	if err != nil {
		return err
	}

	if err := clientStream.writeHeaders(makeRequestHeaders(ctx, method)); err != nil {
		return err
	}

	if err := clientStream.writeMessage(args, true); err != nil {
		return err
	}

	return clientStream.RecvMsg(reply)
}

// NewStream creates a new Stream for the client side. This is typically
// called by generated code. ctx is used for the lifetime of the stream.
//
// To ensure resources are not leaked due to the stream returned, one of the following
// actions must be performed:
//
//      1. Call Close on the ClientConn.
//      2. Cancel the context provided.
//      3. Call RecvMsg until a non-nil error is returned. A protobuf-generated
//         client-streaming RPC, for instance, might use the helper function
//         CloseAndRecv (note that CloseSend does not Recv, therefore is not
//         guaranteed to release all resources).
//      4. Receive a non-nil, non-io.EOF error from Header or SendMsg.
//
// If none of the above happen, a goroutine and a context will be leaked, and grpc
// will not call the optionally-configured stats handler with a stats.End message.
func (ch *ClientChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	clientStream, err := ch.newStream(ctx, ch.nextStreamID())
	if err != nil {
		return nil, err
	}

	if err := clientStream.writeHeaders(makeRequestHeaders(ctx, method)); err != nil {
		return nil, err
	}

	return clientStream, nil
}

func makeRequestHeaders(ctx context.Context, method string) *webrtcpb.RequestHeaders {
	var headersMD metadata.MD
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		headersMD = make(metadata.MD, len(md))
		for k, v := range headersMD {
			headersMD[k] = v
		}
	}
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	return &webrtcpb.RequestHeaders{
		Method:   method,
		Metadata: metadataToProto(headersMD),
		Timeout:  durationpb.New(timeout),
	}
}

func (ch *ClientChannel) nextStreamID() *webrtcpb.Stream {
	return &webrtcpb.Stream{
		Id: atomic.AddUint64(&ch.streamIDCounter, 1),
	}
}

func (ch *ClientChannel) removeStreamByID(id uint64) {
	ch.mu.Lock()
	delete(ch.streams, id)
	ch.mu.Unlock()
}

func (ch *ClientChannel) newStream(ctx context.Context, stream *webrtcpb.Stream) (*ClientStream, error) {
	id := stream.Id
	ch.mu.Lock()
	activeStream, ok := ch.streams[id]
	if !ok {
		if len(ch.streams) == MaxStreamCount {
			return nil, errMaxStreams
		}
		ctx, cancel := context.WithCancel(ctx)
		clientStream := NewClientStream(
			ctx,
			ch,
			stream,
			ch.removeStreamByID,
			ch.baseChannel.logger.With("id", id),
		)
		activeStream = activeClienStream{clientStream, cancel}
		ch.streams[id] = activeStream
	}
	ch.mu.Unlock()
	return activeStream.cs, nil
}

func (ch *ClientChannel) onChannelMessage(msg webrtc.DataChannelMessage) {
	resp := &webrtcpb.Response{}
	err := proto.Unmarshal(msg.Data, resp)
	if err != nil {
		ch.baseChannel.logger.Errorw("error unmarshaling message; discarding", "error", err)
		return
	}

	stream := resp.Stream
	if stream == nil {
		ch.baseChannel.logger.Error("no stream id; discarding")
		return
	}

	id := stream.Id
	ch.mu.Lock()
	activeStream, ok := ch.streams[id]
	if !ok {
		ch.baseChannel.logger.Errorw("no stream for id; discarding", "id", id)
		ch.mu.Unlock()
		return
	}
	ch.mu.Unlock()

	activeStream.cs.onResponse(resp)
}

func (ch *ClientChannel) writeHeaders(stream *webrtcpb.Stream, headers *webrtcpb.RequestHeaders) error {
	return ch.baseChannel.write(&webrtcpb.Request{
		Stream: stream,
		Type: &webrtcpb.Request_Headers{
			Headers: headers,
		},
	})
}

func (ch *ClientChannel) writeMessage(stream *webrtcpb.Stream, msg *webrtcpb.RequestMessage) error {
	return ch.baseChannel.write(&webrtcpb.Request{
		Stream: stream,
		Type: &webrtcpb.Request_Message{
			Message: msg,
		},
	})
}
