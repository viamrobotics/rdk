package protoutils

import (
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestRoundtrip(t *testing.T) {
	m1 := map[string]interface{}{
		"d":   5.4,
		"av":  spatialmath.AngularVelocity{1, 2, 3},
		"vv":  r3.Vector{1, 2, 3},
		"ea":  &spatialmath.EulerAngles{Roll: 3, Pitch: 5, Yaw: 4},
		"q":   &spatialmath.Quaternion{1, 2, 3, 4},
		"ov":  &spatialmath.OrientationVector{Theta: 1, OX: 2, OY: 3, OZ: 4},
		"ovd": &spatialmath.OrientationVectorDegrees{Theta: 1, OX: 2, OY: 3, OZ: 4},
		"aa":  &spatialmath.R4AA{Theta: 1, RX: 2, RY: 3, RZ: 4},
		"gp":  geo.NewPoint(12, 13),
	}

	p, err := ReadingGoToProto(m1)
	test.That(t, err, test.ShouldBeNil)

	m2, err := ReadingProtoToGo(p)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m2, test.ShouldResemble, m1)
}
