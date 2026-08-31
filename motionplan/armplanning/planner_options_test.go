package armplanning

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestNewPlannerOptionsFromExtraCollisionBuffer(t *testing.T) {
	t.Run("absent key keeps the default", func(t *testing.T) {
		opt, err := NewPlannerOptionsFromExtra(map[string]interface{}{"timeout": 30})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opt.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
	})

	t.Run("explicit zero is normalized to the default", func(t *testing.T) {
		opt, err := NewPlannerOptionsFromExtra(map[string]interface{}{"collision_buffer_mm": 0})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opt.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
	})

	t.Run("positive value is honored", func(t *testing.T) {
		opt, err := NewPlannerOptionsFromExtra(map[string]interface{}{"collision_buffer_mm": 2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opt.CollisionBufferMM, test.ShouldEqual, 2.0)
	})

	t.Run("negative value errors", func(t *testing.T) {
		_, err := NewPlannerOptionsFromExtra(map[string]interface{}{"collision_buffer_mm": -1})
		test.That(t, err, test.ShouldNotBeNil)
	})
}

// TestReadRequestSeedsPlannerOptionDefaults pins that decoding a saved request
// merges its planner_options into the defaults: a field absent from the JSON
// must decode to its default, not the type's zero value. collision_buffer_mm
// is the load-bearing case - a decoded 0 flips collision verdicts for
// geometries modeled in contact.
func TestReadRequestSeedsPlannerOptionDefaults(t *testing.T) {
	read := func(t *testing.T, body string) *PlanRequest {
		t.Helper()
		path := filepath.Join(t.TempDir(), "req.json")
		test.That(t, os.WriteFile(path, []byte(body), 0o600), test.ShouldBeNil)
		req, err := ReadRequestFromFile(path)
		test.That(t, err, test.ShouldBeNil)
		return req
	}

	t.Run("options object without the buffer key", func(t *testing.T) {
		req := read(t, `{"planner_options": {"timeout": 30}}`)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
		test.That(t, req.PlannerOptions.Timeout, test.ShouldEqual, 30.0)
	})

	t.Run("options object with the buffer key", func(t *testing.T) {
		req := read(t, `{"planner_options": {"collision_buffer_mm": 2}}`)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, 2.0)
	})

	t.Run("null options fall back to defaults", func(t *testing.T) {
		req := read(t, `{"planner_options": null}`)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
	})

	t.Run("absent options fall back to defaults", func(t *testing.T) {
		req := read(t, `{}`)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
	})
}

// TestValidatePlanRequestCollisionBuffer pins the request-level backstop: a
// request assembled with a zero buffer (however it was produced) plans with
// the default, and a negative buffer is rejected.
func TestValidatePlanRequestCollisionBuffer(t *testing.T) {
	newReq := func(t *testing.T) *PlanRequest {
		t.Helper()
		req, err := readRequestFromBytes(wineAdjustJSON)
		test.That(t, err, test.ShouldBeNil)
		return req
	}

	t.Run("zero is normalized to the default", func(t *testing.T) {
		req := newReq(t)
		req.PlannerOptions.CollisionBufferMM = 0
		test.That(t, req.validatePlanRequest(), test.ShouldBeNil)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, defaultCollisionBufferMM)
	})

	t.Run("negative is rejected", func(t *testing.T) {
		req := newReq(t)
		req.PlannerOptions.CollisionBufferMM = -1
		test.That(t, req.validatePlanRequest(), test.ShouldNotBeNil)
	})

	t.Run("positive is untouched", func(t *testing.T) {
		req := newReq(t)
		req.PlannerOptions.CollisionBufferMM = 2
		test.That(t, req.validatePlanRequest(), test.ShouldBeNil)
		test.That(t, req.PlannerOptions.CollisionBufferMM, test.ShouldEqual, 2.0)
	})
}
