// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// OOBErrString is a string that all OOB errors should contain, so that they can be checked for distinct from other Transform errors.
const OOBErrString = "input out of bounds"

// Limit represents the limits of motion for a Frame.
type Limit struct {
	Min float64
	Max float64
}

// Range gives the range of the limit.
func (l *Limit) Range() float64 {
	return l.Max - l.Min
}

// Hash returns a hash value for this limit.
func (l *Limit) Hash() int {
	hash := 0
	hash += (5 * (int(l.Min*100) + 1000)) * 2
	hash += (6 * (int(l.Max*100) + 2000)) * 3
	return hash
}

const rangeLimit = 999

// GoodLimits gives min, max, range, but capped to -999,999.
func (l *Limit) GoodLimits() (float64, float64, float64) {
	a := l.Min
	b := l.Max
	if a < -1*rangeLimit {
		a = -1 * rangeLimit
	}
	if b > rangeLimit {
		b = rangeLimit
	}
	return a, b, b - a
}

// IsValid return if the value is in the range
func (l *Limit) IsValid(v float64) bool {
	return v >= l.Min && v <= l.Max
}

// AreInputsValid checks if all values are within limit ranges
func AreInputsValid(ls []Limit, values []float64) bool {
	if len(ls) != len(values) {
		return false
	}

	for i, l := range ls {
		if !l.IsValid(values[i]) {
			return false
		}
	}

	return true
}

func limitsAlmostEqual(limits1, limits2 []Limit, epsilon float64) bool {
	if len(limits1) != len(limits2) {
		return false
	}
	for i := range limits1 {
		if math.Abs(limits1[i].Max-limits2[i].Max) > epsilon || math.Abs(limits1[i].Min-limits2[i].Min) > epsilon {
			return false
		}
	}
	return true
}

// RestrictedRandomFrameInputs will produce a list of valid, in-bounds inputs for the frame.
// The range of selection is restricted to `restrictionPercent` percent of the limits, and the
// selection frame is centered at reference.
func RestrictedRandomFrameInputs(m Frame, rSeed *rand.Rand, restrictionPercent float64, reference []Input) ([]Input, error) {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	if len(reference) != len(dof) {
		return nil, NewIncorrectDoFError(len(reference), len(dof))
	}
	pos := make([]Input, 0, len(dof))
	for i, limit := range dof {
		l, u := limit.Min, limit.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		frameSpan := u - l
		minVal := math.Max(l, reference[i]-restrictionPercent*frameSpan/2)
		maxVal := math.Min(u, reference[i]+restrictionPercent*frameSpan/2)
		samplingSpan := maxVal - minVal
		pos = append(pos, samplingSpan*rSeed.Float64()+minVal)
	}
	return pos, nil
}

// RandomFrameInputs will produce a list of valid, in-bounds inputs for the Frame.
func RandomFrameInputs(m Frame, rSeed *rand.Rand) []Input {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	pos := make([]Input, 0, len(dof))
	for _, lim := range dof {
		l, u := lim.Min, lim.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}
		pos = append(pos, rSeed.Float64()*(u-l)+l)
	}
	return pos
}

// Limited represents anything that has Limits.
type Limited interface {
	// DoF will return a slice with length equal to the number of degrees of freedom.  Each element
	// describes the min and max movement limit of that degree of freedom.  For robot parts that
	// don't move, it returns an empty slice.
	DoF() []Limit
}

// Frame represents a reference frame, e.g. an arm, a joint, a gripper, a board, etc.
type Frame interface {
	Limited
	// Name returns the name of the Frame
	Name() string

	Hash() int

	// Transform is the pose (rotation and translation) that goes FROM current frame TO parent's
	// reference frame.
	//
	// If the transform cannot be computed (including if inputs are out of bounds), the returned
	// pose will be nil and the error will not be nil.
	Transform([]Input) (spatial.Pose, error)

	// Interpolate interpolates the given amount between the two sets of inputs.
	Interpolate([]Input, []Input, float64) ([]Input, error)

	// Geometries returns a map between names and geometries for the reference frame and any
	// intermediate frames that may be defined for it, e.g. links in an arm. If a frame does not
	// have a geometry it will not be added into the map
	Geometries([]Input) (*GeometriesInFrame, error)

	// InputFromProtobuf does there correct thing for this frame to convert protobuf units
	// (degrees/mm) to input units (radians/mm)
	InputFromProtobuf(*pb.JointPositions) []Input

	// ProtobufFromInput does there correct thing for this frame to convert input units (radians/mm)
	// to protobuf units (degrees/mm)
	ProtobufFromInput([]Input) *pb.JointPositions

	json.Marshaler
	json.Unmarshaler
}

