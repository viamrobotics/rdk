package armplanning

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
)

// TestNewTrajGenDefaults verifies that nil optional fields are replaced by their defaults.
func TestNewTrajGenDefaults(t *testing.T) {
	tg := NewTrajGen(nil, nil, nil, nil, 1.0, 2.0, nil)
	test.That(t, tg.PathToleranceDeltaRads, test.ShouldEqual, defaultTrajGenPathToleranceDeltaRads)
	test.That(t, tg.PathColinearizationRatio, test.ShouldEqual, defaultTrajGenPathColinearizationRatio)
	test.That(t, tg.WaypointDeduplicationToleranceRads, test.ShouldEqual, defaultTrajGenWaypointDeduplicationToleranceRads)
	test.That(t, tg.SamplingFreqHz, test.ShouldEqual, defaultTrajGenSamplingFreqHz)
	// Non-optional fields pass through unchanged.
	test.That(t, tg.VelocityLimitsRadsPerSec, test.ShouldEqual, 1.0)
	test.That(t, tg.AccelerationLimitsRadsPerSec2, test.ShouldEqual, 2.0)
}

// TestNewTrajGenExplicitValues verifies that non-nil optional fields override the defaults.
func TestNewTrajGenExplicitValues(t *testing.T) {
	pt := 0.05
	cr := 0.3
	dd := 0.002
	sf := 20.0
	tg := NewTrajGen(nil, &pt, &cr, &dd, 3.0, 4.0, &sf)
	test.That(t, tg.PathToleranceDeltaRads, test.ShouldEqual, 0.05)
	test.That(t, tg.PathColinearizationRatio, test.ShouldEqual, 0.3)
	test.That(t, tg.WaypointDeduplicationToleranceRads, test.ShouldEqual, 0.002)
	test.That(t, tg.SamplingFreqHz, test.ShouldEqual, 20.0)
}

// TestTrajGenConfigValidate checks all validation rules.
func TestTrajGenConfigValidate(t *testing.T) {
	valid := TrajGenConfig{
		Service:                       "my_svc",
		VelocityLimitsRadsPerSec:      1.0,
		AccelerationLimitsRadsPerSec2: 2.0,
	}

	t.Run("valid config returns no remote dependency", func(t *testing.T) {
		// The trajectory generator is now an in-process trajex cgo backend, so Validate
		// declares no remote mlmodel dependency.
		deps, err := valid.Validate("path")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldBeEmpty)
	})

	t.Run("missing service is allowed", func(t *testing.T) {
		// Service is vestigial with the in-process backend; its absence is no longer an error.
		cfg := valid
		cfg.Service = ""
		_, err := cfg.Validate("path")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("zero velocity limit", func(t *testing.T) {
		cfg := valid
		cfg.VelocityLimitsRadsPerSec = 0
		_, err := cfg.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("negative velocity limit", func(t *testing.T) {
		cfg := valid
		cfg.VelocityLimitsRadsPerSec = -1
		_, err := cfg.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("zero acceleration limit", func(t *testing.T) {
		cfg := valid
		cfg.AccelerationLimitsRadsPerSec2 = 0
		_, err := cfg.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
	})
}

// TestDetectMovingFrames verifies the single-pass moving-frame detection logic.
func TestDetectMovingFrames(t *testing.T) {
	t.Run("empty trajectory returns empty map", func(t *testing.T) {
		moving := detectMovingFrames(motionplan.Trajectory{})
		test.That(t, moving, test.ShouldBeEmpty)
	})

	t.Run("single step — no pairs to compare, fallback treats all as moving", func(t *testing.T) {
		traj := motionplan.Trajectory{{"arm": []float64{0, 0, 0}}}
		moving := detectMovingFrames(traj)
		test.That(t, moving, test.ShouldContainKey, "arm")
	})

	t.Run("frame with identical inputs is not moving; fallback fires for all-static", func(t *testing.T) {
		traj := motionplan.Trajectory{
			{"arm": []float64{1, 2, 3}},
			{"arm": []float64{1, 2, 3}},
		}
		// Nothing moved → fallback treats all frames as moving.
		moving := detectMovingFrames(traj)
		test.That(t, moving, test.ShouldContainKey, "arm")
	})

	t.Run("frame with changing inputs is detected as moving", func(t *testing.T) {
		traj := motionplan.Trajectory{
			{"arm": []float64{0, 0, 0}},
			{"arm": []float64{1, 0, 0}},
			{"arm": []float64{2, 0, 0}},
		}
		moving := detectMovingFrames(traj)
		test.That(t, moving, test.ShouldContainKey, "arm")
	})

	t.Run("only the moving frame among multiple frames is returned", func(t *testing.T) {
		traj := motionplan.Trajectory{
			{"arm": []float64{0, 0, 0}, "gripper": []float64{0}},
			{"arm": []float64{1, 0, 0}, "gripper": []float64{0}},
		}
		moving := detectMovingFrames(traj)
		test.That(t, moving, test.ShouldContainKey, "arm")
		test.That(t, moving, test.ShouldNotContainKey, "gripper")
	})

	t.Run("detects change that only appears in a later step", func(t *testing.T) {
		traj := motionplan.Trajectory{
			{"arm": []float64{0, 0, 0}},
			{"arm": []float64{0, 0, 0}},
			{"arm": []float64{0, 0, 1}}, // change only in last step
		}
		moving := detectMovingFrames(traj)
		test.That(t, moving, test.ShouldContainKey, "arm")
	})
}

// TestInferTrajGenEmptyWaypoints verifies the early-return for an empty waypoint list.
func TestInferTrajGenEmptyWaypoints(t *testing.T) {
	result, err := inferTrajGen(context.Background(), nil, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldNotBeNil)
	test.That(t, result.configurations, test.ShouldBeEmpty)
}
