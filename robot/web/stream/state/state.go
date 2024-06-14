// Package state controls the source of the RTP packets being written to the stream's subscribers
// and ensures there is only one active at a time while there are peer connections to receive RTP packets.
package state

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

var (
	// ErrRTPPassthroughNotSupported indicates that rtp_passthrough is not supported by the stream's camera.
	ErrRTPPassthroughNotSupported = errors.New("RTP Passthrough Not Supported")
	// ErrClosed indicates that the StreamState is already closed.
	ErrClosed = errors.New("StreamState already closed")
	// ErrUninitialized indicates that Init() has not been called on StreamState prior to Increment or Decrement being called.
	ErrUninitialized = errors.New("uniniialized")
)

// StreamState controls the source of the RTP packets being written to the stream's subscribers
// and ensures there is only one active at a time while there are subsribers.
type StreamState struct {
	// Stream is the StreamState's stream
	Stream      gostream.Stream
	robot       robot.Robot
	closedCtx   context.Context
	closedFn    context.CancelFunc
	wg          sync.WaitGroup
	logger      logging.Logger
	initialized atomic.Bool

	msgChan  chan msg
	tickChan chan struct{}

	activeClients int
	streamSource  streamSource
	// streamSourceSub is only non nil if streamSource == streamSourcePassthrough
	streamSourceSub rtppassthrough.Subscription
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
	return &StreamState{
		Stream:    stream,
		closedCtx: ctx,
		closedFn:  cancel,
		robot:     r,
		msgChan:   make(chan msg),
		tickChan:  make(chan struct{}),
		logger:    logger,
	}
}

// Init initializes the StreamState
// Init must be called before any other methods.
func (ss *StreamState) Init() {
	ss.wg.Add(1)
	utils.ManagedGo(ss.initEventHandler, ss.wg.Done)
	ss.wg.Add(1)
	utils.ManagedGo(ss.initStreamSourceMonitor, ss.wg.Done)
	ss.initialized.Store(true)
}

// Increment increments the peer connections subscribed to the stream.
func (ss *StreamState) Increment() error {
	if !ss.initialized.Load() {
		return ErrUninitialized
	}
	if err := ss.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return ss.send(msgTypeIncrement)
}

// Decrement decrements the peer connections subscribed to the stream.
func (ss *StreamState) Decrement() error {
	if !ss.initialized.Load() {
		return ErrUninitialized
	}
	if err := ss.closedCtx.Err(); err != nil {
		return multierr.Combine(ErrClosed, err)
	}
	return ss.send(msgTypeDecrement)
}

// Tick looks at the activeClients and starts / stops camera streams.
func (ss *StreamState) Tick() {
	if err := ss.closedCtx.Err(); err != nil {
		return
	}
	utils.UncheckedError(ss.send(msgTypeTick))
}

// Close closes the StreamState.
func (ss *StreamState) Close() error {
	ss.closedFn()
	ss.wg.Wait()
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
	msgTypeTick
)

func (mt msgType) String() string {
	switch mt {
	case msgTypeIncrement:
		return "Increment"
	case msgTypeDecrement:
		return "Decrement"
	case msgTypeTick:
		return "Tick"
	case msgTypeUnknown:
		fallthrough
	default:
		return "Unknown"
	}
}

type msg struct {
	msgType  msgType
	respChan chan struct{}
}

// events (Inc Dec Tick).
func (ss *StreamState) initEventHandler() {
	ss.logger.Debug("StreamState initEventHandler booted")
	defer ss.logger.Debug("StreamState initEventHandler terminated")
	defer func() { ss.stopBasedOnSub() }()
	for {
		if ss.closedCtx.Err() != nil {
			return
		}

		select {
		case <-ss.closedCtx.Done():
			return
		case msg := <-ss.msgChan:
			ss.handleMsg(msg)
		}
	}
}

