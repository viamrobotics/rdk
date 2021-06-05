package rpcwebrtc

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/multierr"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var _ = grpc.ServerStream(&ServerStream{})

var (
	// ErrIllegalHeaderWrite indicates that setting header is illegal because of
	// the state of the stream.
	ErrIllegalHeaderWrite = errors.New("transport: the stream is done or WriteHeader was already called")
)

// A ServerStream is the high level gRPC streaming interface used for handling both
// unary and streaming call responses.
type ServerStream struct {
	*baseStream
	ch              *ServerChannel
	headersWritten  bool
	headersReceived bool
	header          metadata.MD
	trailer         metadata.MD
}

// newServerStream creates a gRPC stream from the given server channel with a
// unique identity in order to be able to recognize requests on a single
// underlying data channel.
func newServerStream(
	ctx context.Context,
	cancelCtx func(),
	channel *ServerChannel,
	stream *webrtcpb.Stream,
	onDone func(id uint64),
	logger golog.Logger,
) *ServerStream {
	bs := newBaseStream(ctx, cancelCtx, stream, onDone, logger)
	s := &ServerStream{
		baseStream: bs,
		ch:         channel,
	}
	return s
}

// SetHeader sets the header metadata. It may be called multiple times.
// When call multiple times, all the provided metadata will be merged.
// All the metadata will be sent out when one of the following happens:
//  - ServerStream.SendHeader() is called;
//  - The first response is sent out;
//  - An RPC status is sent out (error or success).
func (s *ServerStream) SetHeader(header metadata.MD) error {
	if s.headersWritten {
		return errors.Wrap(ErrIllegalHeaderWrite, 0)
	}
	if s.header == nil {
		s.header = header
	} else if header != nil {
		s.header = metadata.Join(s.header, header)
	}
	return nil
}

// SendHeader sends the header metadata.
// The provided md and headers set by SetHeader() will be sent.
// It fails if called multiple times.
func (s *ServerStream) SendHeader(header metadata.MD) error {
	err := s.SetHeader(header)
	if err != nil {
		return err
	}
	return s.writeHeaders()
}

// SetTrailer sets the trailer metadata which will be sent with the RPC status.
// When called more than once, all the provided metadata will be merged.
func (s *ServerStream) SetTrailer(trailer metadata.MD) {
	if s.trailer == nil {
		s.trailer = trailer
	} else if trailer != nil {
		s.trailer = metadata.Join(s.trailer, trailer)
	}
}

var maxResponseMessagePacketDataSize int

