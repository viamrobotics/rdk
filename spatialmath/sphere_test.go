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
	offset := NewPose(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

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
	customDensity := 4.
	output := sphere.ToPoints(customDensity)
	checkAgainst := []r3.Vector{
		{-2, 0.5, -2},
		{-2.736728522874529279107492, 0.291666666666666518636930, -1.325096323560166267085947},
		{-1.879184030870393318224387, .083333333333333481363070, -3.376635653986012286509322},
		{-0.993888803664079834021550, -0.125, -0.687706107380094522341096},
		{-3.834905242938365876881335, -0.333333333333333037273860, -2.324568901251363506332837},
		{-.286685555024980498473042, -.541666666666666962726140, -3.089870405841368850019535},
		{-2.562059807441225522950390, -0.75, .090834468067506612953821},
		{-3.047479592544048543345525, -0.958333333333333481363070, -4.016858214837946583486428},
		{0.214001527812407665862793, -1.166666666666666518636930, -1.191450192999911728009010},
		{-4.237484340664094517592275, -1.375, -1.076398990211160811014679},
		{-0.955205532312214833368103, -1.583333333333333481363070, -4.232665091136126100934689},
		{-1.254392815159415697223722, -1.791666666666666518636930, 0.377113196323709320978423},
		{-4.163028024383074843228769, -2, -3.253518953081066200638816},
		{0.433196575524168459025987, -2.208333333333333037273860, -2.534931441476087510267234},
		{-3.417713161833014545720744, -2.416666666666666962726140, 0.016551085307368484933477},
		{-2.311074850875705077868361, -2.625, -4.400543154611609608650724},
		{-0.197705033618501113679145, -2.833333333333333037273860, -0.481023894292123221916313},
		{-4.270707638258272709208541, -3.041666666666666962726140, -1.906099110168808064003088},
		{-0.465339300658994758919107, -3.25, -3.527192370953369682240464},
		{-2.093795525847933713947668, -3.458333333333333037273860, 0.028414722935080849453016},
		{-3.193891001620486669310139, -3.666666666666666962726140, -3.430680431987469525267898},
		{-0.361173001190764564327651, -3.875, -1.779498145191667601849872},
		{-3.134366295729567930550274, -4.083333333333333037273860, -1.210737477555787755534311},
		{-1.780709235539242207835287, -4.291666666666666962726140, -2.974769434025864356385682},
		{-2, -4.5, -2},
	}
	for i, v := range output {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst[i], 1e-2), test.ShouldBeTrue)
	}
}
