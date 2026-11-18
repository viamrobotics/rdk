package worksheet

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
)

// snapFloat rounds values very close to integers to that integer, for clean display.
func snapFloat(f float64) float64 {
	rounded := math.Round(f)
	if math.Abs(f-rounded) < 1e-6 {
		return rounded
	}
	return math.Round(f*100) / 100
}

// FormatPoint formats an r3.Vector for display.
func FormatPoint(v r3.Vector) string {
	return fmt.Sprintf("r3.Vector{X: %g, Y: %g, Z: %g}", snapFloat(v.X), snapFloat(v.Y), snapFloat(v.Z))
}

// FormatOrientation formats an Orientation as OrientationVectorDegrees for display.
func FormatOrientation(o spatialmath.Orientation) string {
	ovd := o.OrientationVectorDegrees()
	return fmt.Sprintf("OrientationVectorDegrees{Theta: %g, OX: %g, OY: %g, OZ: %g}",
		snapFloat(ovd.Theta), snapFloat(ovd.OX), snapFloat(ovd.OY), snapFloat(ovd.OZ))
}

// FormatPose formats a Pose showing both point and orientation.
func FormatPose(p spatialmath.Pose) string {
	return fmt.Sprintf("Point:       %s\n    Orientation: %s",
		FormatPoint(p.Point()), FormatOrientation(p.Orientation()))
}