func init() {
	md, err := proto.Marshal(&webrtcpb.Response{
		Stream: &webrtcpb.Stream{
			Id: 1,
		},
		Type: &webrtcpb.Response_Message{
			Message: &webrtcpb.ResponseMessage{
				PacketMessage: &webrtcpb.PacketMessage{Eom: true},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	// max msg size - packet size - msg type size - proto padding (?)
	maxResponseMessagePacketDataSize = maxDataChannelSize - len(md) - 1
}

// SendMsg sends a message. On error, SendMsg aborts the stream and the
// error is returned directly.
//
// SendMsg blocks until:
//   - There is sufficient flow control to schedule m with the transport, or
//   - The stream is done, or
//   - The stream breaks.
//
// SendMsg does not wait until the message is received by the client. An
// untimely stream closure may result in lost messages.
//
// It is safe to have a goroutine calling SendMsg and another goroutine
// calling RecvMsg on the same stream at the same time, but it is not safe
// to call SendMsg on the same stream in different goroutines.
func (s *ServerStream) SendMsg(m interface{}) (err error) {
	defer func() {
		if err != nil {
			err = multierr.Combine(err, s.closeWithSendError(err))
		}
	}()

	if err := s.writeHeaders(); err != nil {
		return err
	}
	data, err := proto.Marshal(m.(proto.Message))
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return s.ch.writeMessage(s.stream, &webrtcpb.ResponseMessage{
			PacketMessage: &webrtcpb.PacketMessage{
				Eom: true,
			},
		})
	}

	for len(data) != 0 {
		amountToSend := maxResponseMessagePacketDataSize
		if len(data) < amountToSend {
			amountToSend = len(data)
		}
		packet := &webrtcpb.PacketMessage{
			Data: data[:amountToSend],
		}
		data = data[amountToSend:]
		if len(data) == 0 {
			packet.Eom = true
		}
		if err := s.ch.writeMessage(s.stream, &webrtcpb.ResponseMessage{
			PacketMessage: packet,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ServerStream) onRequest(request *webrtcpb.Request) {
	switch r := request.Type.(type) {
	case *webrtcpb.Request_Headers:
		if s.headersReceived {
			if err := s.closeWithSendError(status.Error(codes.InvalidArgument, "headers already received")); err != nil {
				s.logger.Errorw("error closing", "error", err)
			}
			return
		}
		s.processHeaders(r.Headers)
	case *webrtcpb.Request_Message:
		if !s.headersReceived {
			if err := s.closeWithSendError(status.Error(codes.InvalidArgument, "headers not yet received")); err != nil {
				s.logger.Errorw("error closing", "error", err)
			}
			return
		}
		s.processMessage(r.Message)
	default:
		if err := s.closeWithSendError(status.Error(codes.InvalidArgument, fmt.Sprintf("unknown request type %T", r))); err != nil {
			s.logger.Errorw("error closing", "error", err)
		}
	}
}

func (s *ServerStream) processHeaders(headers *webrtcpb.RequestHeaders) {
	s.logger = s.logger.With("method", headers.Method)

	handlerFunc, ok := s.ch.server.handler(headers.Method)
	if !ok {
		if err := s.closeWithSendError(status.Error(codes.Unimplemented, codes.Unimplemented.String())); err != nil {
			s.logger.Errorw("error closing", "error", err)
		}
		return
	}

	// take a ticket
	select {
	case <-s.ch.server.ctx.Done():
		if err := s.closeWithSendError(status.FromContextError(s.ch.server.ctx.Err()).Err()); err != nil {
			s.logger.Errorw("error closing", "error", err)
		}
		return
	case s.ch.server.callTickets <- struct{}{}:
	default:
		if err := s.closeWithSendError(status.Error(codes.ResourceExhausted, "too many in-flight requests")); err != nil {
			s.logger.Errorw("error closing", "error", err)
		}
		return
	}

	s.headersReceived = true
	s.ch.server.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer func() {
			<-s.ch.server.callTickets // return a ticket
		}()
		defer s.ch.server.activeBackgroundWorkers.Done()
		if err := handlerFunc(s); err != nil {
			if utils.FilterOutError(err, context.Canceled) != nil || errors.Is(err, io.ErrClosedPipe) {
				return
			}
			s.logger.Errorw("error calling handler", "error", err)
		}
	})
}

func (s *ServerStream) processMessage(msg *webrtcpb.RequestMessage) {
	if s.recvClosed {
		s.logger.Error("message received after EOS")
		return
	}
	if msg.HasMessage {
		data, eop := s.baseStream.processMessage(msg.PacketMessage)
		if !eop {
			return
		}
		s.msgCh <- data
	}
	if msg.Eos {
		s.CloseRecv()
	}
}

func (s *ServerStream) closeWithSendError(err error) error {
	defer s.close()
	if err != nil && (errors.Is(err, io.ErrClosedPipe)) {
		return nil
	}
	chClosed, chClosedReason := s.ch.Closed()
	if s.Closed() || chClosed {
		if err != nil && errors.Is(chClosedReason, errDataChannelClosed) && errors.Is(err, context.Canceled) {
			return nil
		}
		if err == nil {
			return errors.New("close called multiple times")
		}
		return errors.Wrap(fmt.Errorf("close called multiple times with error: %w", err), 0)
	}
	if err := s.writeHeaders(); err != nil {
		return err
	}
	var respStatus *status.Status
	if err == nil {
		respStatus = ErrorToStatus(s.ctx.Err())
	} else {
		respStatus = ErrorToStatus(err)
	}
	return s.ch.writeTrailers(s.stream, &webrtcpb.ResponseTrailers{
		Status:   respStatus.Proto(),
		Metadata: metadataToProto(s.trailer),
	})
}

func (s *ServerStream) writeHeaders() error {
	if !s.headersWritten {
		s.headersWritten = true
		return s.ch.writeHeaders(s.stream, &webrtcpb.ResponseHeaders{
			Metadata: metadataToProto(s.header),
		})
	}
	return nil
}

// ErrorToStatus converts an error to a gRPC status. A nil
// error becomes a successful status.
func ErrorToStatus(err error) *status.Status {
	respStatus := status.FromContextError(err)
	if respStatus.Code() == codes.Unknown {
		respStatus = status.Convert(err)
		if respStatus == nil {
			respStatus = status.New(codes.OK, "")
		}
	}
	return respStatus
}