func (ss *StreamState) initStreamSourceMonitor() {
	timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second)
	for {
		select {
		case <-ss.closedCtx.Done():
			timeoutFn()
			return
		case <-timeoutCtx.Done():
			timeoutFn()
			ss.Tick()
			timeoutCtx, timeoutFn = context.WithTimeout(context.Background(), time.Second)
		case <-ss.tickChan:
			timeoutFn()
			ss.Tick()
		}
	}
}

func (ss *StreamState) monitorSubscription(sub rtppassthrough.Subscription) {
	if ss.streamSource == streamSourceGoStream {
		ss.logger.Debugf("monitorSubscription stopping gostream %s", ss.Stream.Name())
		// if we were streaming using gostream, stop streaming using gostream as we are now using passthrough
		ss.Stream.Stop()
	}
	ss.streamSourceSub = sub
	ss.streamSource = streamSourcePassthrough
	monitorSubFunc := func() {
		// if the stream state is shutting down, terminate
		if ss.closedCtx.Err() != nil {
			return
		}

		select {
		case <-ss.closedCtx.Done():
			return
		case <-sub.Terminated.Done():
			select {
			case ss.tickChan <- struct{}{}:
				ss.logger.Info("monitorSubscription sent to tickChan")
			default:
			}
			return
		}
	}

	ss.wg.Add(1)
	utils.ManagedGo(monitorSubFunc, ss.wg.Done)
}

func (ss *StreamState) stopBasedOnSub() {
	switch ss.streamSource {
	case streamSourceGoStream:
		ss.logger.Debugf("%s stopBasedOnSub stopping GoStream", ss.Stream.Name())
		ss.Stream.Stop()
		ss.streamSource = streamSourceUnknown
		return
	case streamSourcePassthrough:
		ss.logger.Debugf("%s stopBasedOnSub stopping passthrough", ss.Stream.Name())
		err := ss.unsubscribeH264Passthrough(ss.closedCtx, ss.streamSourceSub.ID)
		if err != nil {
			ss.logger.Error(err.Error())
			return
		}
		ss.streamSourceSub = rtppassthrough.NilSubscription
		ss.streamSource = streamSourceUnknown

	case streamSourceUnknown:
		fallthrough
	default:
	}
}

func (ss *StreamState) send(msgType msgType) error {
	if err := ss.closedCtx.Err(); err != nil {
		return err
	}
	msg := msg{
		msgType:  msgType,
		respChan: make(chan struct{}),
	}
	select {
	case ss.msgChan <- msg:
		select {
		case <-msg.respChan:
			return nil
		case <-ss.closedCtx.Done():
			return ss.closedCtx.Err()
		}
	case <-ss.closedCtx.Done():
		return ss.closedCtx.Err()
	}
}

func (ss *StreamState) handleMsg(msg msg) {
	switch msg.msgType {
	case msgTypeIncrement:
		ss.inc()
		select {
		case msg.respChan <- struct{}{}:
		case <-ss.closedCtx.Done():
			return
		}
	case msgTypeTick:
		ss.tick()
		select {
		case msg.respChan <- struct{}{}:
		case <-ss.closedCtx.Done():
			return
		}
	case msgTypeDecrement:
		ss.dec()
		select {
		case msg.respChan <- struct{}{}:
		case <-ss.closedCtx.Done():
			return
		}
	case msgTypeUnknown:
		fallthrough
	default:
		ss.logger.Error("Invalid StreamState msg type received: %s", msg.msgType)
	}
}

