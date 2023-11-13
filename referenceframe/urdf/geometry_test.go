package urdf

import (
	"encoding/xml"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestGeometrySerialization(t *testing.T) {
	box, err := spatialmath.NewBox(
		spatialmath.NewPose(r3.Vector{X: 10, Y: 2, Z: 3}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 30}),
		r3.Vector{X: 4, Y: 5, Z: 6},
		"",
	)
	test.That(t, err, test.ShouldBeNil)
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 3.3, "")
	test.That(t, err, test.ShouldBeNil)
	capsule, err := spatialmath.NewCapsule(spatialmath.NewZeroPose(), 1, 10, "")
	test.That(t, err, test.ShouldBeNil)

	testCases := []struct {
		name    string
		g       spatialmath.Geometry
		success bool
	}{
		{"box", box, true},
		{"sphere", sphere, true},
		{"capsule", capsule, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urdf, err := newCollision(tc.g)
			if !tc.success {
				test.That(t, err.Error(), test.ShouldContainSubstring, errGeometryTypeUnsupported.Error())
				return
			}
			test.That(t, err, test.ShouldBeNil)
			bytes, err := xml.MarshalIndent(urdf, "", "  ")
			var urdf2 collision
			xml.Unmarshal(bytes, &urdf2)
			g2, err := urdf2.parse()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, tc.g.AlmostEqual(g2), test.ShouldBeTrue)
		})
	}
}
