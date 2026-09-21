package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func testStaticScene(t *testing.T, dims r3.Vector) []spatialmath.Geometry {
	t.Helper()
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 100}), dims, "wall")
	test.That(t, err, test.ShouldBeNil)
	sphere, err := spatialmath.NewSphere(spatialmath.NewPoseFromPoint(r3.Vector{Y: 200}), 30, "ball")
	test.That(t, err, test.ShouldBeNil)
	return []spatialmath.Geometry{box, sphere}
}

func TestSDFRegistrySharing(t *testing.T) {
	sceneA := testStaticScene(t, r3.Vector{X: 50, Y: 400, Z: 400})

	c1 := NewCollisionCache()
	sdf1 := c1.SDFFor(sceneA)
	test.That(t, sdf1, test.ShouldNotBeNil)

	// A different plan's cache, same scene in reversed order: same field.
	c2 := NewCollisionCache()
	sdf2 := c2.SDFFor([]spatialmath.Geometry{sceneA[1], sceneA[0]})
	test.That(t, sdf2, test.ShouldEqual, sdf1)

	// Same labels and poses but a resized geometry must not reuse the field.
	sceneB := testStaticScene(t, r3.Vector{X: 50, Y: 400, Z: 800})
	sdf3 := NewCollisionCache().SDFFor(sceneB)
	test.That(t, sdf3, test.ShouldNotBeNil)
	test.That(t, sdf3, test.ShouldNotEqual, sdf1)
}

func TestSDFRegistryEviction(t *testing.T) {
	r := sdfRegistry{entries: map[uint64]*sdfRegistryEntry{}}
	scene := testStaticScene(t, r3.Vector{X: 50, Y: 400, Z: 400})
	sdf := spatialmath.NewVoxelSDF(scene, sdfResolutionMM)
	test.That(t, sdf, test.ShouldNotBeNil)

	for key := uint64(0); key < sdfRegistryCap+3; key++ {
		r.insert(key, sdf)
		if key > 0 {
			// Keep key 0 warm so eviction drops the stale middle keys instead.
			_, ok := r.lookup(0)
			test.That(t, ok, test.ShouldBeTrue)
		}
	}
	test.That(t, len(r.entries), test.ShouldEqual, sdfRegistryCap)
	_, ok := r.lookup(0)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = r.lookup(1)
	test.That(t, ok, test.ShouldBeFalse)
}
