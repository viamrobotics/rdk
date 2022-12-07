package referenceframe

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestStaticFrame(t *testing.T) {
	// define a static transform
	expPose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.R4AA{math.Pi / 2, 0., 0., 1.})
	frame, err := NewStaticFrame("test", expPose)
	test.That(t, err, test.ShouldBeNil)
	// get expected transform back
	emptyInput := FloatsToInputs([]float64{})
	pose, err := frame.Transform(emptyInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in non-empty input, should get err back
	nonEmptyInput := FloatsToInputs([]float64{0, 0, 0})
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
	input := FloatsToInputs([]float64{5})
	pose, err := frame.Transform(input)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostEqual(pose, expPose), test.ShouldBeTrue)

	// if you feed in too many inputs, should get an error back
	input = FloatsToInputs([]float64{0, 20, 0})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)

	// if you feed in empty input, should get an error
	input = FloatsToInputs([]float64{})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldNotBeNil)

	// if you try to move beyond set limits, should get an error
	overLimit := 50.0
	input = FloatsToInputs([]float64{overLimit})
	_, err = frame.Transform(input)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f %s %.5f", overLimit, OOBErrString, frame.DoF()[0]))

	// gets the correct limits back
	frameLimits := frame.DoF()
	test.That(t, frameLimits[0], test.ShouldResemble, limit)

	randomInputs := RandomFrameInputs(frame, nil)
	test.That(t, len(randomInputs), test.ShouldEqual, len(frame.DoF()))
	restrictRandomInputs := RestrictedRandomFrameInputs(frame, nil, 0.001)
	test.That(t, len(restrictRandomInputs), test.ShouldEqual, len(frame.DoF()))
	test.That(t, restrictRandomInputs[0].Value, test.ShouldBeLessThan, 0.03)
	test.That(t, restrictRandomInputs[0].Value, test.ShouldBeGreaterThan, -0.03)
}

func TestRevoluteFrame(t *testing.T) {
	axis := r3.Vector{1, 0, 0}                                                                // axis of rotation is x axis
	frame := &rotationalFrame{&baseFrame{"test", []Limit{{-math.Pi / 2, math.Pi / 2}}}, axis} // limits between -90 and 90 degrees
	// expected output
	expPose := spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.R4AA{math.Pi / 4, 1, 0, 0}) // 45 degrees
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
	test.That(t, err, test.ShouldBeError, errors.Errorf("%.5f %s %.5f", utils.DegToRad(overLimit), OOBErrString, frame.DoF()[0]))
	// gets the correct limits back
	limit := frame.DoF()
	expLimit := []Limit{{Min: -math.Pi / 2, Max: math.Pi / 2}}
	test.That(t, limit, test.ShouldHaveLength, 1)
	test.That(t, limit[0], test.ShouldResemble, expLimit[0])
}