// baseFrame contains all the data and methods common to all frames, notably it does not implement
// the Frame interface itself.
type baseFrame struct {
	name   string
	limits []Limit
}

func (bf *baseFrame) hash() int {
	h := 0
	h += 10 * len(bf.limits)

	for i, l := range bf.limits {
		h += (i + 7) + (((i + 9) * 11) * l.Hash())
	}

	return h
}

// Name returns the name of the Frame.
func (bf *baseFrame) Name() string {
	return bf.name
}

// DoF will return a slice with length equal to the number of joints/degrees of freedom.
func (bf *baseFrame) DoF() []Limit {
	return bf.limits
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (bf *baseFrame) Interpolate(from, to []Input, by float64) ([]Input, error) {
	err := bf.validInputs(from)
	if err != nil {
		return nil, err
	}
	err = bf.validInputs(to)
	if err != nil {
		return nil, err
	}
	return interpolateInputs(from, to, by), nil
}

// validInputs checks whether the given array of joint positions violates any joint limits.
func (bf *baseFrame) validInputs(inputs []Input) error {
	var errAll error
	if len(inputs) != len(bf.limits) {
		return NewIncorrectDoFError(len(inputs), len(bf.limits))
	}

	for i := 0; i < len(bf.limits); i++ {
		if inputs[i] < bf.limits[i].Min || inputs[i] > bf.limits[i].Max {
			lim := []float64{bf.limits[i].Max, bf.limits[i].Min}
			multierr.AppendInto(&errAll, fmt.Errorf("%s %s %s, %s %.5f %s %.5f", "joint", fmt.Sprint(i),
				OOBErrString, "input", inputs[i], "needs to be within range", lim))
		}
	}

	return errAll
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation
// from the current Frame to the parent's reference frame.
type staticFrame struct {
	*baseFrame
	transform spatial.Pose
	geometry  spatial.Geometry
}

// a tailGeometryStaticFrame is a static frame whose geometry is placed at the end of the frame's transform, rather than at the beginning.
type tailGeometryStaticFrame struct {
	*staticFrame
}

func (sf *tailGeometryStaticFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if sf.geometry == nil {
		return NewGeometriesInFrame(sf.Name(), nil), nil
	}
	if len(input) != 0 {
		return nil, NewIncorrectDoFError(len(input), 0)
	}
	newGeom := sf.geometry.Transform(sf.transform)
	if newGeom.Label() == "" {
		newGeom.SetLabel(sf.name)
	}

	// Create the new geometry at a pose of `transform` from the frame
	return NewGeometriesInFrame(sf.name, []spatial.Geometry{newGeom}), nil
}

func (sf *tailGeometryStaticFrame) UnmarshalJSON(data []byte) error {
	var inner staticFrame

	err := json.Unmarshal(data, &inner)
	if err != nil {
		return err
	}
	sf.staticFrame = &inner
	return nil
}

// Hash returns a hash value for this tail geometry static frame.
func (sf *tailGeometryStaticFrame) Hash() int {
	return sf.staticFrame.Hash() + 7 // Add distinguishing factor for tail geometry placement
}

// namedFrame is used to change the name of a frame.
type namedFrame struct {
	Frame
	name string
}

// Name returns the name of the namedFrame.
func (nf *namedFrame) Name() string {
	return nf.name
}

func (nf *namedFrame) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	gif, err := nf.Frame.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return NewGeometriesInFrame(nf.name, gif.geometries), nil
}

// Hash returns a hash value for this named frame.
func (nf *namedFrame) Hash() int {
	return nf.Frame.Hash() + hashString(nf.name)
}

// NewNamedFrame will return a frame which has a new name but otherwise passes through all functions of the original frame.
func NewNamedFrame(frame Frame, name string) Frame {
	return &namedFrame{Frame: frame, name: name}
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is not allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{&baseFrame{name, []Limit{}}, pose, nil}, nil
}

// NewZeroStaticFrame creates a frame with no translation or orientation changes.
func NewZeroStaticFrame(name string) Frame {
	return &staticFrame{&baseFrame{name, []Limit{}}, spatial.NewZeroPose(), nil}
}

// NewStaticFrameWithGeometry creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It also has an associated geometry representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewStaticFrameWithGeometry(name string, pose spatial.Pose, geometry spatial.Geometry) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{&baseFrame{name, []Limit{}}, pose, geometry}, nil
}

