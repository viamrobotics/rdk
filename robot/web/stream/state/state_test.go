package state_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pion/rtp"
	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/web/stream/state"
	"go.viam.com/rdk/testutils/inject"
)

type mockStream struct {
	name         string
	t            *testing.T
	startFunc    func()
	stopFunc     func()
	writeRTPFunc func(*rtp.Packet) error
}

func (mS *mockStream) Name() string {
	return mS.name
}

func (mS *mockStream) Start() {
	mS.startFunc()
}

func (mS *mockStream) Stop() {
	mS.stopFunc()
}

func (mS *mockStream) WriteRTP(pkt *rtp.Packet) error {
	return mS.writeRTPFunc(pkt)
}

// BEGIN Not tested gostream functions.
func (mS *mockStream) StreamingReady() (<-chan struct{}, context.Context) {
	mS.t.Log("unimplemented")
	mS.t.FailNow()
	return nil, context.Background()
}

func (mS *mockStream) InputVideoFrames(props prop.Video) (chan<- gostream.MediaReleasePair[image.Image], error) {
	mS.t.Log("unimplemented")
	mS.t.FailNow()
	return nil, errors.New("unimplemented")
}

func (mS *mockStream) InputAudioChunks(props prop.Audio) (chan<- gostream.MediaReleasePair[wave.Audio], error) {
	mS.t.Log("unimplemented")
	mS.t.FailNow()
	return make(chan gostream.MediaReleasePair[wave.Audio]), nil
}

func (mS *mockStream) VideoTrackLocal() (webrtc.TrackLocal, bool) {
	mS.t.Log("unimplemented")
	mS.t.FailNow()
	return nil, false
}

func (mS *mockStream) AudioTrackLocal() (webrtc.TrackLocal, bool) {
	mS.t.Log("unimplemented")
	mS.t.FailNow()
	return nil, false
}

type mockRTPPassthroughSource struct {
	subscribeRTPFunc func(
		ctx context.Context,
		bufferSize int,
		packetsCB rtppassthrough.PacketCallback,
	) (rtppassthrough.Subscription, error)
	unsubscribeFunc func(
		ctx context.Context,
		id rtppassthrough.SubscriptionID,
	) error
}

func (s *mockRTPPassthroughSource) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	return s.subscribeRTPFunc(ctx, bufferSize, packetsCB)
}

func (s *mockRTPPassthroughSource) Unsubscribe(
	ctx context.Context,
	id rtppassthrough.SubscriptionID,
) error {
	return s.unsubscribeFunc(ctx, id)
}

var camName = "my-cam"

// END Not tested gostream functions.
func mockRobot(s rtppassthrough.Source) robot.Robot {
	robot := &inject.Robot{}
	robot.MockResourcesFromMap(map[resource.Name]resource.Resource{
		camera.Named(camName): &inject.Camera{RTPPassthroughSource: s},
	})
	return robot
}

