package coverage

import (
	"testing"
	"fmt"
	
	"go.viam.com/test"
	"go.viam.com/rdk/spatialmath"
	"github.com/golang/geo/r3"
)

func TestReadPLY(t *testing.T) {
	m, err := ReadPLY("/home/peter/Downloads/lod_100_ascii.ply")
	test.That(t, err, test.ShouldBeNil)
	//~ MeshWaypoints(m, n, 0, nil)
	
	tri := spatialmath.NewTriangle(
		r3.Vector{100, 0, 100},
		r3.Vector{0, 100, 50},
		r3.Vector{0, -100, 150},
	)
	
	//~ cameraPose := spatialmath.NewPose(
		//~ r3.Vector{0, -100, 250},
		//~ &spatialmath.OrientationVector{OX:1},
	//~ )
	
	fmt.Println(tri.ClosestInsidePoint(r3.Vector{100, 0, 100.001}))
	//~ mesh1 := spatialmath.NewMesh(spatialmath.NewZeroPose(), []*spatialmath.Triangle{tri})
	wps, err := MeshWaypoints(m, 10, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wps), test.ShouldBeGreaterThan, 0)
	//~ for _, wp := range wps {
		//~ fmt.Println(spatialmath.PoseToProtobuf(spatialmath.Compose(cameraPose, wp)))
	//~ }
}
