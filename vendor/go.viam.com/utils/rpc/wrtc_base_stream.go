package rpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	protov1 "github.com/golang/protobuf/proto" //nolint:staticcheck
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"go.viam.com/utils"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

type webrtcBaseStream struct {
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	stream        *webrtcpb.Stream
	msgCh         chan []byte
	onDone        func(id uint64)
	err           error
	recvClosed    atomic.Bool
	closed        atomic.Bool
	logger        utils.ZapCompatibleLogger
	packetBuf     bytes.Buffer
	activeSenders sync.WaitGroup
}

// newWebRTCBaseStream makes a new webrtcBaseStream where the context should originate
// from the owning channel where if the channel is closed, all operations
// on this stream should be canceled with their callers subsequently
// notified.
func newWebRTCBaseStream(
	ctx context.Context,
	cancelCtx func(),
	stream *webrtcpb.Stream,
	onDone func(id uint64),
	logger utils.ZapCompatibleLogger,
) *webrtcBaseStream {
	bs := webrtcBaseStream{
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
func (s *webrtcBaseStream) Context() context.Context {
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
func (s *webrtcBaseStream) RecvMsg(m interface{}) error {
	if v1Msg, ok := m.(protov1.Message); ok {
		m = protov1.MessageV2(v1Msg)
	}

	checkLastOrErr := func(origErr error) ([]byte, error) {
		// checkLastOrErr is an attempt to return the most informative, relevant
		// error message to the user when we cannot receive a message in the base
		// stream for some reason.
		select {
		case msgBytes, ok := <-s.msgCh:
			if ok {
				return msgBytes, nil
			}
			s.mu.Lock()
			if s.err != nil {
				s.mu.Unlock()
				if errors.Is(s.err, errExpectedClosure) {
					return nil, io.EOF
				}
				return nil, s.err
			}
			s.mu.Unlock()
			if origErr == nil {
				return nil, io.EOF
			}
			return nil, origErr
		default:
			return nil, nil
		}
	}

	// RSDK-4473: There are three ways a stream can be signaled that it should
	// give up receiving a message:
	//   - `s.ctx` errors due to a timeout or some other grpc/webrtc error.
	//   - `s.ctx` is canceled when the underlying webrtc data channel
	//      connection experiences an error.
	//   - `s.msgCh` is closed when the stream, channel or connection has been
	//      closed.
	select {
	case <-s.ctx.Done():
		msgBytes, err := checkLastOrErr(s.ctx.Err())
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
		_, err := checkLastOrErr(nil)
		return err
	}
}

// Must _not_ be holding the `webrtcBaseStream.mu` mutex.
func (s *webrtcBaseStream) CloseRecv() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeRecv()
}

// Must be called with the `webrtcBaseStream.mu` mutex held.
func (s *webrtcBaseStream) closeRecv() {
	if !s.recvClosed.CompareAndSwap(false, true) {
		return
	}
	s.activeSenders.Wait()
	close(s.msgCh)
}

// Must be called with the `webrtcBaseStream.mu` mutex held.
func (s *webrtcBaseStream) close() {
	s.closeWithError(nil, false)
}

func (s *webrtcBaseStream) Closed() bool {
	return s.closed.Load()
}

// Must be called with the `webrtcBaseStream.mu` mutex held.
func (s *webrtcBaseStream) closeFromTrailers(err error) {
	s.closeWithError(err, err == nil)
}

var errExpectedClosure = errors.New("internal: closed via normal flow of operations")

// Must be called with the `webrtcBaseStream.mu` mutex held.
func (s *webrtcBaseStream) closeWithError(err error, expected bool) {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.closeRecv()
	if err != nil {
		s.err = err
	} else if expected {
		s.err = errExpectedClosure
	}
	s.cancel()
	s.onDone(s.stream.GetId())
}

func (s *webrtcBaseStream) processMessage(msg *webrtcpb.PacketMessage) ([]byte, bool) {
	if len(msg.GetData()) == 0 && msg.GetEom() {
		return []byte{}, true
	}
	if len(msg.GetData())+s.packetBuf.Len() > MaxMessageSize {
		s.packetBuf.Reset()
		s.logger.Errorf("message size larger than max %d; discarding", MaxMessageSize)
		return nil, false
	}
	s.packetBuf.Write(msg.GetData())
	if msg.GetEom() {
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
	if mdProto == nil || mdProto.Md == nil || len(mdProto.GetMd()) == 0 {
		return nil
	}
	result := make(metadata.MD, len(mdProto.GetMd()))
	for key, values := range mdProto.GetMd() {
		valuesCopy := make([]string, len(values.GetValues()))
		copy(valuesCopy, values.GetValues())
		result[key] = valuesCopy
	}
	return result
}
