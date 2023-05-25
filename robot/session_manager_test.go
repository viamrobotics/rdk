package robot_test

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/testutils/inject"
)

func TestSessionManager(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}

	r.LoggerFunc = func() golog.Logger {
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
	logger, logs := golog.NewObservedTestLogger(t)
	r := &inject.Robot{}

	r.LoggerFunc = func() golog.Logger {
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

func TestSessionManagerExpiredSessionsDuringClose(t *testing.T) {
	// Primarily a regression test for RSDK-3176

	ctx := context.Background()
	logger, logs := golog.NewObservedTestLogger(t)
	r := &inject.Robot{}

	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	// Setup inject robot to always return a NewNotFoundError for ResourceByName.
	// We want to mimic the behavior of the robot when the resource manager has
	// been closed, and the resource has been removed from the graph.
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return nil, resource.NewNotFoundError(name)
	}

	heartbeatWindow := 100 * time.Millisecond
	sm := robot.NewSessionManager(r, heartbeatWindow)

	// Start a new session and associate a generic resource with it.
	fooSess, err := sm.Start(ctx, "fooSess")
	test.That(t, err, test.ShouldBeNil)
	sm.AssociateResource(fooSess.ID(), generic.Named("foo"))

	// Sleep for the heartbeat window to cause a session expiration. Close the
	// session manager immediately after to potentially cause the expireLoop to
	// try to handle "foo"'s expired session.
	time.Sleep(heartbeatWindow)
	sm.Close()

	// Assert that no session expiration errors are logged (expireLoop returned
	// early after double-checking context expiration).
	test.That(t, logs.FilterMessageSnippet("sessions expired").Len(),
		test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("tried to stop some resources").Len(),
		test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("failed to stop some resources").Len(),
		test.ShouldEqual, 0)
}
