package webstream_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pion/webrtc/v3"
	"go.viam.com/test"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	webstream "go.viam.com/rdk/robot/web/stream"
)

var errImageRetrieval = errors.New("image retrieval failed")

type mockErrorVideoSource struct {
	callsLeft int
	wg        sync.WaitGroup
}

func newMockErrorVideoReader(expectedCalls int) *mockErrorVideoSource {
	mock := &mockErrorVideoSource{callsLeft: expectedCalls}
	mock.wg.Add(expectedCalls)
	return mock
}

func (videoSource *mockErrorVideoSource) Read(ctx context.Context) (image.Image, func(), error) {
	if videoSource.callsLeft > 0 {
		videoSource.wg.Done()
		videoSource.callsLeft--
	}
	return nil, nil, errImageRetrieval
}

func (videoSource *mockErrorVideoSource) Close(ctx context.Context) error {
	return nil
}

type mockStream struct {
	name               string
	streamingReadyFunc func() <-chan struct{}
	inputFramesFunc    func() (chan<- gostream.MediaReleasePair[image.Image], error)
}

func (mS *mockStream) StreamingReady() (<-chan struct{}, context.Context) {
	return mS.streamingReadyFunc(), context.Background()
}

func (mS *mockStream) InputVideoFrames(props prop.Video) (chan<- gostream.MediaReleasePair[image.Image], error) {
	return mS.inputFramesFunc()
}

func (mS *mockStream) InputAudioChunks(props prop.Audio) (chan<- gostream.MediaReleasePair[wave.Audio], error) {
	return make(chan gostream.MediaReleasePair[wave.Audio]), nil
}

func (mS *mockStream) Name() string {
	return mS.name
}

func (mS *mockStream) Start() {
}

func (mS *mockStream) Stop() {
}

func (mS *mockStream) VideoTrackLocal() (webrtc.TrackLocal, bool) {
	return nil, false
}

func (mS *mockStream) AudioTrackLocal() (webrtc.TrackLocal, bool) {
	return nil, false
}

func TestStreamSourceErrorBackoff(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())

	backoffOpts := &webstream.BackoffTuningOptions{
		BaseSleep: 50 * time.Microsecond,
		MaxSleep:  250 * time.Millisecond,
		Cooldown:  time.Second,
	}
	calls := 25
	videoReader := newMockErrorVideoReader(calls)
	videoSrc := gostream.NewVideoSource(videoReader, prop.Video{})
	defer func() {
		test.That(t, videoSrc.Close(context.Background()), test.ShouldBeNil)
	}()

	totalExpectedSleep := int64(0)
	// Note that we do not add the expected sleep duration for the last error since the
	// streaming context will be cancelled during that error.
	for i := 1; i < calls; i++ {
		totalExpectedSleep += backoffOpts.GetSleepTimeFromErrorCount(i).Nanoseconds()
	}
	str := &mockStream{}
	readyChan := make(chan struct{})
	inputChan := make(chan gostream.MediaReleasePair[image.Image])
	str.streamingReadyFunc = func() <-chan struct{} {
		return readyChan
	}
	str.inputFramesFunc = func() (chan<- gostream.MediaReleasePair[image.Image], error) {
		return inputChan, nil
	}

	go webstream.StreamVideoSource(ctx, videoSrc, str, backoffOpts, logger)
	start := time.Now()
	readyChan <- struct{}{}
	videoReader.wg.Wait()
	cancel()

	duration := time.Since(start).Nanoseconds()
	test.That(t, duration, test.ShouldBeGreaterThanOrEqualTo, totalExpectedSleep)
}
