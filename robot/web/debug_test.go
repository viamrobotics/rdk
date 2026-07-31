package web

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
)

// TestDebugEndpointsGatedByWebProfile verifies that the /debug endpoints (pprof and the
// resource graph visualization) are only exposed when the web profile option is enabled.
func TestDebugEndpointsGatedByWebProfile(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, robot := setupRobotCtx(t)

	const dot = "digraph rdk {}"
	robot.(*inject.Robot).ExportResourcesAsDotFunc = func(index int) (resource.GetSnapshotInfo, error) {
		return resource.GetSnapshotInfo{
			Snapshot: resource.Snapshot{Dot: dot, CreatedAt: time.Now()},
			Index:    0,
			Count:    1,
		}, nil
	}

	get := func(t *testing.T, url string) (int, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		test.That(t, err, test.ShouldBeNil)
		resp, err := http.DefaultClient.Do(req)
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, resp.Body.Close(), test.ShouldBeNil) }()
		body, err := io.ReadAll(resp.Body)
		test.That(t, err, test.ShouldBeNil)
		return resp.StatusCode, string(body)
	}

	startWeb := func(t *testing.T, enableWebProfile bool) string {
		t.Helper()
		svc := New(robot, logger)
		options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		options.Pprof = enableWebProfile
		test.That(t, svc.Start(ctx, options), test.ShouldBeNil)
		t.Cleanup(func() { test.That(t, svc.Close(context.Background()), test.ShouldBeNil) })
		return "http://" + addr
	}

	// Sanity check: the web profile option defaults to disabled.
	test.That(t, weboptions.New().Pprof, test.ShouldBeFalse)

	t.Run("disabled by default", func(t *testing.T) {
		base := startWeb(t, false)

		code, body := get(t, base+"/debug/pprof/")
		test.That(t, code, test.ShouldNotEqual, http.StatusOK)
		test.That(t, body, test.ShouldNotContainSubstring, "Types of profiles available")

		code, body = get(t, base+"/debug/graph?layout=text")
		test.That(t, code, test.ShouldNotEqual, http.StatusOK)
		test.That(t, body, test.ShouldNotContainSubstring, dot)
	})

	t.Run("enabled with web profile", func(t *testing.T) {
		base := startWeb(t, true)

		code, body := get(t, base+"/debug/pprof/")
		test.That(t, code, test.ShouldEqual, http.StatusOK)
		test.That(t, body, test.ShouldContainSubstring, "Types of profiles available")

		code, body = get(t, base+"/debug/graph?layout=text")
		test.That(t, code, test.ShouldEqual, http.StatusOK)
		test.That(t, body, test.ShouldContainSubstring, dot)
	})
}
