package robot_test

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/testutils/inject"
)

func TestSessionManager(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r := &inject.Robot{}

	r.LoggerFunc = func() logging.Logger {
		return logger
	}

	sm := robot.NewSessionManager(r, config.DefaultSessionHeartbeatWindow)
	defer sm.Close()

	// Start two arbitrary sessions.
	fooSess, err := sm.Start(ctx, "foo")
	test.That(t, err, test.ShouldBeNil)

	barSess, err := sm.Start(ctx, "bar")
	test.That(t, err, test.ShouldBeNil)

	// Assert that FindByID requires correct owner ID.
	foundSess, err := sm.FindByID(ctx, fooSess.ID(), "bar")
	test.That(t, foundSess, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, session.ErrNoSession)

	// Assert that fooSess and barSess can be found with FindByID.
	foundFooSess, err := sm.FindByID(ctx, fooSess.ID(), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, foundFooSess, test.ShouldEqual, fooSess)

	foundBarSess, err := sm.FindByID(ctx, barSess.ID(), "bar")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, foundBarSess, test.ShouldEqual, barSess)

	// Assert that fooSess and barSess can be found with All.
	allSessions := sm.All()
	test.That(t, len(allSessions), test.ShouldEqual, 2)
	test.That(t, allSessions[0], test.ShouldBeIn, fooSess, barSess)
	test.That(t, allSessions[1], test.ShouldBeIn, fooSess, barSess)
}

func TestSessionManagerExpiredSessions(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)
	r := &inject.Robot{}

	r.LoggerFunc = func() logging.Logger {
		return logger
	}

	// Use a negative duration to cause immediate heartbeat timeout and
	// session expiration.
	sm := robot.NewSessionManager(r, time.Duration(-1))
	defer sm.Close()

	_, err := sm.Start(ctx, "foo")
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessageSnippet("sessions expired").Len(),
			test.ShouldEqual, 1)
	})
}
