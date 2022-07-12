package webstream_test

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pion/webrtc/v3"
	"go.viam.com/test"

	webstream "go.viam.com/rdk/robot/web/stream"
)

var errImageRetrieval = errors.New("image retrieval failed")

type mockErrorImageSource struct {
	called   int
	maxCalls int
}

func (imageSource *mockErrorImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	if imageSource.called < imageSource.maxCalls {
		imageSource.called++
	}
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
	imgSrc := &mockErrorImageSource{maxCalls: 25}
	totalExpectedSleep := int64(0)
	for i := 0; i < imgSrc.maxCalls; i++ {
		totalExpectedSleep += backoffOpts.GetSleepTimeFromErrorCount(i + 1).Nanoseconds()
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
	readyChan <- struct{}{}

	// Wait for the expect timeout duration, with some padding.
	time.Sleep(time.Duration(totalExpectedSleep))
	time.Sleep(1 * time.Millisecond)

	cancel()
	test.That(t, imgSrc.called, test.ShouldEqual, imgSrc.maxCalls)
}