func (ss *StreamState) inc() {
	ss.logger.Debugf("increment %s START activeClients: %d", ss.Stream.Name(), ss.activeClients)
	defer func() { ss.logger.Debugf("increment %s END activeClients: %d", ss.Stream.Name(), ss.activeClients) }()
	ss.activeClients++
	if ss.activeClients == 1 {
		select {
		case ss.tickChan <- struct{}{}:
			ss.logger.Info("inc sent to tickChan")
		default:
		}
	}
	// 	if ss.streamSource != streamSourceUnknown {
	// 		return fmt.Errorf("unexpected stream %s source %s", ss.Stream.Name(), ss.streamSource)
	// 	}
	// 	this is the first subscription, attempt passthrough
	// 	ss.logger.CDebugw(ctx, "attempting to subscribe to rtp_passthrough", "name", ss.Stream.Name())
	// 	err := ss.streamH264Passthrough(ctx)
	// 	if err != nil {
	// 		ss.logger.CDebugw(ctx, "rtp_passthrough not possible, falling back to GoStream", "err", err.Error(), "name", ss.Stream.Name())
	// 		// if passthrough failed, fall back to gostream based approach
	// 		ss.Stream.Start()
	// 		ss.streamSource = streamSourceGoStream
	// 	}
	// 	ss.activeClients++
	// 	return nil
	// }

	// switch ss.streamSource {
	// case streamSourcePassthrough:
	// 	ss.logger.Debugw("continuing using rtp_passthrough", "name", ss.Stream.Name())
	// 	// noop as we are already subscribed
	// case streamSourceGoStream:
	// 	ss.logger.Debugw("currently using gostream, trying upgrade to rtp_passthrough", "name", ss.Stream.Name())
	// 	// attempt to cut over to passthrough
	// 	err := ss.streamH264Passthrough(ctx)
	// 	if err != nil {
	// 		ss.logger.Debugw("rtp_passthrough not possible, continuing with gostream", "err", err.Error(), "name", ss.Stream.Name())
	// 	}
	// case streamSourceUnknown:
	// 	fallthrough
	// default:
	// 	err := fmt.Errorf("%s streamSource in unexpected state %s", ss.Stream.Name(), ss.streamSource)
	// 	ss.logger.Error(err.Error())
	// 	return err
	// }
	// ss.activeClients++
}

func (ss *StreamState) dec() {
	ss.logger.Debugf("decrement START %s activeClients: %d", ss.Stream.Name(), ss.activeClients)
	defer func() { ss.logger.Debugf("decrement END %s activeClients: %d", ss.Stream.Name(), ss.activeClients) }()
	if ss.activeClients == 1 {
		select {
		case ss.tickChan <- struct{}{}:
			ss.logger.Info("dec sent to tickChan")
		default:
		}
	}

	ss.activeClients--
	if ss.activeClients < 0 {
		ss.logger.Errorf("ss.activeClients is less than 0 %d", ss.activeClients)
		ss.activeClients = 0
	}
}