func TestStreamState(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("Stream returns the provided stream", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		test.That(t, s.Stream, test.ShouldEqual, streamMock)
	})

	t.Run("close succeeds if no methods have been called", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		test.That(t, s.Close(), test.ShouldBeNil)
	})

	t.Run("Increment() returns an error if Init() is not called first", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		test.That(t, s.Increment(ctx), test.ShouldBeError, state.ErrUninitialized)
	})

	t.Run("Decrement() returns an error if Init() is not called first", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		test.That(t, s.Decrement(ctx), test.ShouldBeError, state.ErrUninitialized)
	})

	t.Run("Increment() returns an error if called after Close()", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		s.Init()
		s.Close()
		test.That(t, s.Increment(ctx), test.ShouldWrap, state.ErrClosed)
	})

	t.Run("Decrement() returns an error if called after Close()", func(t *testing.T) {
		mockRTPPassthroughSource := &mockRTPPassthroughSource{}
		robot := mockRobot(mockRTPPassthroughSource)
		streamMock := &mockStream{name: camName, t: t}
		s := state.New(streamMock, robot, logger)
		s.Init()
		s.Close()
		test.That(t, s.Decrement(ctx), test.ShouldWrap, state.ErrClosed)
	})

	t.Run("when rtppassthrough.Souce is provided but SubscribeRTP always returns an error", func(t *testing.T) {
		var startCount atomic.Int64
		var stopCount atomic.Int64
		streamMock := &mockStream{
			name: camName,
			t:    t,
			startFunc: func() {
				startCount.Add(1)
			},
			stopFunc: func() {
				stopCount.Add(1)
			},
			writeRTPFunc: func(pkt *rtp.Packet) error {
				t.Log("should not happen")
				t.FailNow()
				return nil
			},
		}

		var subscribeRTPCount atomic.Int64

		subscribeRTPFunc := func(
			ctx context.Context,
			bufferSize int,
			packetsCB rtppassthrough.PacketCallback,
		) (rtppassthrough.Subscription, error) {
			subscribeRTPCount.Add(1)
			return rtppassthrough.NilSubscription, errors.New("unimplemented")
		}

		unsubscribeFunc := func(ctx context.Context, id rtppassthrough.SubscriptionID) error {
			t.Log("should not happen")
			t.FailNow()
			return errors.New("unimplemented")
		}

		mockRTPPassthroughSource := &mockRTPPassthroughSource{
			subscribeRTPFunc: subscribeRTPFunc,
			unsubscribeFunc:  unsubscribeFunc,
		}
		robot := mockRobot(mockRTPPassthroughSource)
		s := state.New(streamMock, robot, logger)
		defer func() { utils.UncheckedError(s.Close()) }()
		s.Init()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("the first Increment() calls SubscribeRTP and then calls Start() when an error is reurned")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("subsequent Increment() all calls call SubscribeRTP trying to determine " +
			"if they can upgrade but don't call any other gostream methods as SubscribeRTP returns an error")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("as long as the number of Decrement() calls is less than the number of Increment() calls, no gostream methods are called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		t.Log("then when the number of Increment() calls exceeds Decrement(), both SubscribeRTP & Start are called again")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		t.Log("calling Decrement() more times than Increment() has a floor of zero")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// multiple Decrement() calls when the count is already at zero doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// once the count is at zero , calling Increment() again calls SubscribeRTP and when it returns an error Start
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// set count back to zero
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Increment() with a cancelled context returns an error & does not call any gostream or rtppassthrough.Source methods")
		canceledCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		test.That(t, s.Increment(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		// make it so that non cancelled Decrement() would call stop to confirm that does not happen when context is cancelled
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 6)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Decrement() with a cancelled context returns an error & does not call any gostream methods")
		test.That(t, s.Decrement(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 6)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)
	})

	t.Run("when rtppassthrough.Souce is provided and SubscribeRTP doesn't return an error", func(t *testing.T) {
		writeRTPCalledCtx, writeRTPCalledFunc := context.WithCancel(ctx)
		streamMock := &mockStream{
			name: camName,
			t:    t,
			startFunc: func() {
				t.Logf("should not be called")
				t.FailNow()
			},
			stopFunc: func() {
				t.Logf("should not be called")
				t.FailNow()
			},
			writeRTPFunc: func(pkt *rtp.Packet) error {
				// Test that WriteRTP is eventually called when SubscribeRTP is called
				writeRTPCalledFunc()
				return nil
			},
		}

		var subscribeRTPCount atomic.Int64
		var unsubscribeCount atomic.Int64
		type subAndCancel struct {
			sub      rtppassthrough.Subscription
			cancelFn context.CancelFunc
			wg       *sync.WaitGroup
		}

		var subsAndCancelByIDMu sync.Mutex
		subsAndCancelByID := map[rtppassthrough.SubscriptionID]subAndCancel{}

		subscribeRTPFunc := func(
			ctx context.Context,
			bufferSize int,
			packetsCB rtppassthrough.PacketCallback,
		) (rtppassthrough.Subscription, error) {
			subsAndCancelByIDMu.Lock()
			defer subsAndCancelByIDMu.Unlock()
			defer subscribeRTPCount.Add(1)
			terminatedCtx, terminatedFn := context.WithCancel(context.Background())
			id := uuid.New()
			sub := rtppassthrough.Subscription{ID: id, Terminated: terminatedCtx}
			subsAndCancelByID[id] = subAndCancel{sub: sub, cancelFn: terminatedFn, wg: &sync.WaitGroup{}}
			subsAndCancelByID[id].wg.Add(1)
			utils.ManagedGo(func() {
				for terminatedCtx.Err() == nil {
					packetsCB([]*rtp.Packet{{}})
					time.Sleep(time.Millisecond * 50)
				}
			}, subsAndCancelByID[id].wg.Done)
			return sub, nil
		}

		unsubscribeFunc := func(ctx context.Context, id rtppassthrough.SubscriptionID) error {
			subsAndCancelByIDMu.Lock()
			defer subsAndCancelByIDMu.Unlock()
			defer unsubscribeCount.Add(1)
			subAndCancel, ok := subsAndCancelByID[id]
			if !ok {
				t.Logf("Unsubscribe called with unknown id: %s", id.String())
				t.FailNow()
			}
			subAndCancel.cancelFn()
			return nil
		}

		mockRTPPassthroughSource := &mockRTPPassthroughSource{
			subscribeRTPFunc: subscribeRTPFunc,
			unsubscribeFunc:  unsubscribeFunc,
		}
		robot := mockRobot(mockRTPPassthroughSource)
		s := state.New(streamMock, robot, logger)
		defer func() { utils.UncheckedError(s.Close()) }()

		s.Init()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, writeRTPCalledCtx.Err(), test.ShouldBeNil)

		t.Log("the first Increment() call calls SubscribeRTP()")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		// WriteRTP is called
		<-writeRTPCalledCtx.Done()

		t.Log("subsequent Increment() calls don't call any other rtppassthrough.Source methods")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)

		t.Log("as long as the number of Decrement() calls is less than the number " +
			"of Increment() calls, no rtppassthrough.Source methods are called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)

		t.Log("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)

		t.Log("then when the number of Increment() calls exceeds Decrement(), SubscribeRTP is called again")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)

		t.Log("calling Decrement() more times than Increment() has a floor of zero")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 2)

		// multiple Decrement() calls when the count is already at zero doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 2)

		// once the count is at zero , calling Increment() again calls start
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 2)

		// set count back to zero
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Increment() with a cancelled context returns an error & does not call any rtppassthrough.Source methods")
		canceledCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		test.That(t, s.Increment(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 3)

		// make it so that non cancelled Decrement() would call stop to confirm that does not happen when context is cancelled
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Decrement() with a cancelled context returns an error & does not call any rtppassthrough.Source methods")
		test.That(t, s.Decrement(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 3)

		t.Log("when the subscription terminates while there are still subscribers, SubscribeRTP is called again")
		var cancelledSubs int
		subsAndCancelByIDMu.Lock()
		for _, subAndCancel := range subsAndCancelByID {
			if subAndCancel.sub.Terminated.Err() == nil {
				subAndCancel.cancelFn()
				cancelledSubs++
			}
		}
		subsAndCancelByIDMu.Unlock()
		test.That(t, cancelledSubs, test.ShouldEqual, 1)

		// wait unil SubscribeRTP is called
		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Log("timed out waiting for a new sub to be created after an in progress one terminated unexpectedly")
				t.FailNow()
			}
			if subscribeRTPCount.Load() == 5 {
				break
			}
			time.Sleep(time.Millisecond * 50)
		}

		// cancel all mock subs
		subsAndCancelByIDMu.Lock()
		for _, subAndCancel := range subsAndCancelByID {
			subAndCancel.cancelFn()
			subAndCancel.wg.Wait()
		}
		subsAndCancelByIDMu.Unlock()
	})

	t.Run("when rtppassthrough.Souce is provided and sometimes returns an errors "+
		"(test rtp_passthrough/gostream upgrade/downgrade path)", func(t *testing.T) {
		var startCount atomic.Int64
		var stopCount atomic.Int64
		streamMock := &mockStream{
			name: camName,
			t:    t,
			startFunc: func() {
				startCount.Add(1)
			},
			stopFunc: func() {
				stopCount.Add(1)
			},
			writeRTPFunc: func(pkt *rtp.Packet) error {
				return nil
			},
		}

		var subscribeRTPCount atomic.Int64
		var unsubscribeCount atomic.Int64
		type subAndCancel struct {
			sub      rtppassthrough.Subscription
			cancelFn context.CancelFunc
		}
		var subsAndCancelByIDMu sync.Mutex
		subsAndCancelByID := map[rtppassthrough.SubscriptionID]subAndCancel{}

		var subscribeRTPReturnError atomic.Bool
		subscribeRTPFunc := func(
			ctx context.Context,
			bufferSize int,
			packetsCB rtppassthrough.PacketCallback,
		) (rtppassthrough.Subscription, error) {
			subsAndCancelByIDMu.Lock()
			defer subsAndCancelByIDMu.Unlock()
			defer subscribeRTPCount.Add(1)
			if subscribeRTPReturnError.Load() {
				return rtppassthrough.NilSubscription, errors.New("SubscribeRTP returned error")
			}
			terminatedCtx, terminatedFn := context.WithCancel(context.Background())
			id := uuid.New()
			sub := rtppassthrough.Subscription{ID: id, Terminated: terminatedCtx}
			subsAndCancelByID[id] = subAndCancel{sub: sub, cancelFn: terminatedFn}
			return sub, nil
		}

		unsubscribeFunc := func(ctx context.Context, id rtppassthrough.SubscriptionID) error {
			subsAndCancelByIDMu.Lock()
			defer subsAndCancelByIDMu.Unlock()
			defer unsubscribeCount.Add(1)
			subAndCancel, ok := subsAndCancelByID[id]
			if !ok {
				t.Logf("Unsubscribe called with unknown id: %s", id.String())
				t.FailNow()
			}
			subAndCancel.cancelFn()
			return nil
		}

		mockRTPPassthroughSource := &mockRTPPassthroughSource{
			subscribeRTPFunc: subscribeRTPFunc,
			unsubscribeFunc:  unsubscribeFunc,
		}
		robot := mockRobot(mockRTPPassthroughSource)
		s := state.New(streamMock, robot, logger)
		defer func() { utils.UncheckedError(s.Close()) }()

		// start with RTPPassthrough being supported
		s.Init()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("the first Increment() call calls SubscribeRTP() which returns a success")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("subsequent Increment() calls don't call any other rtppassthrough.Source or gostream methods")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("if the subscription terminates and SubscribeRTP() returns an error, starts gostream")
		subscribeRTPReturnError.Store(true)
		subsAndCancelByIDMu.Lock()
		test.That(t, len(subsAndCancelByID), test.ShouldEqual, 1)
		for _, s := range subsAndCancelByID {
			s.cancelFn()
		}
		subsAndCancelByIDMu.Unlock()

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Log("timed out waiting for gostream start to be called on stream which terminated unexpectedly")
				t.FailNow()
			}
			if subscribeRTPCount.Load() == 2 {
				break
			}
			time.Sleep(time.Millisecond * 50)
		}

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("when the number of Decrement() calls is less than the number of " +
			"Increment() calls no rtppassthrough.Source or gostream methods are called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("when the number of Decrement() calls is equal to the number of " +
			"Increment() calls stop is called (as gostream is the data source)")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		t.Log("then when the number of Increment() calls exceeds Decrement(), " +
			"SubscribeRTP is called again followed by Start if it returns an error")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		t.Log("calling Decrement() more times than Increment() has a floor of zero and calls Stop()")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// multiple Decrement() calls when the count is already at zero doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 3)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// once the count is at zero , calling Increment() again calls SubscribeRTP followed by Start
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 4)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		t.Log("if while gostream is being used Increment is called and SubscribeRTP succeeds, Stop is called")
		subscribeRTPReturnError.Store(false)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		// calling Decrement() fewer times than Increment() doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Decrement() more times than Increment() has a floor of zero and calls Unsubscribe()")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		// multiple Decrement() calls when the count is already at zero doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("if while rtp_passthrough is being used the the subscription " +
			"terminates & afterwards rtp_passthrough is no longer supported, Start is called")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 6)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		subscribeRTPReturnError.Store(true)

		subsAndCancelByIDMu.Lock()
		for _, s := range subsAndCancelByID {
			s.cancelFn()
		}
		subsAndCancelByIDMu.Unlock()
		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Log("timed out waiting for Start() to be called after an in progress sub terminated unexpectedly")
				t.FailNow()
			}
			if startCount.Load() == 4 {
				break
			}
			time.Sleep(time.Millisecond * 50)
		}
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 7)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		// Decrement() calls Stop() as gostream is the data source
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 7)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 1)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 4)
	})

	t.Run("when the camera does not implement rtppassthrough.Souce", func(t *testing.T) {
		var startCount atomic.Int64
		var stopCount atomic.Int64
		streamMock := &mockStream{
			name: "my-cam",
			t:    t,
			startFunc: func() {
				startCount.Add(1)
			},
			stopFunc: func() {
				stopCount.Add(1)
			},
			writeRTPFunc: func(pkt *rtp.Packet) error {
				t.Log("should not happen")
				t.FailNow()
				return nil
			},
		}
		robot := mockRobot(nil)
		s := state.New(streamMock, robot, logger)
		defer func() { utils.UncheckedError(s.Close()) }()
		s.Init()

		t.Log("the first Increment() -> Start()")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("subsequent Increment() all calls don't call any other gostream methods")
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("as long as the number of Decrement() calls is less than the number of Increment() calls, no gostream methods are called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		t.Log("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)
		test.That(t, s.Increment(ctx), test.ShouldBeNil)

		t.Log("then when the number of Increment() calls exceeds Decrement(), Start is called again")
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		t.Log("calling Decrement() more times than Increment() has a floor of zero")
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// multiple Decrement() calls when the count is already at zero doesn't call any methods
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 2)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// once the count is at zero , calling Increment() again calls start
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 2)

		// set count back to zero
		test.That(t, s.Decrement(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Increment() with a cancelled context returns an error & does not call any gostream methods")
		canceledCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		test.That(t, s.Increment(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		// make it so that non cancelled Decrement() would call stop to confirm that does not happen when context is cancelled
		test.That(t, s.Increment(ctx), test.ShouldBeNil)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		t.Log("calling Decrement() with a cancelled context returns an error & does not call any gostream methods")
		test.That(t, s.Decrement(canceledCtx), test.ShouldBeError, context.Canceled)
		test.That(t, startCount.Load(), test.ShouldEqual, 4)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)
	})
}
