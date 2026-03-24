package referenceframe

import (
	"encoding/json"
	"io"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils"

	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const defaultFloatPrecision = 1e-8

func TestStaticFrame(t *testing.T) {
	// define a static transform
	expPose := spatial.NewPose(r3.Vector{1, 2, 3}, &spatial.R4AA{math.Pi / 2, 0., 0., 1.})
	frame, err := NewStaticFrame("test", expPose)
	test.That(t, err, test.ShouldBeNil)
	// get expected transform back
	emptyInput := []Input{}
	pose, err := frame.Transform(emptyInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in non-empty input, should get err back
	nonEmptyInput := []Input{0, 0, 0}
	_, err = frame.Transform(nonEmptyInput)
	test.That(t, err, test.ShouldNotBeNil)
	// check that there are no limits on the static frame
	limits := frame.DoF()
	test.That(t, limits, test.ShouldResemble, []Limit{})

	errExpect := errors.New("pose is not allowed to be nil")
	f, err := NewStaticFrame("test2", nil)
	test.That(t, err.Error(), test.ShouldEqual, errExpect.Error())
	test.That(t, f, test.ShouldBeNil)
}

func TestPrismaticFrame(t *testing.T) {
	// define a prismatic transform
	limit := Limit{Min: -30, Max: 30}
	frame, err := NewTranslationalFrame("test", r3.Vector{3, 4, 0}, limit)
	test.That(t, err, test.ShouldBeNil)

	// get expected transform back
	expPose := spatial.NewPoseFromPoint(r3.Vector{3, 4, 0})
	input := []Input{5}
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostEqual(pose, expPose), test.ShouldBeTrue)

	// if you feed in too many inputs, should get an error back
	input = []Input{0, 20, 0}
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)

	// if you feed in empty input, should get an error
	input = []Input{}
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)

	// if you try to move beyond set limits, should get an error
	overLimit := 50.0
	input = []Input{overLimit}
	_, err = frame.Transform(input)
	s := "joint 0 input out of bounds, input 50.00000 needs to be within range [30.00000 -30.00000]"
	test.That(t, err.Error(), test.ShouldEqual, s)

	// gets the correct limits back
	frameLimits := frame.DoF()
	test.That(t, frameLimits[0], test.ShouldResemble, limit)

	randomInputs := RandomFrameInputs(frame, nil)
	test.That(t, len(randomInputs), test.ShouldEqual, len(frame.DoF()))

	for i := 0; i < 10; i++ {
		restrictRandomInputs, err := RestrictedRandomFrameInputs(frame, nil, 0.001, []Input{-10})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(restrictRandomInputs), test.ShouldEqual, len(frame.DoF()))
		test.That(t, restrictRandomInputs[0], test.ShouldBeLessThan, -9.07)
		test.That(t, restrictRandomInputs[0], test.ShouldBeGreaterThan, -10.03)
	}
}