func (ss *StreamState) tick() {
	ss.logger.Debugf("tick %s START activeClients: %d", ss.Stream.Name(), ss.activeClients)
	defer func() { ss.logger.Debugf("tick %s END activeClients: %d", ss.Stream.Name(), ss.activeClients) }()
	// _, err := ss.Camera()
	switch {
	// case err != nil && ss.activeClients > 0:
	// 	ss.logger.Warn("camera no longer exists in resource graph, stopping subscriptions and setting active clients to 0")
	// 	// stop stream if the camera no longer exists
	// 	// noop if there is no stream source
	// 	ss.stopBasedOnSub()
	// 	// nil out subsription & stream source even if stopBasedOnSub didn't
	// 	ss.streamSourceSub = rtppassthrough.NilSubscription
	// 	ss.streamSource = streamSourceUnknown
	// 	ss.activeClients = 0
	case ss.activeClients < 0:
		ss.logger.Fatal("tick: activeClients is less than 0")
	case ss.activeClients == 0:
		// stop stream if there are no active clients
		// noop if there is no stream source
		ss.stopBasedOnSub()
	case ss.streamSource == streamSourcePassthrough && ss.streamSourceSub.Terminated.Err() != nil:
		// restart stream if there we were using passthrough but the sub is termianted
		ss.logger.Debugw("tick: previous subscription termianted attempting to subscribe to rtp_passthrough", "name", ss.Stream.Name())
		err := ss.streamH264Passthrough()
		if err != nil {
			ss.logger.Debugw("tick: rtp_passthrough not possible, falling back to GoStream", "err", err.Error(), "name", ss.Stream.Name())
			// if passthrough failed, fall back to gostream based approach
			ss.Stream.Start()
			ss.streamSource = streamSourceGoStream
		}
	case ss.streamSource == streamSourcePassthrough:
		// no op if we are using passthrough & are healthy
		ss.logger.Debug("tick: still using stream source, passing")
	case ss.streamSource == streamSourceGoStream:
		// try to upgrade to passthrough if we are using gostream
		ss.logger.Debugw("tick: currently using gostream, trying upgrade to rtp_passthrough", "name", ss.Stream.Name())
		// attempt to cut over to passthrough
		err := ss.streamH264Passthrough()
		if err != nil {
			ss.logger.Debugw("tick: rtp_passthrough not possible, continuing with gostream", "err", err.Error(), "name", ss.Stream.Name())
		}
	case ss.streamSource == streamSourceUnknown:
		// this is the first subscription, attempt passthrough
		ss.logger.Debugw("tick: attempting to subscribe to rtp_passthrough", "name", ss.Stream.Name())
		err := ss.streamH264Passthrough()
		if err != nil {
			ss.logger.Debugw("tick: rtp_passthrough not possible, falling back to GoStream", "err", err.Error(), "name", ss.Stream.Name())
			// if passthrough failed, fall back to gostream based approach
			ss.Stream.Start()
			ss.streamSource = streamSourceGoStream
		}
	}
}

func (ss *StreamState) Camera() (camera.Camera, error) {
	// Stream names are slightly modified versions of the resource short name
	shortName := resource.SDPTrackNameToShortName(ss.Stream.Name())
	cam, err := camera.FromRobot(ss.robot, shortName)
	if err != nil {
		return nil, err
	}
	return cam, nil
}

func (ss *StreamState) streamH264Passthrough() error {
	cam, err := ss.Camera()
	if err != nil {
		return err
	}

	rtpPassthroughSource, ok := cam.(rtppassthrough.Source)
	if !ok {
		err := fmt.Errorf("expected %s to implement rtppassthrough.Source", ss.Stream.Name())
		return errors.Wrap(ErrRTPPassthroughNotSupported, err.Error())
	}

	var count atomic.Uint64
	cb := func(pkts []*rtp.Packet) {
		for _, pkt := range pkts {
			if count.Load()%100 == 0 {
				golog.Global().Infof("calling WriteRTP %s", ss.Stream.Name())
			}
			count.Add(1)
			if err := ss.Stream.WriteRTP(pkt); err != nil {
				ss.logger.Debugw("stream.WriteRTP", "name", ss.Stream.Name(), "err", err.Error())
			}
		}
	}

	sub, err := rtpPassthroughSource.SubscribeRTP(ss.closedCtx, rtpBufferSize, cb)
	if err != nil {
		return errors.Wrap(ErrRTPPassthroughNotSupported, err.Error())
	}
	ss.logger.Warnw("Stream using experimental H264 passthrough", "name", ss.Stream.Name())
	ss.monitorSubscription(sub)

	return nil
}

func (ss *StreamState) unsubscribeH264Passthrough(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	cam, err := ss.Camera()
	if err != nil {
		return err
	}

	rtpPassthroughSource, ok := cam.(rtppassthrough.Source)
	if !ok {
		err := fmt.Errorf("expected %s to implement rtppassthrough.Source", ss.Stream.Name())
		return errors.Wrap(ErrRTPPassthroughNotSupported, err.Error())
	}

	if err := rtpPassthroughSource.Unsubscribe(ctx, id); err != nil {
		return err
	}

	return nil
}
