package rpcwebrtc

import (
	"bytes"
	"context"
	"io"
	"sync"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"

	"github.com/edaniels/golog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type baseStream struct {
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	stream     *webrtcpb.Stream
	msgCh      chan []byte
	onDone     func(id uint64)
	err        error
	recvClosed bool
	closed     bool
	logger     golog.Logger
	packetBuf  bytes.Buffer
}

// newBaseStream makes a new baseStream where the context should originate
// from the owning channel where if the channel is closed, all operations
// on this stream should be canceled with their callers subsequently
// notified.
func newBaseStream(
	ctx context.Context,
	cancelCtx func(),
	stream *webrtcpb.Stream,
	onDone func(id uint64),
	logger golog.Logger,
) *baseStream {
	bs := baseStream{
		ctx:    ctx,
		cancel: cancelCtx,
		stream: stream,
		onDone: onDone,
		logger: logger,
	}
	bs.msgCh = make(chan []byte, 1)
	return &bs
}

// Context returns the context for this stream.
func (s *baseStream) Context() context.Context {
	return s.ctx
}

// RecvMsg blocks until it receives a message into m or the stream is
// done. It returns io.EOF when the stream completes successfully. On
// any other error, the stream is aborted and the error contains the RPC
// status.
//
// It is safe to have a goroutine calling SendMsg and another goroutine
// calling RecvMsg on the same stream at the same time, but it is not
// safe to call RecvMsg on the same stream in different goroutines.
func (s *baseStream) RecvMsg(m interface{}) error {
	checkLastOrErr := func() ([]byte, error) {
		select {
		case msgBytes, ok := <-s.msgCh:
			if ok {
				return msgBytes, nil
			}
			s.mu.Lock()
			if s.err != nil {
				s.mu.Unlock()
				return nil, s.err
			}
			s.mu.Unlock()
			return nil, io.EOF
		default:
			return nil, nil
		}
	}
	select {
	case <-s.ctx.Done():
		msgBytes, err := checkLastOrErr()
		if err != nil {
			return err
		}
		if msgBytes != nil {
			return proto.Unmarshal(msgBytes, m.(proto.Message))
		}
		return s.ctx.Err()
	case msgBytes, ok := <-s.msgCh:
		if ok {
			return proto.Unmarshal(msgBytes, m.(proto.Message))
		}
		_, err := checkLastOrErr()
		return err
	}
}

func (s *baseStream) CloseRecv() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeRecv()
}

func (s *baseStream) closeRecv() {
	if s.recvClosed {
		return
	}
	s.recvClosed = true
	close(s.msgCh)
}

func (s *baseStream) close() {
	s.closeWithRecvError(nil)
}

func (s *baseStream) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *baseStream) closeWithRecvError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closeRecv()
	s.closed = true
	if err != nil {
		s.err = err
	}
	s.cancel()
	s.onDone(s.stream.Id)
}

func (s *baseStream) processMessage(msg *webrtcpb.PacketMessage) ([]byte, bool) {
	if len(msg.Data) == 0 && msg.Eom {
		return nil, true
	}
	if len(msg.Data)+s.packetBuf.Len() > MaxMessageSize {
		s.packetBuf.Reset()
		s.logger.Errorf("message size larger than max %d; discarding", MaxMessageSize)
		return nil, false
	}
	s.packetBuf.Write(msg.Data)
	if msg.Eom {
		data := make([]byte, s.packetBuf.Len())
		copy(data, s.packetBuf.Bytes())
		s.packetBuf.Reset()
		return data, true
	}
	return nil, false
}

func metadataToProto(md metadata.MD) *webrtcpb.Metadata {
	if md == nil || md.Len() == 0 {
		return nil
	}
	result := make(map[string]*webrtcpb.Strings, md.Len())
	for key, values := range md {
		result[key] = &webrtcpb.Strings{
			Values: values,
		}
	}
	return &webrtcpb.Metadata{
		Md: result,
	}
}

func metadataFromProto(mdProto *webrtcpb.Metadata) metadata.MD {
	if mdProto == nil || mdProto.Md == nil || len(mdProto.Md) == 0 {
		return nil
	}
	result := make(metadata.MD, len(mdProto.Md))
	for key, values := range mdProto.Md {
		valuesCopy := make([]string, len(values.Values))
		copy(valuesCopy, values.Values)
		result[key] = valuesCopy
	}
	return result
}
