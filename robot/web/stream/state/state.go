// Package state controls the source of the RTP packets being written to the stream's subscribers
// and ensures there is only one active at a time while there are peer connections to receive RTP packets.
package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	camerautils "go.viam.com/rdk/robot/web/stream/camera"
)

// ErrClosed indicates that the StreamState is already closed.
var ErrClosed = errors.New("StreamState already closed")

// StreamState controls the source of the RTP packets being written to the stream's subscribers
// and ensures there is only one active at a time while there are subsribers.
type StreamState struct {
	// Stream is the StreamState's stream
	Stream    gostream.Stream
	robot     robot.Robot
	closedCtx context.Context
	closedFn  context.CancelFunc
	wg        sync.WaitGroup
	logger    logging.Logger

	msgChan  chan msg
	tickChan chan struct{}

	activeClients int
	streamSource  streamSource
	// streamSourceSub is only non nil if streamSource == streamSourcePassthrough
	streamSourceSub rtppassthrough.Subscription
	// isResized indicates whether the stream has been resized by the stream server.
	// When set to true, it signals that the passthrough stream should not be restarted.
	isResized bool
}

// New returns a new *StreamState.
// rtpPassthroughSource is allowed to be nil
// if the camere does not implement rtppassthrough.Source.
func New(
	stream gostream.Stream,
	r robot.Robot,
	logger logging.Logger,
) *StreamState {
	ctx, cancel := context.WithCancel(context.Background())
	ret := &StreamState{
		Stream:    stream,
		closedCtx: ctx,
		closedFn:  cancel,
		robot:     r,
		msgChan:   make(chan msg),
		tickChan:  make(chan struct{}),
		isResized: false,
		logger:    logger,
	}

	ret.wg.Add(1)
	// The event handler for a stream input manages the following events:
	// - There's a new subscriber (bump ref counter)
	// - A subscriber has left (dec ref counter)
	// - Camera is remevod (dec for all subscribers)
	// - Peer connection is closed (dec for all subscribers)
	utils.ManagedGo(ret.sourceEventHandler, ret.wg.Done)
	return ret
}

// Increment increments the peer connections subscribed to the stream.
func (state *StreamState) Increment() error {
	if err := state.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return state.send(msgTypeIncrement)
}

// Decrement decrements the peer connections subscribed to the stream.
func (state *StreamState) Decrement() error {
	if err := state.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return state.send(msgTypeDecrement)
}

// Resize notifies that the gostream source has been resized. This will stop and prevent
// the use of the passthrough stream if it is supported.
func (state *StreamState) Resize() error {
	if err := state.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return state.send(msgTypeResize)
}

// Reset notifies that the gostream source has been reset to the original resolution.
// This will restart the passthrough stream if it is supported.
func (state *StreamState) Reset() error {
	if err := state.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return state.send(msgTypeReset)
}

// Close closes the StreamState.
func (state *StreamState) Close() error {
	state.logger.Info("Closing streamState")
	state.closedFn()
	state.wg.Wait()
	return nil
}

// Internals

const rtpBufferSize int = 512

type streamSource uint8

const (
	streamSourceUnknown streamSource = iota
	streamSourceGoStream
	streamSourcePassthrough
)

func (s streamSource) String() string {
	switch s {
	case streamSourceGoStream:
		return "GoStream"
	case streamSourcePassthrough:
		return "RTP Passthrough"
	case streamSourceUnknown:
		fallthrough
	default:
		return "Unknown"
	}
}

type msgType uint8

const (
	msgTypeUnknown msgType = iota
	msgTypeIncrement
	msgTypeDecrement
	msgTypeResize
	msgTypeReset
)

func (mt msgType) String() string {
	switch mt {
	case msgTypeIncrement:
		return "Increment"
	case msgTypeDecrement:
		return "Decrement"
	case msgTypeResize:
		return "Resize"
	case msgTypeReset:
		return "Reset"
	case msgTypeUnknown:
		fallthrough
	default:
		return "Unknown"
	}
}

type msg struct {
	msgType msgType
}

