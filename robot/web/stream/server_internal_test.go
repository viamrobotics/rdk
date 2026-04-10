package webstream

import (
	"context"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/web/stream/state"
	"go.viam.com/rdk/testutils/inject"
)

// fakeVideoEncoder is a no-op encoder used to satisfy gostream.NewStream.
type fakeVideoEncoder struct{}

func (f *fakeVideoEncoder) Encode(_ context.Context, _ image.Image) ([]byte, error) {
	return nil, nil
}

func (f *fakeVideoEncoder) Close() error { return nil }

// fakeVideoEncoderFactory produces fakeVideoEncoders.
type fakeVideoEncoderFactory struct{}

func (f *fakeVideoEncoderFactory) New(_, _, _ int, _ logging.Logger) (codec.VideoEncoder, error) {
	return &fakeVideoEncoder{}, nil
}

func (f *fakeVideoEncoderFactory) MIMEType() string { return "image/fake" }

// makeTestStream creates a gostream.Stream with the given name using a fake encoder.
func makeTestStream(t *testing.T, name string, logger logging.Logger) gostream.Stream {
	t.Helper()
	stream, err := gostream.NewStream(gostream.StreamConfig{
		Name:                name,
		VideoEncoderFactory: &fakeVideoEncoderFactory{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	return stream
}

const (
	testDebugInterval = 10 * time.Millisecond
	testWarnInterval  = 50 * time.Millisecond
)

// newTestServer builds a test Server with minimal fields without starting the background monitor goroutine.
// It uses short throttle intervals for fast tests.
func newTestServer(r *inject.Robot, logger logging.Logger) *Server {
	closedCtx, closedFn := context.WithCancel(context.Background())
	return &Server{
		closedCtx:          closedCtx,
		closedFn:           closedFn,
		robot:              r,
		logger:             logger,
		nameToStreamState:  map[string]*state.StreamState{},
		videoSources:       map[string]gostream.HotSwappableVideoSource{},
		streamErrors:       map[string]*streamErrorState{},
		debugLogInterval:   testDebugInterval,
		warnRepeatInterval: testWarnInterval,
		isAlive:            true,
	}
}

// filterLogsByLevelAndMessage returns log entries matching the given level
// whose message contains the given snippet.
func filterLogsByLevelAndMessage(logs []observer.LoggedEntry, level zapcore.Level, snippet string) []observer.LoggedEntry {
	var result []observer.LoggedEntry
	for _, entry := range logs {
		if entry.Level == level && strings.Contains(entry.Message, snippet) {
			result = append(result, entry)
		}
	}
	return result
}

func TestRemoveMissingStreams_LogThrottling(t *testing.T) {
	logger, observedLogs := logging.NewObservedTestLogger(t)

	r := &inject.Robot{}

	server := newTestServer(r, logger)
	defer server.closedFn()

	// Create a stream state for "cam1".
	stream := makeTestStream(t, "cam1", logger)
	server.nameToStreamState["cam1"] = state.New(stream, r, logger)

	buildErr := fmt.Errorf("build error: missing dependency")

	// Configure robot to return a non-NotFound error for cam1.
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name == camera.Named("cam1") {
			return nil, buildErr
		}
		return nil, resource.NewNotFoundError(name)
	}

	const msg = "Camera unavailable"

	// --- First call: new error should be logged at WARN ---
	observedLogs.TakeAll() // clear any setup logs
	server.removeMissingStreams()

	allLogs := observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 1)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 0)

	// --- Immediate repeat: should be suppressed (within debug interval) ---
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 0)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 0)

	// --- After debug interval: should log at DEBUG ---
	time.Sleep(testDebugInterval)
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 0)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 1)

	// --- After warn interval: should re-WARN even though error is the same ---
	time.Sleep(testWarnInterval)
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 1)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 0)

	// --- Camera becomes healthy: should clear tracked state ---
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name == camera.Named("cam1") {
			return &inject.Camera{}, nil
		}
		return nil, resource.NewNotFoundError(name)
	}
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 0)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 0)
	test.That(t, server.streamErrors, test.ShouldNotContainKey, "cam1")

	// --- Same error returns after recovery: should WARN again ---
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name == camera.Named("cam1") {
			return nil, buildErr
		}
		return nil, resource.NewNotFoundError(name)
	}
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 1)

	// --- Different error: should WARN again regardless of timing ---
	newErr := fmt.Errorf("config validation failed")
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name == camera.Named("cam1") {
			return nil, newErr
		}
		return nil, resource.NewNotFoundError(name)
	}
	server.removeMissingStreams()

	allLogs = observedLogs.TakeAll()
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.WarnLevel, msg)), test.ShouldEqual, 1)
	test.That(t, len(filterLogsByLevelAndMessage(allLogs, zapcore.DebugLevel, msg)), test.ShouldEqual, 0)
}
