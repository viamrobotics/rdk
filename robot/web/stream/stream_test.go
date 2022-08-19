package webstream_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pion/webrtc/v3"
	"go.viam.com/test"

	webstream "go.viam.com/rdk/robot/web/stream"
)

var errImageRetrieval = errors.New("image retrieval failed")

type mockErrorImageSource struct {
	callsLeft int
	wg        sync.WaitGroup
}

func newMockErrorImageSource(expectedCalls int) *mockErrorImageSource {
	mock := &mockErrorImageSource{callsLeft: expectedCalls}
	mock.wg.Add(expectedCalls)
	return mock
}

func (imageSource *mockErrorImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	if imageSource.callsLeft > 0 {
		imageSource.wg.Done()
	} else {
		panic("mock image source was called too many times")
	}
	imageSource.callsLeft--
	return nil, nil, errImageRetrieval
}

type mockStream struct {
	name               string
	streamingReadyFunc func() <-chan struct{}
	inputFramesFunc    func() chan<- gostream.FrameReleasePair
}

func (mS *mockStream) StreamingReady() <-chan struct{} {
	return mS.streamingReadyFunc()
}

func (mS *mockStream) InputFrames() chan<- gostream.FrameReleasePair {
	return mS.inputFramesFunc()
}

func (mS *mockStream) Name() string {
	return mS.name
}

func (mS *mockStream) Start() {
}

func (mS *mockStream) Stop() {
}

func (mS *mockStream) TrackLocal() webrtc.TrackLocal {
	return nil
}

func TestStreamSourceErrorBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	backoffOpts := &webstream.BackoffTuningOptions{
		BaseSleep: 50 * time.Microsecond,
		MaxSleep:  250 * time.Millisecond,
	}
	calls := 25
	imgSrc := newMockErrorImageSource(calls)

	totalExpectedSleep := int64(0)
	// Note that we do not add the expected sleep duration for the last error since the
	// streaming context will be cancelled during that error.
	for i := 1; i < calls; i++ {
		totalExpectedSleep += backoffOpts.GetSleepTimeFromErrorCount(i).Nanoseconds()
	}
	str := &mockStream{}
	readyChan := make(chan struct{})
	inputChan := make(chan gostream.FrameReleasePair)
	str.streamingReadyFunc = func() <-chan struct{} {
		return readyChan
	}
	str.inputFramesFunc = func() chan<- gostream.FrameReleasePair {
		return inputChan
	}

	go webstream.StreamSource(ctx, imgSrc, str, backoffOpts)
	start := time.Now()
	readyChan <- struct{}{}
	imgSrc.wg.Wait()
	cancel()

	duration := time.Since(start).Nanoseconds()
	test.That(t, duration, test.ShouldBeGreaterThanOrEqualTo, totalExpectedSleep)
}