// events (Inc Dec Tick).
func (state *StreamState) sourceEventHandler() {
	state.logger.Debug("sourceEventHandler booted")
	defer func() {
		state.logger.Debug("sourceEventHandler terminating")
		state.stopInputStream()
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		// We wait for:
		// - A message to be discovered on the `msgChan` queue.
		// - The `tick` timer to be fired
		// - The server/stream (i.e: camera) to be shutdown.
		var msg msg
		select {
		case <-state.closedCtx.Done():
			return
		case msg = <-state.msgChan:
		case <-ticker.C:
			state.tick()
			continue
		}

		switch msg.msgType {
		case msgTypeIncrement:
			state.activeClients++
			state.logger.Debugw("activeClients incremented", "activeClientCnt", state.activeClients)
			if state.activeClients == 1 {
				state.tick()
			}
		case msgTypeDecrement:
			state.activeClients--
			state.logger.Debugw("activeClients decremented", "activeClientCnt", state.activeClients)
			if state.activeClients == 0 {
				state.tick()
			}
		case msgTypeResize:
			state.logger.Debug("resize event received")
			state.isResized = true
			state.tick()
		case msgTypeReset:
			state.logger.Debug("reset event received")
			state.isResized = false
			state.tick()
		case msgTypeUnknown:
			fallthrough
		default:
			state.logger.Errorw("Invalid StreamState msg type received", "type", msg.msgType)
		}
	}
}

func (state *StreamState) monitorSubscription(terminatedCtx context.Context) {
	select {
	case <-state.closedCtx.Done():
		return
	case <-terminatedCtx.Done():
		select {
		case state.tickChan <- struct{}{}:
			state.logger.Info("monitorSubscription sent to tickChan")
		default:
		}
		return
	}
}

func (state *StreamState) stopInputStream() {
	switch state.streamSource {
	case streamSourceGoStream:
		state.logger.Debug("stopping gostream stream")
		defer state.logger.Debug("gostream stopped")
		state.Stream.Stop()
		state.streamSource = streamSourceUnknown
		return
	case streamSourcePassthrough:
		state.logger.Debug("stopping h264 passthrough stream")
		defer state.logger.Debug("h264 passthrough stream stopped")
		err := state.unsubscribeH264Passthrough(state.closedCtx, state.streamSourceSub.ID)
		if err != nil && errors.Is(err, camera.ErrUnknownSubscriptionID) {
			state.logger.Warnw("Error calling unsubscribe", "err", err)
			return
		}
		state.streamSourceSub = rtppassthrough.NilSubscription
		state.streamSource = streamSourceUnknown
	case streamSourceUnknown:
	default:
	}
}

func (state *StreamState) send(msgType msgType) error {
	select {
	case state.msgChan <- msg{msgType: msgType}:
		return nil
	case <-state.closedCtx.Done():
		return state.closedCtx.Err()
	}
}

func (state *StreamState) tick() {
	switch {
	case state.activeClients < 0:
		state.logger.Error("activeClients is less than 0")
	case state.activeClients == 0:
		// stop stream if there are no active clients
		// noop if there is no stream source
		state.stopInputStream()
	// If streamSource is unknown and resized is true, we do not want to attempt passthrough.
	case state.streamSource == streamSourceUnknown && state.isResized:
		state.logger.Debug("in a resized state and stream source is unknown, defaulting to GoStream")
		state.Stream.Start()
		state.streamSource = streamSourceGoStream
	case state.streamSource == streamSourceUnknown: // && state.activeClients > 0
		// this is the first subscription, attempt passthrough
		state.logger.Info("attempting to subscribe to rtp_passthrough")
		err := state.streamH264Passthrough()
		if err != nil {
			state.logger.Warnw("tick: rtp_passthrough not possible, falling back to GoStream", "err", err)
			// if passthrough failed, fall back to gostream based approach
			state.Stream.Start()
			state.streamSource = streamSourceGoStream
		}
	// If we are currently using passthrough, and the stream state changes to resized
	// we need to stop the passthrough stream and restart it through gostream.
	case state.streamSource == streamSourcePassthrough && state.isResized:
		state.logger.Info("stream resized, stopping passthrough stream")
		state.stopInputStream()
		state.Stream.Start()
		state.streamSource = streamSourceGoStream
	case state.streamSource == streamSourcePassthrough && state.streamSourceSub.Terminated.Err() != nil:
		// restart stream if there we were using passthrough but the sub is terminated
		state.logger.Info("previous subscription terminated attempting to subscribe to rtp_passthrough")

		err := state.streamH264Passthrough()
		if err != nil {
			state.logger.Warn("rtp_passthrough not possible, falling back to GoStream", "err", err)
			// if passthrough failed, fall back to gostream based approach
			state.Stream.Start()
			state.streamSource = streamSourceGoStream
		}
	case state.streamSource == streamSourcePassthrough:
		// no op if we are using passthrough & are healthy
		state.logger.Debug("still healthy and using h264 passthrough")
	case state.streamSource == streamSourceGoStream && !state.isResized:
		// Try to upgrade to passthrough if we are using gostream. We leave logs these as debugs as
		// we expect some components to not implement rtp passthrough.
		state.logger.Debugw("currently using gostream, trying upgrade to rtp_passthrough")
		// attempt to cut over to passthrough
		err := state.streamH264Passthrough()
		if err != nil {
			state.logger.Debugw("rtp_passthrough upgrade failed, continuing with gostream", "err", err)
		}
	}
}