func TestMobile2DFrame(t *testing.T) {
	expLimit := []Limit{{-10, 10}, {-10, 10}}
	frame := &mobile2DFrame{&baseFrame{"test", expLimit}, nil}
	// expected output
	expPose := spatial.NewPoseFromPoint(r3.Vector{3, 5, 0})
	// get expected transform back
	pose, err := frame.Transform(FloatsToInputs([]float64{3, 5}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	_, err = frame.Transform(FloatsToInputs([]float64{3, 5, 10}))
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in too few inputs, should get errr back
	_, err = frame.Transform(FloatsToInputs([]float64{3, 5, 10}))
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	_, err = frame.Transform(FloatsToInputs([]float64{3, 100}))
	test.That(t, err, test.ShouldNotBeNil)
	// gets the correct limits back
	limit := frame.DoF()
	test.That(t, limit[0], test.ShouldResemble, expLimit[0])
}

func TestGeometries(t *testing.T) {
	bc, err := spatial.NewBoxCreator(r3.Vector{1, 1, 1}, spatial.NewZeroPose(), "")
	test.That(t, err, test.ShouldBeNil)
	pose := spatial.NewPoseFromPoint(r3.Vector{0, 10, 0})
	expectedBox := bc.NewGeometry(pose)

	// test creating a new translational frame with a geometry
	tf, err := NewTranslationalFrameWithGeometry("", r3.Vector{0, 1, 0}, Limit{Min: -30, Max: 30}, bc)
	test.That(t, err, test.ShouldBeNil)
	geometries, err := tf.Geometries(FloatsToInputs([]float64{10}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, expectedBox.AlmostEqual(geometries.Geometries()[""]), test.ShouldBeTrue)

	// test erroring correctly from trying to create a geometry for a rotational frame
	rf, err := NewRotationalFrame("", spatial.R4AA{3.7, 2.1, 3.1, 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)
	geometries, err = rf.Geometries([]Input{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, geometries, test.ShouldBeNil)

	// test creating a new mobile frame with a geometry
	mf, err := NewMobile2DFrame("", []Limit{{-10, 10}, {-10, 10}}, bc)
	test.That(t, err, test.ShouldBeNil)
	geometries, err = mf.Geometries(FloatsToInputs([]float64{0, 10}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, expectedBox.AlmostEqual(geometries.Geometries()[""]), test.ShouldBeTrue)

	// test creating a new static frame with a geometry
	expectedBox = bc.NewGeometry(spatial.NewZeroPose())
	sf, err := NewStaticFrameWithGeometry("", pose, bc)
	test.That(t, err, test.ShouldBeNil)
	geometries, err = sf.Geometries([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, expectedBox.AlmostEqual(geometries.Geometries()[""]), test.ShouldBeTrue)

	// test inheriting a geometry creator
	sf, err = NewStaticFrameFromFrame(tf, pose)
	test.That(t, err, test.ShouldBeNil)
	geometries, err = sf.Geometries([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, expectedBox.AlmostEqual(geometries.Geometries()[""]), test.ShouldBeTrue)
}

func TestSerializationStatic(t *testing.T) {
	f, err := NewStaticFrame("foo", spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.R4AA{math.Pi / 2, 4, 5, 6}))
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
}

func TestSerializationTranslation(t *testing.T) {
	f, err := NewTranslationalFrame("foo", r3.Vector{1, 0, 0}, Limit{1, 2})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
	test.That(t, f2, test.ShouldResemble, f)
}

func TestSerializationRotations(t *testing.T) {
	f, err := NewRotationalFrame("foo", spatial.R4AA{3.7, 2.1, 3.1, 4.1}, Limit{5, 6})
	test.That(t, err, test.ShouldBeNil)

	data, err := f.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	f2, err := UnmarshalFrameJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, f.AlmostEquals(f2), test.ShouldBeTrue)
	test.That(t, f2, test.ShouldResemble, f)
}

func TestRandomFrameInputs(t *testing.T) {
	frame, _ := NewMobile2DFrame("", []Limit{{-10, 10}, {-10, 10}}, nil)
	seed := rand.New(rand.NewSource(23))
	for i := 0; i < 100; i++ {
		_, err := frame.Transform(RandomFrameInputs(frame, seed))
		test.That(t, err, test.ShouldBeNil)
	}

	limitedFrame, _ := NewMobile2DFrame("", []Limit{{-2, 2}, {-2, 2}}, nil)
	for i := 0; i < 100; i++ {
		_, err := limitedFrame.Transform(RestrictedRandomFrameInputs(frame, seed, .2))
		test.That(t, err, test.ShouldBeNil)
	}
}


func TestFrame(t *testing.T) {
	file, err := os.Open("data/frame.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := io.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)

	frame := Frame{}
	err = json.Unmarshal(testMap["test"], &frame)
	test.That(t, err, test.ShouldBeNil)
	bc, err := spatial.NewBoxCreator(r3.Vector{1, 2, 3}, spatial.NewPoseFromPoint(r3.Vector{4, 5, 6}), "")
	test.That(t, err, test.ShouldBeNil)
	exp := Frame{
		Parent:      "world",
		Translation: r3.Vector{1, 2, 3},
		Orientation: &spatial.OrientationVectorDegrees{Theta: 85, OZ: 1},
		Geometry:    bc,
	}
	test.That(t, frame, test.ShouldResemble, exp)

	// test going back to json and validating.
	rd, err := json.Marshal(&frame)
	test.That(t, err, test.ShouldBeNil)
	frame2 := Frame{}
	err = json.Unmarshal(rd, &frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame2, test.ShouldResemble, exp)

	pose := frame.Pose()
	expPose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, exp.Orientation)
	test.That(t, pose, test.ShouldResemble, expPose)

	staticFrame, err := frame.StaticFrame("test")
	test.That(t, err, test.ShouldBeNil)
	expStaticFrame, err := NewStaticFrameWithGeometry("test", expPose, bc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, staticFrame, test.ShouldResemble, expStaticFrame)
}

func TestMergeFrameSystems(t *testing.T) {
	blankPos := map[string][]Input{}
	// build 2 frame systems
	fs1 := NewEmptySimpleFrameSystem("test1")
	fs2 := NewEmptySimpleFrameSystem("test2")

	frame1, err := NewStaticFrame("frame1", spatial.NewPoseFromPoint(r3.Vector{-5, 5, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame1, fs1.World())
	test.That(t, err, test.ShouldBeNil)
	frame2, err := NewStaticFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{0, 0, 10}))
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame2, fs1.Frame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// frame3 - pure translation
	frame3, err := NewStaticFrame("frame3", spatial.NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs2.AddFrame(frame3, fs2.World())
	test.That(t, err, test.ShouldBeNil)
	// frame4 - pure rotiation around y 90 degrees
	frame4, err := NewStaticFrame(
		"frame4",
		spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.R4AA{math.Pi / 2, 0., 1., 0.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs2.AddFrame(frame4, fs2.Frame("frame3"))
	test.That(t, err, test.ShouldBeNil)

	// merge to fs1 with zero offset
	err = MergeFrameSystems(fs1, fs2, nil)
	test.That(t, err, test.ShouldBeNil)
	poseStart := spatial.NewZeroPose()                         // PoV from frame 2
	poseEnd := spatial.NewPoseFromPoint(r3.Vector{-9, -2, -3}) // PoV from frame 4
	transformPoint, err := fs1.Transform(blankPos, NewPoseInFrame("frame2", poseStart), "frame4")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)

	// reset fs1 framesystem to original
	fs1 = NewEmptySimpleFrameSystem("test1")
	err = fs1.AddFrame(frame1, fs1.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame2, fs1.Frame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// merge to fs1 with an offset and rotation
	offsetConfig := &Frame{
		Parent: "frame1", Translation: r3.Vector{1, 2, 3},
		Orientation: &spatial.R4AA{Theta: math.Pi / 2, RZ: 1.},
	}
	err = MergeFrameSystems(fs1, fs2, offsetConfig)
	test.That(t, err, test.ShouldBeNil)
	// the frame of test2_world is rotated around z by 90 degrees, then displaced by (1,2,3) in the frame of frame1,
	// so the origin of frame1 from the perspective of test2_frame should be (-2, 1, -3)
	poseStart = spatial.NewZeroPose()                        // PoV from frame 1
	poseEnd = spatial.NewPoseFromPoint(r3.Vector{-2, 1, -3}) // PoV from the world of test2
	transformPoint, err = fs1.Transform(blankPos, NewPoseInFrame("frame1", poseStart), "test2_world")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)

	// frame frame 2 to frame 4
	poseStart = spatial.NewZeroPose()                        // PoV from frame 2
	poseEnd = spatial.NewPoseFromPoint(r3.Vector{-6, -6, 0}) // PoV from frame 4
	transformPoint, err = fs1.Transform(blankPos, NewPoseInFrame("frame2", poseStart), "frame4")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)
}