func TestRevoluteFrame(t *testing.T) {
	axis := r3.Vector{1, 0, 0}                                                                // axis of rotation is x axis
	frame := &rotationalFrame{&baseFrame{"test", []Limit{{-math.Pi / 2, math.Pi / 2}}}, axis} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromOrientation(&spatial.R4AA{math.Pi / 4, 1, 0, 0}) // 45 degrees
	// get expected transform back
	input := frame.InputFromProtobuf(&pb.JointPositions{Values: []float64{45}})
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	input = frame.InputFromProtobuf(&pb.JointPositions{Values: []float64{45, 55}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in empty input, should get errr back
	input = frame.InputFromProtobuf(&pb.JointPositions{Values: []float64{}})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	overLimit := 100.0 // degrees
	input = frame.InputFromProtobuf(&pb.JointPositions{Values: []float64{overLimit}})
	_, err = frame.Transform(input)
	s := "joint 0 input out of bounds, input 1.74533 needs to be within range [1.57080 -1.57080]"
	test.That(t, err.Error(), test.ShouldEqual, s)
	// gets the correct limits back
	limit := frame.DoF()
	expLimit := []Limit{{Min: -math.Pi / 2, Max: math.Pi / 2}}
	test.That(t, limit, test.ShouldHaveLength, 1)
	test.That(t, limit[0], test.ShouldResemble, expLimit[0])
}

func TestGeometries(t *testing.T) {
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	pose := spatial.NewPoseFromPoint(r3.Vector{0, 10, 0})
	expectedBox := bc.Transform(pose)

	// test creating a new translational frame with a geometry
	tf, err := NewTranslationalFrameWithGeometry("", r3.Vector{0, 1, 0}, Limit{Min: -30, Max: 30}, bc)
	test.That(t, err, test.ShouldBeNil)
	geometries, err := tf.Geometries([]Input{10})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.GeometriesAlmostEqual(expectedBox, geometries.Geometries()[0]), test.ShouldBeTrue)

	// test erroring correctly from trying to create a geometry for a rotational frame
	rf, err := NewRotationalFrame("", spatial.R4AA{3.7, 2.1, 3.1, 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)
	geometries, err = rf.Geometries([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(geometries.Geometries()), test.ShouldEqual, 0)

	// test creating a new static frame with a geometry
	expectedBox = bc.Transform(spatial.NewZeroPose())
	sf, err := NewStaticFrameWithGeometry("", pose, bc)
	test.That(t, err, test.ShouldBeNil)
	geometries, err = sf.Geometries([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.GeometriesAlmostEqual(expectedBox, geometries.Geometries()[0]), test.ShouldBeTrue)
}

func TestSerializationStatic(t *testing.T) {
	f, err := NewStaticFrame("foo", spatial.NewPose(r3.Vector{1, 2, 3}, &spatial.R4AA{math.Pi / 2, 4, 5, 6}))
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2Cfg := &LinkConfig{}
	err = json.Unmarshal(data, f2Cfg)
	test.That(t, err, test.ShouldBeNil)

	f2if, err := f2Cfg.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	f2, err := f2if.ToStaticFrame("")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f2.Name(), test.ShouldResemble, f.Name())
	p1, err := f.Transform(nil)
	test.That(t, err, test.ShouldBeNil)
	p2, err := f2.Transform(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostEqual(p1, p2), test.ShouldBeTrue)
}

func TestSerializationTranslation(t *testing.T) {
	f, err := NewTranslationalFrame("foo", r3.Vector{1, 0, 0}, Limit{1, 2})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2Cfg := &JointConfig{}
	err = json.Unmarshal(data, f2Cfg)
	test.That(t, err, test.ShouldBeNil)

	f2, err := f2Cfg.ToFrame()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f2, test.ShouldResemble, f)
}

func TestSerializationRotations(t *testing.T) {
	f, err := NewRotationalFrame("foo", spatial.R4AA{3.7, 2.1, 3.1, 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2Cfg := &JointConfig{}
	err = json.Unmarshal(data, f2Cfg)
	test.That(t, err, test.ShouldBeNil)

	f2, err := f2Cfg.ToFrame()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f2, test.ShouldResemble, f)
}

func TestRandomFrameInputs(t *testing.T) {
	frame, _ := NewTranslationalFrame("", r3.Vector{X: 1}, Limit{-10, 10})
	seed := rand.New(rand.NewSource(23))
	for i := 0; i < 100; i++ {
		_, err := frame.Transform(RandomFrameInputs(frame, seed))
		test.That(t, err, test.ShouldBeNil)
	}

	limitedFrame, _ := NewTranslationalFrame("", r3.Vector{X: 1}, Limit{-2, 2})
	for i := 0; i < 100; i++ {
		r, err := RestrictedRandomFrameInputs(frame, seed, .2, []Input{0})
		test.That(t, err, test.ShouldBeNil)
		_, err = limitedFrame.Transform(r)
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestFrame(t *testing.T) {
	file, err := os.Open("../config/data/frame.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := io.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)

	frame := LinkConfig{}
	err = json.Unmarshal(testMap["test"], &frame)
	test.That(t, err, test.ShouldBeNil)
	bc, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{4, 5, 6}), r3.Vector{1, 2, 3}, "")
	test.That(t, err, test.ShouldBeNil)
	pose := spatial.NewPose(r3.Vector{1, 2, 3}, &spatial.OrientationVectorDegrees{Theta: 85, OZ: 1})
	expFrame, err := NewStaticFrameWithGeometry("", pose, bc)
	test.That(t, err, test.ShouldBeNil)
	sFrameif, err := frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	sFrame, err := sFrameif.ToStaticFrame("")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sFrame, test.ShouldResemble, expFrame)

	// test going back to json and validating.
	rd, err := json.Marshal(&frame)
	test.That(t, err, test.ShouldBeNil)
	frame2 := LinkConfig{}
	err = json.Unmarshal(rd, &frame2)
	test.That(t, err, test.ShouldBeNil)

	sFrame2if, err := frame2.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	sFrame2, err := sFrame2if.ToStaticFrame("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sFrame2, test.ShouldResemble, expFrame)

	pose, err = frame.Pose()
	test.That(t, err, test.ShouldBeNil)
	expPose, err := expFrame.Transform([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)

	bc.SetLabel("test")
	sFrameif, err = frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	sFrame, err = sFrameif.ToStaticFrame("test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	expStaticFrame, err := NewStaticFrameWithGeometry("test", expPose, bc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sFrame, test.ShouldResemble, expStaticFrame)
}

func TestFrameToJSONAndBack(t *testing.T) {
	// static frame
	static, err := NewStaticFrame("foo", spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.R4AA{Theta: math.Pi / 2, RX: 4, RY: 5, RZ: 6}))
	test.That(t, err, test.ShouldBeNil)

	jsonData, err := frameToJSON(static)
	test.That(t, err, test.ShouldBeNil)

	static2, err := jsonToFrame(json.RawMessage(jsonData))
	test.That(t, err, test.ShouldBeNil)

	eq, err := framesAlmostEqual(static, static2, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	staticFrame, ok := static.(*staticFrame)
	test.That(t, ok, test.ShouldBeTrue)
	tailGeoFrame := tailGeometryStaticFrame{staticFrame: staticFrame}

	jsonData, err = frameToJSON(&tailGeoFrame)
	test.That(t, err, test.ShouldBeNil)

	tailGeoFrameParsed, err := jsonToFrame(json.RawMessage(jsonData))
	test.That(t, err, test.ShouldBeNil)

	eq, err = framesAlmostEqual(&tailGeoFrame, tailGeoFrameParsed, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	// translational frame
	tF, err := NewTranslationalFrame("foo", r3.Vector{X: 1, Y: 0, Z: 0}, Limit{1, 2})
	test.That(t, err, test.ShouldBeNil)

	jsonData, err = frameToJSON(tF)
	test.That(t, err, test.ShouldBeNil)

	tF2, err := jsonToFrame(json.RawMessage(jsonData))
	test.That(t, err, test.ShouldBeNil)

	eq, err = framesAlmostEqual(tF, tF2, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	// rotational frame
	rot, err := NewRotationalFrame("foo", spatial.R4AA{Theta: 3.7, RX: 2.1, RY: 3.1, RZ: 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)

	jsonData, err = frameToJSON(rot)
	test.That(t, err, test.ShouldBeNil)

	rot2, err := jsonToFrame(json.RawMessage(jsonData))
	test.That(t, err, test.ShouldBeNil)

	eq, err = framesAlmostEqual(rot, rot2, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	// SimpleModel
	simpleModel, err := ParseModelJSONFile(rdkutils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	jsonData, err = frameToJSON(simpleModel)
	test.That(t, err, test.ShouldBeNil)

	simpleModel2, err := jsonToFrame(json.RawMessage(jsonData))
	test.That(t, err, test.ShouldBeNil)

	eq, err = framesAlmostEqual(simpleModel, simpleModel2, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)
}

func TestLimitMethods(t *testing.T) {
	// test basic
	l := Limit{0, 10}
	a, b, c := l.GoodLimits()
	test.That(t, a, test.ShouldEqual, 0)
	test.That(t, b, test.ShouldEqual, 10)
	test.That(t, c, test.ShouldEqual, 10)

	d := l.Jog(5, .25)
	test.That(t, d, test.ShouldEqual, 7.5)

	// test rotational magic

	test.That(t, isMultipleOfPi(0), test.ShouldBeTrue)
	test.That(t, isMultipleOfPi(-2*math.Pi), test.ShouldBeTrue)
	test.That(t, isMultipleOfPi(math.Pi), test.ShouldBeTrue)
	test.That(t, isMultipleOfPi(4*math.Pi), test.ShouldBeTrue)
	test.That(t, isMultipleOfPi(.1), test.ShouldBeFalse)
	test.That(t, isMultipleOfPi(10), test.ShouldBeFalse)

	l = Limit{-4 * math.Pi, 4 * math.Pi}
	_, _, c = l.GoodLimits()
	test.That(t, c, test.ShouldEqual, 8*math.Pi)

	d = l.Jog(0, .25)
	test.That(t, d, test.ShouldEqual, math.Pi/2)

	l = Limit{-6.265732014659642, 6.26573201465964} // from xarm
	d = l.Jog(0, .25)
	test.That(t, d, test.ShouldEqual, math.Pi/2)

	// sanity
	l = Limit{math.Inf(-1), math.Inf(1)}
	d = l.Jog(0, .25)
	test.That(t, d, test.ShouldAlmostEqual, rangeLimit/2, 1)
}