func (state *StreamState) streamH264Passthrough() error {
	cam, err := camerautils.Camera(state.robot, state.Stream)
	if err != nil {
		return err
	}

	// Get the camera and see if it implements the rtp passthrough API of SubscribeRTP + Unsubscribe
	rtpPassthroughSource, ok := cam.(rtppassthrough.Source)
	if !ok {
		return errors.New("stream does not support RTP passthrough")
	}

	var count atomic.Uint64

	// We might be already sending video via gostream. In this case we:
	// - First try and create an RTP passthrough subscription
	// - If not successful, continue with gostream.
	// - Otherwise if successful, stop gostream.
	// - Once we're sure gostream is stopped, we close the `releasePackets` channel
	//
	// This ensures we only start sending passthrough packets after gostream has stopped sending
	// video packets.
	releasePackets := make(chan struct{})

	cb := func(pkts []*rtp.Packet) {
		<-releasePackets
		for _, pkt := range pkts {
			// Also, look at unsubscribe error logs. Definitely a bug. Probably benign.
			if count.Add(1)%10000 == 0 {
				state.logger.Debugw("WriteRTP called. Sampling 1/10000",
					"count", count.Load(), "seqNumber", pkt.Header.SequenceNumber, "ts", pkt.Header.Timestamp)
			}
			if err := state.Stream.WriteRTP(pkt); err != nil {
				state.logger.Debugw("stream.WriteRTP", "name", state.Stream.Name(), "err", err.Error())
			}
		}
	}

	sub, err := rtpPassthroughSource.SubscribeRTP(state.closedCtx, rtpBufferSize, cb)
	if err != nil {
		return fmt.Errorf("SubscribeRTP failed: %w", err)
	}
	state.logger.Warnw("Stream using experimental H264 passthrough", "name", state.Stream.Name())

	if state.streamSource == streamSourceGoStream {
		state.logger.Debugf("monitorSubscription stopping gostream %s", state.Stream.Name())
		// We've succeeded creating a passthrough stream. If we were streaming using gostream, stop it.
		state.Stream.Stop()
	}
	close(releasePackets)
	state.streamSourceSub = sub
	state.streamSource = streamSourcePassthrough

	state.wg.Add(1)
	utils.ManagedGo(func() {
		state.monitorSubscription(sub.Terminated)
	}, state.wg.Done)

	return nil
}

func (state *StreamState) unsubscribeH264Passthrough(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	cam, err := camerautils.Camera(state.robot, state.Stream)
	if err != nil {
		return err
	}

	rtpPassthroughSource, ok := cam.(rtppassthrough.Source)
	if !ok {
		return fmt.Errorf("subscription resource does not implement rtpPassthroughSource. CamType: %T", rtpPassthroughSource)
	}

	if err := rtpPassthroughSource.Unsubscribe(ctx, id); err != nil {
		return err
	}

	return nil
}