func (sf *staticFrame) Hash() int {
	h := sf.hash()
	if sf.transform != nil {
		h += (123 * spatial.HashPose(sf.transform))
	}
	if sf.geometry != nil {
		h += +(111 * sf.geometry.Hash())
	}
	return h
}

// Transform returns the pose associated with this static Frame.
func (sf *staticFrame) Transform(input []Input) (spatial.Pose, error) {
	if len(input) != 0 {
		return nil, NewIncorrectDoFError(len(input), 0)
	}
	return sf.transform, nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (sf *staticFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	return []Input{}
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (sf *staticFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	return &pb.JointPositions{}
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (sf *staticFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if sf.geometry == nil {
		return NewGeometriesInFrame(sf.Name(), nil), nil
	}
	if len(input) != 0 {
		return nil, NewIncorrectDoFError(len(input), 0)
	}
	newGeom := sf.geometry.Transform(spatial.NewZeroPose())
	if newGeom.Label() == "" {
		newGeom.SetLabel(sf.name)
	}
	return NewGeometriesInFrame(sf.name, []spatial.Geometry{newGeom}), nil
}

func (sf staticFrame) MarshalJSON() ([]byte, error) {
	temp := LinkConfig{
		ID:          sf.name,
		Translation: sf.transform.Point(),
	}

	orientationConfig, err := spatial.NewOrientationConfig(sf.transform.Orientation())
	if err != nil {
		return nil, err
	}
	temp.Orientation = orientationConfig

	if sf.geometry != nil {
		temp.Geometry, err = spatial.NewGeometryConfig(sf.geometry)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(temp)
}

func (sf *staticFrame) UnmarshalJSON(data []byte) error {
	var lc LinkConfig
	if err := json.Unmarshal(data, &lc); err != nil {
		return err
	}

	var transform spatial.Pose
	var geometry spatial.Geometry
	if lc.Orientation != nil {
		orientation, err := lc.Orientation.ParseConfig()
		if err != nil {
			return err
		}
		transform = spatial.NewPose(lc.Translation, orientation)
	} else {
		transform = spatial.NewPose(lc.Translation, nil)
	}
	if lc.Geometry != nil {
		geo, err := lc.Geometry.ParseConfig()
		if err != nil {
			return err
		}
		geometry = geo
	}
	sf.baseFrame = &baseFrame{name: lc.ID, limits: []Limit{}}
	sf.transform = transform
	sf.geometry = geometry
	return nil
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions.
type translationalFrame struct {
	*baseFrame
	transAxis r3.Vector
	geometry  spatial.Geometry
}

// NewTranslationalFrame creates a frame given a name and the axis in which to translate.
func NewTranslationalFrame(name string, axis r3.Vector, limit Limit) (Frame, error) {
	return NewTranslationalFrameWithGeometry(name, axis, limit, nil)
}

// NewTranslationalFrameWithGeometry creates a frame given a given a name and the axis in which to translate.
// It also has an associated geometry representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewTranslationalFrameWithGeometry(name string, axis r3.Vector, limit Limit, geometry spatial.Geometry) (Frame, error) {
	if spatial.R3VectorAlmostEqual(r3.Vector{}, axis, 1e-8) {
		return nil, errors.New("cannot use zero vector as translation axis")
	}
	return &translationalFrame{
		baseFrame: &baseFrame{name: name, limits: []Limit{limit}},
		transAxis: axis.Normalize(),
		geometry:  geometry,
	}, nil
}

func (pf *translationalFrame) Hash() int {
	h := pf.hash()
	h += int(10000 * pf.transAxis.Norm())
	if pf.geometry != nil {
		h += pf.geometry.Hash()
	}
	return h
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	if err := pf.validInputs(input); err != nil {
		return nil, err
	}
	return spatial.NewPoseFromPoint(pf.transAxis.Mul(input[0])), nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (pf *translationalFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	return jp.Values
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (pf *translationalFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	return &pb.JointPositions{Values: input}
}

// Geometries returns an object representing the 3D space associeted with the translationalFrame.
func (pf *translationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if pf.geometry == nil {
		return NewGeometriesInFrame(pf.Name(), nil), nil
	}
	pose, err := pf.Transform(input)
	if err != nil {
		return nil, err
	}
	return NewGeometriesInFrame(pf.name, []spatial.Geometry{pf.geometry.Transform(pose)}), nil
}

func (pf translationalFrame) MarshalJSON() ([]byte, error) {
	if len(pf.limits) > 1 {
		return nil, ErrMarshalingHighDOFFrame
	}
	temp := JointConfig{
		ID:   pf.name,
		Type: PrismaticJoint,
		Axis: spatial.AxisConfig{pf.transAxis.X, pf.transAxis.Y, pf.transAxis.Z},
		Max:  pf.limits[0].Max,
		Min:  pf.limits[0].Min,
	}
	if pf.geometry != nil {
		var err error
		temp.Geometry, err = spatial.NewGeometryConfig(pf.geometry)
		if err != nil {
			return nil, err
		}
	}

	return json.Marshal(temp)
}

func (pf *translationalFrame) UnmarshalJSON(data []byte) error {
	var cfg JointConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	pf.baseFrame = &baseFrame{name: cfg.ID, limits: []Limit{{Min: cfg.Min, Max: cfg.Max}}}
	pf.transAxis = r3.Vector(cfg.Axis).Normalize()
	if cfg.Geometry != nil {
		geometry, err := cfg.Geometry.ParseConfig()
		if err != nil {
			return err
		}
		pf.geometry = geometry
	}
	return nil
}

type rotationalFrame struct {
	*baseFrame
	rotAxis r3.Vector
}

// NewRotationalFrame creates a new rotationalFrame struct.
// A standard revolute joint will have 1 DoF.
func NewRotationalFrame(name string, axis spatial.R4AA, limit Limit) (Frame, error) {
	axis.Normalize()
	return &rotationalFrame{
		baseFrame: &baseFrame{name: name, limits: []Limit{limit}},
		rotAxis:   r3.Vector{axis.RX, axis.RY, axis.RZ},
	}, nil
}

func (rf *rotationalFrame) Hash() int {
	return rf.hash() + int(1000*rf.rotAxis.Norm())
}

// Transform returns the Pose representing the frame's 6DoF motion in space. Requires a slice
// of inputs that has length equal to the degrees of freedom of the Frame.
func (rf *rotationalFrame) Transform(input []Input) (spatial.Pose, error) {
	if err := rf.validInputs(input); err != nil {
		return nil, err
	}
	// Create a copy of the r4aa for thread safety
	return spatial.NewPoseFromOrientation(&spatial.R4AA{input[0], rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z}), nil
}

func (rf *rotationalFrame) InputToOrientation(input Input) spatial.R4AA {
	return spatial.R4AA{input, rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z}
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (rf *rotationalFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (rf *rotationalFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = utils.RadToDeg(a)
	}
	return &pb.JointPositions{Values: n}
}

// Geometries will always return (nil, nil) for rotationalFrames, as not allowing rotationalFrames to occupy geometries is a
// design choice made for simplicity. staticFrame and translationalFrame should be used instead.
func (rf *rotationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	return NewGeometriesInFrame(rf.Name(), nil), nil
}

func (rf rotationalFrame) MarshalJSON() ([]byte, error) {
	if len(rf.limits) > 1 {
		return nil, ErrMarshalingHighDOFFrame
	}
	temp := JointConfig{
		ID:   rf.name,
		Type: RevoluteJoint,
		Axis: spatial.AxisConfig{rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z},
		Max:  utils.RadToDeg(rf.limits[0].Max),
		Min:  utils.RadToDeg(rf.limits[0].Min),
	}

	return json.Marshal(temp)
}

func (rf *rotationalFrame) UnmarshalJSON(data []byte) error {
	var cfg JointConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	rf.baseFrame = &baseFrame{name: cfg.ID, limits: []Limit{{Min: utils.DegToRad(cfg.Min), Max: utils.DegToRad(cfg.Max)}}}
	rotAxis := cfg.Axis.ParseConfig()
	rf.rotAxis = r3.Vector{X: rotAxis.RX, Y: rotAxis.RY, Z: rotAxis.RZ}
	return nil
}

type poseFrame struct {
	*baseFrame
	geometries []spatial.Geometry
}

// NewPoseFrame creates an orientation vector frame, i.e a frame with
// 7 degrees of freedom: X, Y, Z, OX, OY, OZ, and Theta in radians.
func NewPoseFrame(name string, geometry []spatial.Geometry) (Frame, error) {
	limits := []Limit{
		{Min: math.Inf(-1), Max: math.Inf(1)}, // X
		{Min: math.Inf(-1), Max: math.Inf(1)}, // Y
		{Min: math.Inf(-1), Max: math.Inf(1)}, // Z
		{Min: math.Inf(-1), Max: math.Inf(1)}, // OX
		{Min: math.Inf(-1), Max: math.Inf(1)}, // OY
		{Min: math.Inf(-1), Max: math.Inf(1)}, // OZ
		{Min: math.Inf(-1), Max: math.Inf(1)}, // Theta
	}
	return &poseFrame{
		&baseFrame{name, limits},
		geometry,
	}, nil
}

func (pf *poseFrame) Hash() int {
	h := pf.hash() + (111 * len(pf.geometries))
	for _, g := range pf.geometries {
		h += g.Hash()
	}
	return h
}

// Transform on the poseFrame acts as the identity function. Whatever inputs are given are directly translated
// in a 7DoF pose. We note that theta should be in radians.
func (pf *poseFrame) Transform(inputs []Input) (spatial.Pose, error) {
	if err := pf.baseFrame.validInputs(inputs); err != nil {
		return nil, err
	}
	return spatial.NewPose(
		r3.Vector{X: inputs[0], Y: inputs[1], Z: inputs[2]},
		&spatial.OrientationVector{
			OX:    inputs[3],
			OY:    inputs[4],
			OZ:    inputs[5],
			Theta: inputs[6],
		},
	), nil
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (pf *poseFrame) Interpolate(from, to []Input, by float64) ([]Input, error) {
	if err := pf.baseFrame.validInputs(from); err != nil {
		return nil, NewIncorrectDoFError(len(from), len(pf.DoF()))
	}
	if err := pf.baseFrame.validInputs(to); err != nil {
		return nil, NewIncorrectDoFError(len(to), len(pf.DoF()))
	}
	fromPose, err := pf.Transform(from)
	if err != nil {
		return nil, err
	}
	toPose, err := pf.Transform(to)
	if err != nil {
		return nil, err
	}
	interpolatedPose := spatial.Interpolate(fromPose, toPose, by)
	return PoseToInputs(interpolatedPose), nil
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (pf *poseFrame) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	transformByPose, err := pf.Transform(inputs)
	if err != nil {
		return nil, err
	}
	if len(pf.geometries) == 0 {
		return NewGeometriesInFrame(pf.name, []spatial.Geometry{}), nil
	}
	transformedGeometries := []spatial.Geometry{}
	for _, geom := range pf.geometries {
		transformedGeometries = append(transformedGeometries, geom.Transform(transformByPose))
	}
	return NewGeometriesInFrame(pf.name, transformedGeometries), nil
}

// DoF returns the number of degrees of freedom within a model.
func (pf *poseFrame) DoF() []Limit {
	return pf.limits
}

// MarshalJSON serializes a Model.
func (pf *poseFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("serializing a poseFrame is currently not supported")
}

// UnmarshalJSON parses a poseFrame.
func (pf *poseFrame) UnmarshalJSON(data []byte) error {
	return errors.New("deserializing a poseFrame is currently not supported")
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (pf *poseFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values[:len(jp.Values)-1] {
		n[idx] = d
	}
	n[len(jp.Values)-1] = utils.DegToRad(jp.Values[len(jp.Values)-1])
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (pf *poseFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input[:len(input)-1] {
		n[idx] = a
	}
	n[len(input)-1] = utils.RadToDeg(input[len(input)-1])
	return &pb.JointPositions{Values: n}
}

// PoseToInputs is a convenience method for turning a pose into a slice of inputs
// in the form [X, Y, Z, OX, OY, OZ, Theta (in radians)]
// This is the format that is expected by the poseFrame type and should not be used with other frames.
func PoseToInputs(p spatial.Pose) []Input {
	return []Input{
		p.Point().X, p.Point().Y, p.Point().Z,
		p.Orientation().OrientationVectorRadians().OX,
		p.Orientation().OrientationVectorRadians().OY,
		p.Orientation().OrientationVectorRadians().OZ,
		p.Orientation().OrientationVectorRadians().Theta,
	}
}

// framesAlmostEqual is a helper used in testing that determines whether two Frame instances are (nearly) identical.
// For now, we only support implementers of the Frame interface that are registered (see register.go).
// Future implementations within this package should extend this function and add support.
func framesAlmostEqual(frame1, frame2 Frame, epsilon float64) (bool, error) {
	if frame1 == nil {
		return frame2 == nil, nil
	} else if frame2 == nil {
		return false, nil
	}

	switch {
	case reflect.TypeOf(frame1) != reflect.TypeOf(frame2):
		return false, nil
	case frame1.Name() != frame2.Name():
		return false, nil
	case !limitsAlmostEqual(frame1.DoF(), frame2.DoF(), epsilon):
		return false, nil
	default:
	}

	switch f1 := frame1.(type) {
	case *staticFrame:
		f2 := frame2.(*staticFrame)
		switch {
		case !spatial.PoseAlmostEqual(f1.transform, f2.transform):
			return false, nil
		case !spatial.GeometriesAlmostEqual(f1.geometry, f2.geometry):
			return false, nil
		default:
		}
	case *rotationalFrame:
		f2 := frame2.(*rotationalFrame)
		if !spatial.R3VectorAlmostEqual(f1.rotAxis, f2.rotAxis, epsilon) {
			return false, nil
		}
	case *translationalFrame:
		f2 := frame2.(*translationalFrame)
		switch {
		case !spatial.R3VectorAlmostEqual(f1.transAxis, f2.transAxis, epsilon):
			return false, nil
		case !spatial.GeometriesAlmostEqual(f1.geometry, f2.geometry):
			return false, nil
		default:
		}
	case *tailGeometryStaticFrame:
		f2 := frame2.(*tailGeometryStaticFrame)
		switch {
		case f1.staticFrame == nil:
			return f2.staticFrame == nil, nil
		case f2.staticFrame == nil:
			return f1.staticFrame == nil, nil
		default:
			return framesAlmostEqual(f1.staticFrame, f2.staticFrame, epsilon)
		}
	case *SimpleModel:
		f2 := frame2.(*SimpleModel)
		ordTransforms1 := f1.ordTransforms
		ordTransforms2 := f2.ordTransforms
		if len(ordTransforms1) != len(ordTransforms2) {
			return false, nil
		} else {
			for i, f := range ordTransforms1 {
				frameEquality, err := framesAlmostEqual(f, ordTransforms2[i], epsilon)
				if err != nil {
					return false, err
				}
				if !frameEquality {
					return false, nil
				}
			}
		}
	default:
		return false, fmt.Errorf("equality conditions not defined for %t", frame1)
	}
	return true, nil
}

// clone makes a copy of a Frame.
func clone(f Frame) (Frame, error) {
	t := reflect.TypeOf(f)
	var newFrame Frame

	// If f is already a pointer type, we need to create a new instance of the underlying type
	if t.Kind() == reflect.Ptr {
		newValue := reflect.New(t.Elem())
		newFrame = newValue.Interface().(Frame)
	} else {
		newValue := reflect.New(t)
		newFramePointer := newValue.Interface().(*Frame)
		newFrame = *newFramePointer
	}

	data, err := f.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = newFrame.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}

	return newFrame, nil
}
