package armplanning

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Geometry signature unit test
func TestGeometrySignature(t *testing.T) {
	box1, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "box1")
	test.That(t, err, test.ShouldBeNil)
	box2, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "box2")
	test.That(t, err, test.ShouldBeNil)
	box3, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "box3")
	test.That(t, err, test.ShouldBeNil)

	t.Run("deterministic ordering", func(t *testing.T) {
		// Same geometries in different order should produce same signature
		sig1 := geometrySignature(
			[]spatialmath.Geometry{box1, box2},
			[]spatialmath.Geometry{box3},
		)
		sig2 := geometrySignature(
			[]spatialmath.Geometry{box2, box1},
			[]spatialmath.Geometry{box3},
		)
		test.That(t, sig1, test.ShouldEqual, sig2)
	})

	t.Run("distinguishes moving vs static", func(t *testing.T) {
		// box1 moving vs box1 static should produce different signatures
		sig1 := geometrySignature(
			[]spatialmath.Geometry{box1},
			[]spatialmath.Geometry{},
		)
		sig2 := geometrySignature(
			[]spatialmath.Geometry{},
			[]spatialmath.Geometry{box1},
		)
		test.That(t, sig1, test.ShouldNotEqual, sig2)
	})

	t.Run("different geometries produce different signatures", func(t *testing.T) {
		sig1 := geometrySignature(
			[]spatialmath.Geometry{box1},
			[]spatialmath.Geometry{box2},
		)
		sig2 := geometrySignature(
			[]spatialmath.Geometry{box1},
			[]spatialmath.Geometry{box3},
		)
		test.That(t, sig1, test.ShouldNotEqual, sig2)
	})

	t.Run("empty geometries", func(t *testing.T) {
		sig := geometrySignature(
			[]spatialmath.Geometry{},
			[]spatialmath.Geometry{},
		)
		test.That(t, sig, test.ShouldEqual, "")
	})
}

// Collision constraint cache functional test.
func TestCollisionConstraintCache(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// Set up a simple frame system with an arm
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.World())
	test.That(t, err, test.ShouldBeNil)

	seedMap := referenceframe.NewZeroInputs(fs).ToLinearInputs()

	// Create geometries for testing
	box1, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "box1")
	test.That(t, err, test.ShouldBeNil)
	box2, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "box2")
	test.That(t, err, test.ShouldBeNil)

	// Create a minimal planContext with just what we need for caching
	pc := &planContext{
		fs:       fs,
		planOpts: &PlannerOptions{CollisionBufferMM: 1.0},
		request: &PlanRequest{
			WorldState:  &referenceframe.WorldState{},
			Constraints: &motionplan.Constraints{},
		},
		logger:                   logger,
		collisionConstraintCache: make(map[string]*collisionConstraintCacheEntry),
	}

	movingGeoms := []spatialmath.Geometry{box1}
	staticGeoms := []spatialmath.Geometry{box2}

	t.Run("cache miss creates new entry", func(t *testing.T) {
		test.That(t, len(pc.collisionConstraintCache), test.ShouldEqual, 0)

		constraints1, err := pc.getOrCreateCollisionConstraints(movingGeoms, staticGeoms, seedMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, constraints1, test.ShouldNotBeNil)
		test.That(t, len(pc.collisionConstraintCache), test.ShouldEqual, 1)
	})

	t.Run("cache hit returns same constraints", func(t *testing.T) {
		constraints1, err := pc.getOrCreateCollisionConstraints(movingGeoms, staticGeoms, seedMap)
		test.That(t, err, test.ShouldBeNil)

		constraints2, err := pc.getOrCreateCollisionConstraints(movingGeoms, staticGeoms, seedMap)
		test.That(t, err, test.ShouldBeNil)

		// Should be the exact same map (same pointer)
		test.That(t, constraints1, test.ShouldEqual, constraints2)
		// Cache should still have only one entry
		test.That(t, len(pc.collisionConstraintCache), test.ShouldEqual, 1)
	})

	t.Run("different geometries create new cache entry", func(t *testing.T) {
		box3, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 10}, "box3")
		test.That(t, err, test.ShouldBeNil)

		differentMovingGeoms := []spatialmath.Geometry{box3}

		constraints1, err := pc.getOrCreateCollisionConstraints(movingGeoms, staticGeoms, seedMap)
		test.That(t, err, test.ShouldBeNil)

		constraints2, err := pc.getOrCreateCollisionConstraints(differentMovingGeoms, staticGeoms, seedMap)
		test.That(t, err, test.ShouldBeNil)

		// Should be different maps
		test.That(t, constraints1, test.ShouldNotEqual, constraints2)
		// Cache should now have two entries
		test.That(t, len(pc.collisionConstraintCache), test.ShouldEqual, 2)
	})
}
