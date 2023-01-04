package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestBox(o Orientation, point, dims r3.Vector, label string) Geometry {
	box, _ := NewBox(NewPoseFromOrientation(point, o), dims, label)
	return box
}

func TestNewBox(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test box created from NewBox method
	geometry, err := NewBox(offset, r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &box{pose: offset, halfSize: [3]float64{0.5, 0.5, 0.5}, boundingSphereR: math.Sqrt(0.75)})
	_, err = NewBox(offset, r3.Vector{-1, 0, 0}, "")
	test.That(t, err.Error(), test.ShouldContainSubstring, newBadGeometryDimensionsError(&box{}).Error())

	// test box created from GeometryCreator with offset
	gc, err := NewBoxCreator(r3.Vector{1, 1, 1}, offset, "")
	test.That(t, err, test.ShouldBeNil)
	geometry = gc.NewGeometry(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestBoxAlmostEqual(t *testing.T) {
	original := makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{1, 1, 1}, "")
	good := makeTestBox(NewZeroOrientation(), r3.Vector{1e-16, 1e-16, 1e-16}, r3.Vector{1 + 1e-16, 1 + 1e-16, 1 + 1e-16}, "")
	bad := makeTestBox(NewZeroOrientation(), r3.Vector{1e-2, 1e-2, 1e-2}, r3.Vector{1 + 1e-2, 1 + 1e-2, 1 + 1e-2}, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestBoxVertices(t *testing.T) {
	offset := r3.Vector{2, 2, 2}
	box := makeTestBox(NewZeroOrientation(), offset, r3.Vector{2, 2, 2}, "")
	vertices := box.Vertices()
	test.That(t, R3VectorAlmostEqual(vertices[0], r3.Vector{1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[1], r3.Vector{1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[2], r3.Vector{1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[3], r3.Vector{1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[4], r3.Vector{-1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[5], r3.Vector{-1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[6], r3.Vector{-1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[7], r3.Vector{-1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
}

func TestBoxPC(t *testing.T) {
	offset1 := r3.Vector{2, 2, 0}
	dims1 := r3.Vector{2, 2, 2}
	eulerAngle1 := &EulerAngles{45, 45, 0}
	pose1 := NewPoseFromOrientation(offset1, eulerAngle1)
	box1 := &box{pose1, [3]float64{0.5 * dims1.X, 0.5 * dims1.Y, 0.5 * dims1.Z}, 10, ""} // with abitrary radius bounding sphere
	customDensity := 1.
	output1 := box1.ToPoints(customDensity)

	checkAgainst1 := []r3.Vector{
		{2.525321988817730733956068, 2.000000000000001332267630, -0.850903524534118327338206},
		{1.474678011182271264445376, 2.000000000000000888178420, 0.850903524534118549382811},
		{2.972320320618009770186063, 1.149096475465882560840214, -0.574940332598703474076274},
		{2.078323657017452141815284, 2.850903524534119881650440, -1.126866716469533624689348},
		{1.921676342982550300675371, 1.149096475465882782884819, 1.126866716469533624689348},
		{1.027679679381992006170776, 2.850903524534119881650440, 0.574940332598703696120879},
		{2.724036808064585812871883, 2.525321988817730733956068, 0.446998331800279036229995},
		{1.275963191935416185529562, 1.474678011182271486489981, -0.446998331800278925207692},
		{3.171035139864865737280297, 1.674418464283612628662468, 0.722961523735694333581137},
		{2.277038476264307220731098, 3.376225513351849727428089, 0.171035139864864182968063},
		{1.722961523735694999714951, 0.623774486648152826084868, -0.171035139864864044190185},
		{0.828964860135136816232659, 2.325581535716389591783582, -0.722961523735694111536532},
		{3.249358796882316102738741, 2.525321988817730733956068, -0.403905192733839402130513},
		{1.801285180753145809262605, 1.474678011182271264445376, -1.297901856334397363568200},
		{2.198714819246856411183444, 2.525321988817731178045278, 1.297901856334397585612805},
		{0.750641203117686117707308, 1.474678011182271486489981, 0.403905192733839513152816},
		{3.696357128682595138968736, 1.674418464283612628662468, -0.127942000798424243557250},
		{2.802360465082037066508747, 3.376225513351848839249669, -0.679868384669254366414748},
		{2.248283512553424845492600, 0.623774486648152715062565, -1.021938664398982510306269},
		{1.354286848952866773032611, 2.325581535716390035872791, -1.573865048269812660919342},
		{2.645713151047135447413439, 1.674418464283612184573258, 1.573865048269812660919342},
		{1.751716487446577152908844, 3.376225513351849283338879, 1.021938664398982510306269},
		{0.303642871317407081477313, 2.325581535716390035872791, 0.127942000798424410090703},
		{1.197639534917965153937303, 0.623774486648152715062565, 0.679868384669254477437050},
		{2.446998331800279924408414, 1.149096475465882782884819, 0.275963191935414964284234},
		{1.553001668199722073993030, 2.850903524534119881650440, -0.275963191935414853261932},
	}
	for i, v := range output1 {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst1[i], 1e-2), test.ShouldBeTrue)
	}

	// second check
	offset2 := r3.Vector{2, 2, 2}
	dims2 := r3.Vector{1, 1.5, 4}
	eulerAngle2 := &EulerAngles{0, 45, 0}
	pose2 := NewPoseFromOrientation(offset2, eulerAngle2)
	box2 := &box{pose2, [3]float64{0.5 * dims2.X, 0.5 * dims2.Y, 0.5 * dims2.Z}, 10, ""} // with abitrary radius bounding sphere
	output2 := box2.ToPoints(customDensity)

	checkAgainst2 := []r3.Vector{
		{2.262660994408865811067244, 2.000000000000000888178420, 1.574548237732941391442409},
		{1.737339005591135743244990, 2.000000000000000888178420, 2.425451762267059496736010},
		{3.113564518942984360450055, 2.000000000000000888178420, 2.099870226550671237220058},
		{1.411757469874747261684433, 2.000000000000000888178420, 1.049226248915211323620156},
		{2.588242530125254514672406, 2.000000000000000888178420, 2.950773751084789342513659},
		{0.886435481057017415906785, 2.000000000000000888178420, 1.900129773449329650958362},
		{3.964468043477102909832865, 2.000000000000000888178420, 2.625192215368401527086917},
		{0.560853945340628823323925, 2.000000000000000888178420, 0.523904260097481366820205},
		{3.439146054659373064055217, 2.000000000000000888178420, 3.476095739902519188291308},
		{0.035531956522898887340656, 2.000000000000000888178420, 1.374807784631600027225318},
		{2.000000000000000888178420, 2.750000000000000888178420, 2.000000000000000444089210},
		{2.000000000000000888178420, 1.250000000000000666133815, 2.000000000000000444089210},
		{2.850903524534118993472021, 2.750000000000000888178420, 2.525321988817730289866859},
		{1.149096475465882338795609, 2.750000000000000888178420, 1.474678011182270598311561},
		{2.850903524534118993472021, 1.250000000000000666133815, 2.525321988817730289866859},
		{1.149096475465882338795609, 1.250000000000000666133815, 1.474678011182270598311561},
		{3.701807049068237986944041, 2.750000000000000888178420, 3.050643977635460579733717},
		{0.298192950931763844923950, 2.750000000000000888178420, 0.949356022364540641511610},
		{3.701807049068237986944041, 1.250000000000000666133815, 3.050643977635460579733717},
		{0.298192950931763844923950, 1.250000000000000666133815, 0.949356022364540641511610},
		{3.701807049068237986944041, 2.000000000000000888178420, 3.050643977635460579733717},
		{0.298192950931763844923950, 2.000000000000000888178420, 0.949356022364540641511610},
	}
	for i, v := range output2 {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst2[i], 1e-2), test.ShouldBeTrue)
	}
}
