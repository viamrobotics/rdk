package state_test

import (
	"context"
	"errors"
	"fmt"
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
	"go.viam.com/utils/testutils"

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

// Start refers to starting gostream.
func (mS *mockStream) Start() {
	mS.startFunc()
}

// Stop refers to stopping gostream.
func (mS *mockStream) Stop() {
	mS.stopFunc()
}

func (mS *mockStream) WriteRTP(pkt *rtp.Packet) error {
	return mS.writeRTPFunc(pkt)
}

// BEGIN Not tested gostream functions.
func (mS *mockStream) StreamingReady() (<-chan struct{}, context.Context) {
	test.That(mS.t, "should not be called", test.ShouldBeFalse)
	return nil, context.Background()
}

func (mS *mockStream) InputVideoFrames(props prop.Video) (chan<- gostream.MediaReleasePair[image.Image], error) {
	test.That(mS.t, "should not be called", test.ShouldBeFalse)
	return nil, errors.New("unimplemented")
}

func (mS *mockStream) InputAudioChunks(props prop.Audio) (chan<- gostream.MediaReleasePair[wave.Audio], error) {
	test.That(mS.t, "should not be called", test.ShouldBeFalse)
	return make(chan gostream.MediaReleasePair[wave.Audio]), nil
}

func (mS *mockStream) VideoTrackLocal() (webrtc.TrackLocal, bool) {
	test.That(mS.t, "should not be called", test.ShouldBeFalse)
	return nil, false
}

func (mS *mockStream) AudioTrackLocal() (webrtc.TrackLocal, bool) {
	test.That(mS.t, "should not be called", test.ShouldBeFalse)
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
	// we have to use sleep here as we are asserting the state doesn't change after a given period of time
	sleepDuration := time.Millisecond * 200

	t.Run("when rtppassthrough.Source is provided but SubscribeRTP always returns an error", func(t *testing.T) {
		// Define counters that are bumped every time the video stream is started or stopped in
		// "gostream mode".
		var startCount atomic.Int64
		var stopCount atomic.Int64

		// Because SubscribeRTP will always return an error, it should be a test failure if the mock
		// stream tries writing an RTP packet.
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
				test.That(t, "should not be called", test.ShouldBeFalse)
				return nil
			},
		}

		// Define an a function that matches the SubscribeRTP signature. It will fail on each
		// SubscribeRTP call. We define a counter to know how often the function has been called.
		var subscribeRTPCount atomic.Int64
		failingSubscribeRTPFunc := func(
			ctx context.Context,
			bufferSize int,
			packetsCB rtppassthrough.PacketCallback,
		) (rtppassthrough.Subscription, error) {
			subscribeRTPCount.Add(1)
			return rtppassthrough.NilSubscription, errors.New("unimplemented")
		}

		// Because SubscribeRTP will always fail, UnsubscribeRTP must not be called.
		unsubscribeFunc := func(ctx context.Context, id rtppassthrough.SubscriptionID) error {
			test.That(t, "should not be called", test.ShouldBeFalse)
			return errors.New("unimplemented")
		}

		mockRTPPassthroughSource := &mockRTPPassthroughSource{
			subscribeRTPFunc: failingSubscribeRTPFunc,
			unsubscribeFunc:  unsubscribeFunc,
		}
		robot := mockRobot(mockRTPPassthroughSource)
		s := state.New(streamMock, robot, logger)
		defer func() {
			utils.UncheckedError(s.Close())
		}()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		// Now we "increment" the number of `AddStream` calls. This means there's a (single)
		// consumer for this video stream.
		logger.Info("the first Increment() eventually calls SubscribeRTP and then calls Start() when an error is returned")
		test.That(t, s.Increment(), test.ShouldBeNil)

		// The post-loop invariant is that we've called SubscribeRTP (for passthrough) at least
		// once. And gostream has been started exactly once and never stopped. In this state, the
		// stream server will continue trying to upgrade to RTP passthrough. `Stop` will be only
		// called on the state object if the SubscribeRTP method was a success.
		var prevSubscribeRTPCount int64
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThan, 0)
			prevSubscribeRTPCount = subscribeRTPCount.Load()
			test.That(tb, startCount.Load(), test.ShouldEqual, 1)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 0)
		})

		logger.Info("as long as the number of Decrement() calls is less than the number of Increment() calls, no gostream methods are called")
		test.That(t, s.Increment(), test.ShouldBeNil)
		test.That(t, s.Increment(), test.ShouldBeNil)
		test.That(t, s.Decrement(), test.ShouldBeNil)
		test.That(t, s.Decrement(), test.ShouldBeNil)

		// Wait a bit and assert we've continue to try and upgrade to RTP passthrough. As measured
		// by SubscribeRTP calls.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThan, prevSubscribeRTPCount)
		})
		prevSubscribeRTPCount = subscribeRTPCount.Load()
		// Double check we did not stop/restart gostream.
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		// Decrement the stream state which should eventually stop gostream.
		logger.Info("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
		})

		// An upgrade attempt may or may not have been made concurrently with decrementing. Update
		// the running SubscribeRTP call counter.
		prevSubscribeRTPCount = subscribeRTPCount.Load()
		// We must not have tried restarting gostream.
		test.That(t, startCount.Load(), test.ShouldEqual, 1)

		logger.Info("then when the number of Increment() calls exceeds Decrement(), both SubscribeRTP & Start are eventually called again")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			// An increment always tries SubscribeRTP before gostream.
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThan, prevSubscribeRTPCount)
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
		})
		prevSubscribeRTPCount = subscribeRTPCount.Load()

		// Wait some more to observe SubscribeRTP continuing to be called. Then decrement the stream
		// state to observe gostream being `Stop`ed.
		time.Sleep(time.Second)
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThan, prevSubscribeRTPCount)
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})
		prevSubscribeRTPCount = subscribeRTPCount.Load()

		// Once the count is at zero , calling Increment() again calls SubscribeRTP. SubscribeRTP
		// will fail and `Start` gostream.
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThan, prevSubscribeRTPCount)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})

		// set count back to zero
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldBeGreaterThanOrEqualTo, prevSubscribeRTPCount)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})
	})

	t.Run("when rtppassthrough.Source is provided and SubscribeRTP doesn't return an error", func(t *testing.T) {
		writeRTPCalledCtx, writeRTPCalledFunc := context.WithCancel(ctx)
		streamMock := &mockStream{
			name: camName,
			t:    t,
			startFunc: func() {
				test.That(t, "should not be called", test.ShouldBeFalse)
			},
			stopFunc: func() {
				test.That(t, "should not be called", test.ShouldBeFalse)
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
				test.That(t, fmt.Sprintf("Unsubscribe called with unknown id: %s", id.String()), test.ShouldBeFalse)
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
		defer func() {
			utils.UncheckedError(s.Close())
		}()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, writeRTPCalledCtx.Err(), test.ShouldBeNil)

		logger.Info("the first Increment() eventually call calls SubscribeRTP()")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
		})
		// WriteRTP is called
		<-writeRTPCalledCtx.Done()

		logger.Info("subsequent Increment() calls don't call any other rtppassthrough.Source methods")
		test.That(t, s.Increment(), test.ShouldBeNil)
		test.That(t, s.Increment(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)

		logger.Info("as long as the number of Decrement() calls is less than the number " +
			"of Increment() calls, no rtppassthrough.Source methods are called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		test.That(t, s.Decrement(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)

		logger.Info("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
		})

		logger.Info("then when the number of Increment() calls exceeds Decrement(), SubscribeRTP is called again")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 2)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
		})

		test.That(t, s.Decrement(), test.ShouldBeNil)

		// once the count is at zero , calling Increment() again calls start
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 3)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 2)
		})

		// set count back to zero
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 3)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 3)
		})

		// make it so that non cancelled Decrement() would call stop to confirm that does not happen when context is cancelled
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 4)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 3)
		})

		logger.Info("when the subscription terminates while there are still subscribers, SubscribeRTP is called again")
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
				test.That(t,
					"timed out waiting for a new sub to be created after an in progress one terminated unexpectedly",
					test.ShouldBeFalse)
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

	t.Run("when rtppassthrough.Source is provided and sometimes returns an errors "+
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
				test.That(t,
					fmt.Sprintf("Unsubscribe called with unknown id: %s", id.String()),
					test.ShouldBeFalse)
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
		defer func() {
			utils.UncheckedError(s.Close())
		}()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		logger.Info("the first Increment() call calls SubscribeRTP() which returns a success")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			test.That(tb, startCount.Load(), test.ShouldEqual, 0)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 0)
		})

		logger.Info("subsequent Increment() calls don't call any other rtppassthrough.Source or gostream methods")
		test.That(t, s.Increment(), test.ShouldBeNil)
		test.That(t, s.Increment(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 1)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 0)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		logger.Info("if the subscription terminates and SubscribeRTP() returns an error, starts gostream")
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
				test.That(t,
					"timed out waiting for gostream start to be called on stream which terminated unexpectedly",
					test.ShouldBeFalse)
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

		logger.Info("when the number of Decrement() calls is less than the number of " +
			"Increment() calls no rtppassthrough.Source or gostream methods are called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		test.That(t, s.Decrement(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		logger.Info("when the number of Decrement() calls is equal to the number of " +
			"Increment() calls stop is called (as gostream is the data source)")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 2)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 1)

		logger.Info("then when the number of Increment() calls exceeds Decrement(), " +
			"SubscribeRTP is called again followed by Start if it returns an error")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 3)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
		})

		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 3)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})

		// once the count is at zero , calling Increment() again calls SubscribeRTP followed by Start
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 4)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})

		logger.Info("if while gostream is being used Increment is called and SubscribeRTP succeeds, Stop is called")
		subscribeRTPReturnError.Store(false)
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 5)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})

		// calling Decrement() fewer times than Increment() doesn't call any methods
		test.That(t, s.Decrement(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 5)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, startCount.Load(), test.ShouldEqual, 3)
		test.That(t, stopCount.Load(), test.ShouldEqual, 3)

		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 5)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})

		logger.Info("if while rtp_passthrough is being used the the subscription " +
			"terminates & afterwards rtp_passthrough is no longer supported, Start is called")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 6)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})

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
				test.That(t,
					"timed out waiting for Start() to be called after an in progress sub terminated unexpectedly",
					test.ShouldBeFalse)
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
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 7)
			test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
			test.That(tb, startCount.Load(), test.ShouldEqual, 4)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 4)
		})
	})

	t.Run("when the camera does not implement rtppassthrough.Source", func(t *testing.T) {
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
				test.That(t,
					"should not happen",
					test.ShouldBeFalse)
				return nil
			},
		}
		robot := mockRobot(nil)
		s := state.New(streamMock, robot, logger)
		defer func() {
			utils.UncheckedError(s.Close())
		}()
		logger.Info("the first Increment() -> Start()")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 1)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 0)
		})

		logger.Info("subsequent Increment() all calls don't call any other gostream methods")
		test.That(t, s.Increment(), test.ShouldBeNil)
		test.That(t, s.Increment(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		logger.Info("as long as the number of Decrement() calls is less than the number of Increment() calls, no gostream methods are called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		test.That(t, s.Decrement(), test.ShouldBeNil)
		time.Sleep(sleepDuration)
		test.That(t, startCount.Load(), test.ShouldEqual, 1)
		test.That(t, stopCount.Load(), test.ShouldEqual, 0)

		logger.Info("when the number of Decrement() calls is equal to the number of Increment() calls stop is called")
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 1)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
		})

		logger.Info("then when the number of Increment() calls exceeds Decrement(), Start is called again")
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
		})

		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 2)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})

		// once the count is at zero , calling Increment() again calls start
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
		})

		// set count back to zero
		test.That(t, s.Decrement(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 3)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})

		// make it so that non cancelled Decrement() would call stop to confirm that does not happen when context is cancelled
		test.That(t, s.Increment(), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, startCount.Load(), test.ShouldEqual, 4)
			test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
		})
	})

	t.Run("when in rtppassthrough mode and a resize occurs test downgrade path to gostream", func(t *testing.T) {
		var startCount atomic.Int64
		var stopCount atomic.Int64
		writeRTPCalledCtx, writeRTPCalledFunc := context.WithCancel(ctx)
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
				test.That(t, fmt.Sprintf("Unsubscribe called with unknown id: %s", id.String()), test.ShouldBeFalse)
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
		defer func() {
			utils.UncheckedError(s.Close())
		}()

		test.That(t, subscribeRTPCount.Load(), test.ShouldEqual, 0)
		test.That(t, unsubscribeCount.Load(), test.ShouldEqual, 0)
		test.That(t, writeRTPCalledCtx.Err(), test.ShouldBeNil)

		t.Run("Increment should call SubscribeRTP", func(t *testing.T) {
			test.That(t, s.Increment(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 0)
			})
			// WriteRTP is called
			<-writeRTPCalledCtx.Done()
		})

		t.Run("Resize should stop rtp_passthrough and start gostream", func(t *testing.T) {
			test.That(t, s.Resize(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 1)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 0)
			})
		})

		t.Run("Decrement should call Stop as gostream is the data source", func(t *testing.T) {
			test.That(t, s.Decrement(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 1)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
			})
		})

		t.Run("Increment should call Start as gostream is the data source", func(t *testing.T) {
			test.That(t, s.Increment(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 2)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 1)
			})
		})

		t.Run("Decrement should call Stop as gostream is the data source", func(t *testing.T) {
			test.That(t, s.Decrement(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 2)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
			})
		})

		t.Run("Increment should call Start as gostream is the data source", func(t *testing.T) {
			test.That(t, s.Increment(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 1)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 3)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 2)
			})
		})

		t.Run("Reset should call Stop as gostream is the current data source and then "+
			"Subscribe as rtp_passthrough is the new data source", func(t *testing.T) {
			test.That(t, s.Reset(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 2)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 1)
				test.That(tb, startCount.Load(), test.ShouldEqual, 3)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
			})
		})

		t.Run("Decrement should call unsubscribe as rtp_passthrough is the data source", func(t *testing.T) {
			test.That(t, s.Decrement(), test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, subscribeRTPCount.Load(), test.ShouldEqual, 2)
				test.That(tb, unsubscribeCount.Load(), test.ShouldEqual, 2)
				test.That(tb, startCount.Load(), test.ShouldEqual, 3)
				test.That(tb, stopCount.Load(), test.ShouldEqual, 3)
			})
		})
	})
}
