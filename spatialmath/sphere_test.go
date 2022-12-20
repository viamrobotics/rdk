package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestSphere(point r3.Vector, radius float64, label string) Geometry {
	sphere, _ := NewSphere(point, radius, label)
	return sphere
}

func TestNewSphere(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test sphere created from NewBox method
	geometry, err := NewSphere(offset.Point(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &sphere{pose: NewPoseFromPoint(offset.Point()), radius: 1})
	_, err = NewSphere(offset.Point(), -1, "")
	test.That(t, err.Error(), test.ShouldContainSubstring, newBadGeometryDimensionsError(&sphere{}).Error())

	// test sphere created from GeometryCreator with offset
	gc, err := NewSphereCreator(1, offset, "")
	test.That(t, err, test.ShouldBeNil)
	geometry = gc.NewGeometry(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestSphereAlmostEqual(t *testing.T) {
	original := makeTestSphere(r3.Vector{}, 1, "")
	good := makeTestSphere(r3.Vector{1e-16, 1e-16, 1e-16}, 1+1e-16, "")
	bad := makeTestSphere(r3.Vector{1e-2, 1e-2, 1e-2}, 1+1e-2, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestSphereVertices(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(makeTestSphere(r3.Vector{}, 1, "").Vertices()[0], r3.Vector{}, 1e-8), test.ShouldBeTrue)
}

func TestSpherePC(t *testing.T) {
	pt := r3.Vector{-2, -2, -2}
	radius := 2.5
	label := ""
	sphere := &sphere{NewPoseFromPoint(pt), radius, label}
	myMap := make(map[string]interface{})
	myMap["resolution"] = 4. // using custom point density
	output := sphere.ToPointCloud(myMap)
	checkAgainst := []r3.Vector{r3.Vector{-2, 0.500000000000000000000000, -2}, r3.Vector{-3.158663408722436560793767, -.055555555555555524716027, -0.938569405115511345982782}, r3.Vector{-1.818268272954845610200891, -0.611111111111111049432054, -4.070739296412315688655781},
		r3.Vector{-0.565895851548090411675673, -1.166666666666666518636930, -0.129465090689676765034477}, r3.Vector{-4.446540323917821169175113, -1.722222222222222098864108, -2.432758535001817712384309}, r3.Vector{.096326883973217425349844, -2.277777777777777679091287, -3.333511567892824434267141},
		r3.Vector{-2.611893214736875634685020, -2.833333333333333037273860, 0.276212259283946492960382}, r3.Vector{-2.958086763130431950941102, -3.388888888888888839545643, -3.844737761481354709758307}, r3.Vector{-0.523998981458394741395068, -3.944444444444444197728217, -1.460966795333274337309604}, r3.Vector{-2, -4.500000000000000000000000, -2}}
	for i, v := range output {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst[i], 1e-2), test.ShouldBeTrue)
	}
}
