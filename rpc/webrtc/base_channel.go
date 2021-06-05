package rpcwebrtc

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	"google.golang.org/protobuf/proto"
)

// MaxMessageSize is the maximum size a gRPC message can be.
var MaxMessageSize = 1 << 24

type baseChannel struct {
	mu           sync.Mutex
	peerConn     *webrtc.PeerConnection
	dataChannel  *webrtc.DataChannel
	ctx          context.Context
	cancel       func()
	ready        chan struct{}
	closed       bool
	closedReason error
	logger       golog.Logger
}

func newBaseChannel(
	ctx context.Context,
	peerConn *webrtc.PeerConnection,
	dataChannel *webrtc.DataChannel,
	onPeerDone func(),
	logger golog.Logger,
) *baseChannel {
	ctx, cancel := context.WithCancel(ctx)
	ch := &baseChannel{
		peerConn:    peerConn,
		dataChannel: dataChannel,
		ctx:         ctx,
		cancel:      cancel,
		ready:       make(chan struct{}),
		logger:      logger.With("ch", dataChannel.ID()),
	}
	dataChannel.OnOpen(ch.onChannelOpen)
	dataChannel.OnClose(ch.onChannelClose)
	dataChannel.OnError(ch.onChannelError)

	var connID string
	var peerDoneOnce bool
	peerConn.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		ch.mu.Lock()
		defer ch.mu.Unlock()
		if ch.closed {
			if !peerDoneOnce && onPeerDone != nil {
				peerDoneOnce = true
				onPeerDone()
			}
			return
		}

		switch connectionState {
		case webrtc.ICEConnectionStateDisconnected,
			webrtc.ICEConnectionStateFailed,
			webrtc.ICEConnectionStateClosed:
			logger.Debugw("connection state changed",
				"conn_id", connID,
				"conn_state", connectionState.String(),
			)
			if !peerDoneOnce && onPeerDone != nil {
				peerDoneOnce = true
				onPeerDone()
			}
		default:
			connInfo := getPeerConnectionStats(peerConn)
			connID = connInfo.ID
			logger.Debugw("connection state changed",
				"conn_id", connID,
				"conn_state", connectionState.String(),
				"conn_remote_candidates", connInfo.RemoteCandidates,
			)
		}
	})

	return ch
}

func (ch *baseChannel) closeWithReason(err error) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.closed {
		return nil
	}
	ch.closed = true
	ch.closedReason = err
	ch.cancel()
	return ch.peerConn.Close()
}

func (ch *baseChannel) Close() error {
	return ch.closeWithReason(nil)
}

func (ch *baseChannel) Closed() (bool, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return ch.closed, ch.closedReason
}

func (ch *baseChannel) Ready() <-chan struct{} {
	return ch.ready
}

func (ch *baseChannel) onChannelOpen() {
	close(ch.ready)
}

var errDataChannelClosed = errors.New("data channel closed")

func (ch *baseChannel) onChannelClose() {
	if err := ch.closeWithReason(errDataChannelClosed); err != nil {
		ch.logger.Errorw("error closing channel", "error", err)
	}
}

func (ch *baseChannel) onChannelError(err error) {
	ch.logger.Errorw("channel error", "error", err)
	if err := ch.closeWithReason(err); err != nil {
		ch.logger.Errorw("error closing channel", "error", err)
	}
}

const maxDataChannelSize = 16384

func (ch *baseChannel) write(msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	if err := ch.dataChannel.Send(data); err != nil {
		if strings.Contains(err.Error(), "sending payload data in non-established state") {
			return io.ErrClosedPipe
		}
		return err
	}
	return nil
}
