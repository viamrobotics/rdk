//go:build darwin

// Note: reconnecting by name is only needed on darwin

package videosource

import (
	"context"
	"testing"
	"time"

	"github.com/pion/mediadevices/pkg/driver"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Ten monitor ticks is long enough to be sure a reconnect that was going to happen would have happened.
const monitorSettleTime = 10 * testMonitorInterval

// newTestWebcam opens the given registered driver and wires it into a webcam. It bypasses NewWebcam because
// that starts the AVFoundation observer, which is not available in a test process and is not what these tests
// exercise.
func newTestWebcam(t *testing.T, d driver.Driver, idleTimeoutMs int) *webcam {
	t.Helper()
	logger := logging.NewTestLogger(t)
	label := d.Info().Label
	conf := WebcamConfig{Path: label, FrameRate: defaultFrameRate, IdleTimeoutMs: idleTimeoutMs}

	reader, opened, err := getReaderAndDriver(labelFilter(label, true, false), label, makeConstraints(&conf, logger), logger)
	test.That(t, err, test.ShouldBeNil)

	c := newWebcam(resource.NewName(camera.API, "test-webcam"), conf, label, reader, opened, findReaderAndDriver, logger)
	c.monitorInterval = testMonitorInterval
	c.start()
	t.Cleanup(func() {
		test.That(t, c.Close(context.Background()), test.ShouldBeNil)
	})
	return c
}

type webcamSnapshot struct {
	disconnected     bool
	sawOtherSameName bool
	targetPath       string
	driverLabel      string
}

func snapshot(c *webcam) webcamSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := webcamSnapshot{
		disconnected:     c.disconnected,
		sawOtherSameName: c.sawOtherSameName,
		targetPath:       c.targetPath,
	}
	if c.driver != nil {
		s.driverLabel = c.driver.Info().Label
	}
	return s
}

// unplug mimics what the darwin observer does when a device disappears: the device stops reporting as
// available and its registration is removed from the driver manager.
func unplug(fake *fakeCamera, d driver.Driver) {
	fake.available.Store(false)
	driver.GetManager().Delete(d.ID())
}

func waitForDisconnect(t *testing.T, c *webcam) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, snapshot(c).disconnected, test.ShouldBeTrue)
	})
}

func waitForReconnect(t *testing.T, c *webcam, wantLabel string) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		s := snapshot(c)
		test.That(tb, s.disconnected, test.ShouldBeFalse)
		test.That(tb, s.targetPath, test.ShouldEqual, wantLabel)
		test.That(tb, s.driverLabel, test.ShouldEqual, wantLabel)
	})
}

func TestMonitorReconnectsByNameWhenPathChanges(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-replug-a", "rdk-test-replug-cam", true)
	c := newTestWebcam(t, a, 0)

	unplug(fakeA, a)
	waitForDisconnect(t, c)

	// A different camera appears while disconnected; it must not be picked up by the name fallback.
	registerFakeCamera(t, "rdk-test-replug-decoy", "rdk-test-replug-other-cam", true)
	time.Sleep(monitorSettleTime)
	test.That(t, snapshot(c).disconnected, test.ShouldBeTrue)

	// The same camera comes back under a new UID, as happens when it is plugged into a different port.
	registerFakeCamera(t, "rdk-test-replug-a2", "rdk-test-replug-cam", true)
	waitForReconnect(t, c, "rdk-test-replug-a2")
}

func TestMonitorReconnectsByNameWhilePaused(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-idle-a", "rdk-test-idle-cam", true)
	c := newTestWebcam(t, a, testIdleTimeoutMs)
	waitIdle(t, c)

	unplug(fakeA, a)
	waitForDisconnect(t, c)

	registerFakeCamera(t, "rdk-test-idle-a2", "rdk-test-idle-cam", true)
	waitForReconnect(t, c, "rdk-test-idle-a2")
	// Reconnecting does not resume reads; the next Images call must wake the camera on the new driver.
	test.That(t, idleStateOf(c), test.ShouldEqual, stateIdle)

	waitFrame(t, c)
	test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
	test.That(t, snapshot(c).driverLabel, test.ShouldEqual, "rdk-test-idle-a2")
}

func TestMonitorSkipsNameFallbackAfterSeeingDuplicate(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-dup-a", "rdk-test-dup-cam", true)
	_, b := registerFakeCamera(t, "rdk-test-dup-b", "rdk-test-dup-cam", true)
	c := newTestWebcam(t, a, 0)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, snapshot(c).sawOtherSameName, test.ShouldBeTrue)
	})

	// Removing the duplicate must not re-enable the fallback.
	driver.GetManager().Delete(b.ID())
	unplug(fakeA, a)
	waitForDisconnect(t, c)

	registerFakeCamera(t, "rdk-test-dup-a2", "rdk-test-dup-cam", true)
	time.Sleep(monitorSettleTime)
	s := snapshot(c)
	test.That(t, s.disconnected, test.ShouldBeTrue)
	test.That(t, s.driverLabel, test.ShouldEqual, "")

	// Reconnecting by the original path is unaffected by the flag.
	registerFakeCamera(t, "rdk-test-dup-a", "rdk-test-dup-cam", true)
	waitForReconnect(t, c, "rdk-test-dup-a")
}

func TestMonitorRefusesNameFallbackWhenMultipleAppear(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-multi-a", "rdk-test-multi-cam", true)
	c := newTestWebcam(t, a, 0)

	unplug(fakeA, a)
	waitForDisconnect(t, c)

	// Both candidates are registered unavailable first so that a monitor tick landing between the two
	// registrations cannot open the lone candidate; they are only made available once both exist. Such a
	// tick still finds both registered when findReaderAndDriverByName re-checks after opening one.
	fakeA2, _ := registerFakeCamera(t, "rdk-test-multi-a2", "rdk-test-multi-cam", false)
	fakeA3, a3 := registerFakeCamera(t, "rdk-test-multi-a3", "rdk-test-multi-cam", false)
	fakeA2.available.Store(true)
	fakeA3.available.Store(true)

	time.Sleep(monitorSettleTime)
	s := snapshot(c)
	test.That(t, s.disconnected, test.ShouldBeTrue)
	test.That(t, s.driverLabel, test.ShouldEqual, "")

	// Once only one candidate remains the fallback proceeds, since no duplicate was ever seen while connected.
	driver.GetManager().Delete(a3.ID())
	waitForReconnect(t, c, "rdk-test-multi-a2")
}
