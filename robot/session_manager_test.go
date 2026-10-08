package robot_test

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
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
	// The deferred Close is necessary in case any of the asserts between here and the second Close fail.
	// Double closing the session manager will not cause issues.
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

	// The following test assertions use reflection to deep-equals the session objects.
	// The session manager's expireLoop might be writing to those fields while reflection is reading those fields.
	// Thus we close the session manager before proceeding to avoid a data race.
	sm.Close()

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

func TestSessionManagerExpiredActivity(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	activityLogs := logging.NewObservedActivityLogger(t, logger)
	r := &inject.Robot{}

	r.LoggerFunc = func() logging.Logger {
		return logger
	}

	// Use a negative duration to cause immediate heartbeat timeout and
	// session expiration.
	sm := robot.NewSessionManager(r, time.Duration(-1))
	defer sm.Close()

	sess, err := sm.Start(ctx, "foo")
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, activityLogs.Len(), test.ShouldEqual, 1)
	})
	entry := activityLogs.All()[0].ContextMap()
	test.That(t, entry["activity"], test.ShouldEqual, "liveness")
	test.That(t, entry["event"], test.ShouldEqual, "expired")
	test.That(t, entry["session_id"], test.ShouldEqual, sess.ID().String())
}

func TestSessionManagerStopsOutsideLock(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	blockedName := arm.Named("blocked")
	otherName := arm.Named("other")
	blockedStopCalled := make(chan struct{}, 1)
	releaseBlockedStop := make(chan struct{})
	otherStopCalled := make(chan struct{}, 1)
	arms := map[resource.Name]*inject.Arm{
		blockedName: {StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			trySignal(blockedStopCalled)
			<-releaseBlockedStop
			return nil
		}},
		otherName: {StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			trySignal(otherStopCalled)
			return nil
		}},
	}
	r := &inject.Robot{}
	r.LoggerFunc = func() logging.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{blockedName, otherName}
	}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return arms[name], nil
	}

	sm := robot.NewSessionManager(r, time.Second)
	defer sm.Close()
	defer close(releaseBlockedStop)

	fooSess, err := sm.Start(ctx, "foo")
	test.That(t, err, test.ShouldBeNil)
	sm.AssociateResource(fooSess.ID(), blockedName)
	select {
	case <-blockedStopCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("resource of the expired session was never stopped")
	}

	sessionCallsDone := make(chan error, 1)
	go func() {
		barSess, err := sm.Start(ctx, "bar")
		if err == nil {
			_, err = sm.FindByID(ctx, barSess.ID(), "bar")
		}
		if err == nil {
			sm.AssociateResource(barSess.ID(), otherName)
		}
		sessionCallsDone <- err
	}()
	select {
	case err := <-sessionCallsDone:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(time.Second):
		t.Fatal("session calls waited on a blocked Stop")
	}

	select {
	case <-otherStopCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("a blocked Stop delayed the expiry of another session")
	}
}

func TestSessionManagerBoundsRemoteStops(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	localName := arm.Named("local")
	remoteName := arm.Named("rem1arm")
	localHasDeadline := make(chan bool, 1)
	remoteHasDeadline := make(chan bool, 1)
	recordDeadline := func(ch chan bool) func(context.Context, map[string]interface{}) error {
		return func(ctx context.Context, extra map[string]interface{}) error {
			_, ok := ctx.Deadline()
			select {
			case ch <- ok:
			default:
			}
			return nil
		}
	}
	arms := map[resource.Name]*inject.Arm{
		localName:  {StopFunc: recordDeadline(localHasDeadline)},
		remoteName: {StopFunc: recordDeadline(remoteHasDeadline)},
	}
	r := &inject.Robot{}
	r.LoggerFunc = func() logging.Logger {
		return logger
	}
	// The robot lists a remote resource with its remote set; sessions associate it by simple name.
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{localName, {API: arm.API, Remote: "rem1", Name: "rem1arm"}}
	}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return arms[name], nil
	}

	sm := robot.NewSessionManager(r, 100*time.Millisecond)
	defer sm.Close()

	sess, err := sm.Start(ctx, "foo")
	test.That(t, err, test.ShouldBeNil)
	sm.AssociateResource(sess.ID(), localName)
	sm.AssociateResource(sess.ID(), remoteName)

	select {
	case hasDeadline := <-localHasDeadline:
		test.That(t, hasDeadline, test.ShouldBeFalse)
	case <-time.After(5 * time.Second):
		t.Fatal("local resource was never stopped")
	}
	select {
	case hasDeadline := <-remoteHasDeadline:
		test.That(t, hasDeadline, test.ShouldBeTrue)
	case <-time.After(5 * time.Second):
		t.Fatal("remote resource was never stopped")
	}
}

func trySignal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
