package referenceframe

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/spatialmath"
)

// A Model represents a frame that can change its name, and can return itself as a ModelConfig struct.
type Model interface {
	Frame
	ModelConfig() *ModelConfigJSON
}

// KinematicModelFromProtobuf returns a model from a protobuf message representing it.
func KinematicModelFromProtobuf(name string, resp *commonpb.GetKinematicsResponse) (Model, error) {
	if resp == nil {
		return nil, errors.New("*commonpb.GetKinematicsResponse can't be nil")
	}
	format := resp.GetFormat()
	data := resp.GetKinematicsData()

	switch format {
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA:
		return UnmarshalModelJSON(data, name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF:
		meshMap := resp.GetMeshesByUrdfFilepath()
		modelconf, err := UnmarshalModelXML(data, name, meshMap)
		if err != nil {
			return nil, err
		}
		return modelconf.ParseConfig(name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED:
		fallthrough
	default:
		if formatName, ok := commonpb.KinematicsFileFormat_name[int32(format)]; ok {
			return nil, fmt.Errorf("unable to parse file of type %s", formatName)
		}
		return nil, fmt.Errorf("unable to parse unknown file type %d", format)
	}
}

// KinematicModelToProtobuf converts a model into a protobuf message version of that model.
func KinematicModelToProtobuf(model Model) *commonpb.GetKinematicsResponse {
	if model == nil {
		return &commonpb.GetKinematicsResponse{Format: commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED}
	}

	cfg := model.ModelConfig()
	if cfg == nil || cfg.OriginalFile == nil {
		return &commonpb.GetKinematicsResponse{Format: commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED}
	}
	resp := &commonpb.GetKinematicsResponse{KinematicsData: cfg.OriginalFile.Bytes}
	switch cfg.OriginalFile.Extension {
	case "json":
		resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA
	case "urdf":
		resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF
		// Extract mesh data from geometries and populate mesh map for URDF
		resp.MeshesByUrdfFilepath = extractMeshMapFromModelConfig(cfg)
	default:
		resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED
	}
	return resp
}

// extractMeshMapFromModelConfig extracts mesh data from link geometries in a model config.
// Returns a map of URDF file paths to proto Mesh messages.
func extractMeshMapFromModelConfig(cfg *ModelConfigJSON) map[string]*commonpb.Mesh {
	meshMap := make(map[string]*commonpb.Mesh)

	// Iterate through all links and extract mesh geometries
	for _, link := range cfg.Links {
		if link.Geometry == nil {
			continue
		}

		// Check if this is a mesh geometry
		if link.Geometry.Type == spatialmath.MeshType && len(link.Geometry.MeshData) > 0 {
			// Use the original URDF mesh path if available
			meshPath := link.Geometry.MeshFilePath
			if meshPath == "" {
				// Fallback if path wasn't preserved (shouldn't happen with URDF)
				continue
			}

			meshMap[meshPath] = &commonpb.Mesh{
				Mesh:        link.Geometry.MeshData,
				ContentType: link.Geometry.MeshContentType,
			}
		}
	}

	return meshMap
}

// KinematicModelFromFile returns a model frame from a file that defines the kinematics.
func KinematicModelFromFile(modelPath, name string) (Model, error) {
	switch {
	case strings.HasSuffix(modelPath, ".urdf"):
		return ParseModelXMLFile(modelPath, name)
	case strings.HasSuffix(modelPath, ".json"):
		return ParseModelJSONFile(modelPath, name)
	default:
		return nil, errors.New("only files with .json and .urdf file extensions are supported")
	}
}

// SimpleModel is a model that serially concatenates a list of Frames.
type SimpleModel struct {
	baseFrame
	// OrdTransforms is the list of transforms ordered from end effector to base
	ordTransforms []Frame
	modelConfig   *ModelConfigJSON
	poseCache     sync.Map
}

// NewSimpleModel constructs a new model.
func NewSimpleModel(name string) *SimpleModel {
	return &SimpleModel{
		baseFrame: baseFrame{name: name},
	}
}

// NewSerialModel is a convenience constructor that builds a Model from a serial chain of frames.
// Returns an error if duplicate frame names are detected.
func NewSerialModel(name string, frames []Frame) (*SimpleModel, error) {
	seen := make(map[string]bool)
	for _, f := range frames {
		frameName := f.Name()
		if seen[frameName] {
			return nil, NewDuplicateFrameNameError(frameName)
		}
		seen[frameName] = true
	}

	m := NewSimpleModel(name)
	m.setOrdTransforms(frames)
	return m, nil
}

// NewModelWithLimitOverrides constructs a new model identical to base but with the specified
// joint limits overridden. Overrides are keyed by frame name. Each override replaces the
// first DoF limit of the matching frame.
func NewModelWithLimitOverrides(base *SimpleModel, overrides map[string]Limit) (*SimpleModel, error) {
	clonedFrames := make([]Frame, len(base.ordTransforms))
	for i, f := range base.ordTransforms {
		cloned, err := clone(f)
		if err != nil {
			return nil, fmt.Errorf("cloning frame %q: %w", f.Name(), err)
		}
		clonedFrames[i] = cloned
	}

	for name, limit := range overrides {
		found := false
		for _, f := range clonedFrames {
			if f.Name() == name && len(f.DoF()) > 0 {
				f.DoF()[0] = limit
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("frame %q not found or has no DoF", name)
		}
	}

	m := NewSimpleModel(base.name)
	m.setOrdTransforms(clonedFrames)
	m.modelConfig = base.modelConfig
	return m, nil
}

// MoveableFrameNames returns the names of frames with non-zero DoF, in order.
func (m *SimpleModel) MoveableFrameNames() []string {
	var names []string
	for _, f := range m.ordTransforms {
		if len(f.DoF()) > 0 {
			names = append(names, f.Name())
		}
	}
	return names
}

// setOrdTransforms sets the internal ordered transforms and recomputes limits.
func (m *SimpleModel) setOrdTransforms(fs []Frame) {
	m.ordTransforms = fs
	m.limits = []Limit{}
	for _, transform := range m.ordTransforms {
		if len(transform.DoF()) > 0 {
			m.limits = append(m.limits, transform.DoF()...)
		}
	}
}

// GenerateRandomConfiguration generates a list of radian joint positions that are random but valid for each joint.
func GenerateRandomConfiguration(m Model, randSeed *rand.Rand) []float64 {
	limits := m.DoF()
	jointPos := make([]float64, 0, len(limits))

	for i := 0; i < len(limits); i++ {
		jRange := math.Abs(limits[i].Max - limits[i].Min)
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		newPos := randSeed.Float64()*jRange + limits[i].Min
		jointPos = append(jointPos, newPos)
	}
	return jointPos
}

// ModelConfig returns the ModelConfig object used to create this model.
func (m *SimpleModel) ModelConfig() *ModelConfigJSON {
	return m.modelConfig
}

// Hash returns a hash value for this simple model.
func (m *SimpleModel) Hash() int {
	h := m.hash()
	for _, f := range m.ordTransforms {
		h += f.Hash()
	}
	return h
}

// Transform takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func (m *SimpleModel) Transform(inputs []Input) (spatialmath.Pose, error) {
	return m.InputsToTransformOpt(inputs)
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (m *SimpleModel) Interpolate(from, to []Input, by float64) ([]Input, error) {
	interp := make([]Input, 0, len(from))
	posIdx := 0
	for _, transform := range m.ordTransforms {
		dof := len(transform.DoF()) + posIdx
		fromSubset := from[posIdx:dof]
		toSubset := to[posIdx:dof]
		posIdx = dof

		interpSubset, err := transform.Interpolate(fromSubset, toSubset, by)
		if err != nil {
			return nil, err
		}
		interp = append(interp, interpSubset...)
	}
	return interp, nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (m *SimpleModel) InputFromProtobuf(jp *pb.JointPositions) []Input {
	inputs := make([]Input, 0, len(jp.Values))
	posIdx := 0
	for _, transform := range m.ordTransforms {
		dof := len(transform.DoF()) + posIdx
		jPos := jp.Values[posIdx:dof]
		posIdx = dof
		inputs = append(inputs, transform.InputFromProtobuf(&pb.JointPositions{Values: jPos})...)
	}
	return inputs
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (m *SimpleModel) ProtobufFromInput(input []Input) *pb.JointPositions {
	jPos := &pb.JointPositions{}
	posIdx := 0
	for _, transform := range m.ordTransforms {
		dof := len(transform.DoF()) + posIdx
		jPos.Values = append(jPos.Values, transform.ProtobufFromInput(input[posIdx:dof]).Values...)
		posIdx = dof
	}
	return jPos
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (m *SimpleModel) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	frames, err := m.inputsToFrames(inputs, true)
	if err != nil && frames == nil {
		return nil, err
	}
	var errAll error
	geometries := make([]spatialmath.Geometry, 0, len(frames))
	for _, frame := range frames {
		geometriesInFrame, err := frame.Geometries([]Input{})
		if err != nil {
			multierr.AppendInto(&errAll, err)
			continue
		}
		for _, geom := range geometriesInFrame.Geometries() {
			placedGeom := geom.Transform(frame.transform)
			placedGeom.SetLabel(m.name + ":" + geom.Label())
			geometries = append(geometries, placedGeom)
		}
	}
	return NewGeometriesInFrame(m.name, geometries), errAll
}

// CachedTransform will check a sync.Map cache to see if the exact given set of inputs has been computed yet. If so
// it returns without redoing the calculation. Thread safe, but so far has tended to be slightly slower than just doing
// the calculation. This may change with higher DOF models and longer runtimes.
func (m *SimpleModel) CachedTransform(inputs []Input) (spatialmath.Pose, error) {
	key := floatsToString(inputs)
	if val, ok := m.poseCache.Load(key); ok {
		if pose, ok := val.(spatialmath.Pose); ok {
			return pose, nil
		}
	}
	poses, err := m.inputsToFrames(inputs, false)
	if err != nil && poses == nil {
		return nil, err
	}
	m.poseCache.Store(key, poses[len(poses)-1].transform)

	return poses[len(poses)-1].transform, err
}

// DoF returns the number of degrees of freedom within a model.
func (m *SimpleModel) DoF() []Limit {
	return m.limits
}

// MarshalJSON serializes a Model.
func (m *SimpleModel) MarshalJSON() ([]byte, error) {
	type serialized struct {
		Name   string           `json:"name"`
		Model  *ModelConfigJSON `json:"model,omitempty"`
		Limits []Limit          `json:"limits"`
	}
	ser := serialized{
		Name:   m.name,
		Model:  m.modelConfig,
		Limits: m.limits,
	}
	return json.Marshal(ser)
}

// UnmarshalJSON deserializes a Model.
func (m *SimpleModel) UnmarshalJSON(data []byte) error {
	type serialized struct {
		Name   string           `json:"name"`
		Model  *ModelConfigJSON `json:"model,omitempty"`
		Limits []Limit          `json:"limits"`
	}
	var ser serialized
	if err := json.Unmarshal(data, &ser); err != nil {
		return err
	}

	frameName := ser.Name
	if frameName == "" {
		frameName = ser.Model.Name
	}

	if ser.Model != nil {
		parsed, err := ser.Model.ParseConfig(ser.Model.Name)
		if err != nil {
			return err
		}
		newModel, ok := parsed.(*SimpleModel)
		if !ok {
			return fmt.Errorf("could not parse config for simple model, name: %v", ser.Name)
		}
		m.ordTransforms = newModel.ordTransforms
	}
	m.baseFrame = baseFrame{name: frameName, limits: ser.Limits}
	m.modelConfig = ser.Model

	return nil
}

// inputsToFrames takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of each of the links up to and including the end effector. This is useful for when conversions
// between quaternions and OV are not needed.
func (m *SimpleModel) inputsToFrames(inputs []Input, collectAll bool) ([]*staticFrame, error) {
	if len(m.DoF()) != len(inputs) {
		return nil, NewIncorrectDoFError(len(inputs), len(m.DoF()))
	}

	poses := make([]*staticFrame, 0, len(m.ordTransforms))
	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	composedTransformation := spatialmath.NewZeroPose()
	posIdx := 0

	// get quaternions from the base outwards.
	for _, transform := range m.ordTransforms {
		dof := len(transform.DoF()) + posIdx
		input := inputs[posIdx:dof]
		posIdx = dof

		pose, err := transform.Transform(input)
		// Fail if inputs are incorrect and pose is nil, but allow querying out-of-bounds positions
		if pose == nil || err != nil {
			return nil, err
		}

		if collectAll {
			var geometry spatialmath.Geometry
			gf, err := transform.Geometries(input)
			if err != nil {
				return nil, err
			}

			geometries := gf.geometries
			if len(geometries) == 0 {
				geometry = nil
			} else {
				geometry = geometries[0]
			}

			// TODO(pl): Part of the implementation for GetGeometries will require removing the single geometry restriction
			fixedFrame, err := NewStaticFrameWithGeometry(transform.Name(), composedTransformation, geometry)
			if err != nil {
				return nil, err
			}

			poses = append(poses, fixedFrame.(*staticFrame))
		}

		composedTransformation = spatialmath.Compose(composedTransformation, pose)
	}

	// TODO(rb) as written this will return one too many frames, no need to return zeroth frame
	poses = append(poses, &staticFrame{&baseFrame{"", []Limit{}}, composedTransformation, nil})
	return poses, nil
}

// InputsToTransformOpt is like `inputsToFrames` but only returns the end effector pose. That allows
// us to optimize away the recording of intermediate computations.
func (m *SimpleModel) InputsToTransformOpt(inputs []Input) (spatialmath.Pose, error) {
	if len(m.DoF()) != len(inputs) {
		return nil, NewIncorrectDoFError(len(inputs), len(m.DoF()))
	}

	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	// composedTransformation := spatialmath.NewZeroPose()
	composedTransformation := spatialmath.DualQuaternion{
		Number: dualquat.Number{
			Real: quat.Number{Real: 1},
			Dual: quat.Number{},
		},
	}
	posIdx := 0

	// get quaternions from the base outwards.
	for _, transformI := range m.ordTransforms {
		var pose spatialmath.Pose

		switch transform := transformI.(type) {
		case *staticFrame:
			if len(transformI.DoF()) != 0 {
				return nil, NewIncorrectDoFError(len(transformI.DoF()), 0)
			}

			composedTransformation = spatialmath.DualQuaternion{
				Number: composedTransformation.Transformation(transform.transform.(*spatialmath.DualQuaternion).Number),
			}
		case *rotationalFrame:
			if len(transformI.DoF()) != 1 {
				return nil, NewIncorrectDoFError(len(transformI.DoF()), 1)
			}

			if err := transform.validInputs(inputs[posIdx : posIdx+1]); err != nil {
				return nil, err
			}

			orientation := transform.InputToOrientation(inputs[posIdx])
			pose = &spatialmath.DualQuaternion{
				Number: dualquat.Number{
					Real: orientation.Quaternion(),
				},
			}

			posIdx++
			composedTransformation = spatialmath.DualQuaternion{
				Number: composedTransformation.Transformation(pose.(*spatialmath.DualQuaternion).Number),
			}
		default:
			dof := len(transformI.DoF()) + posIdx
			input := inputs[posIdx:dof]
			posIdx = dof

			var err error
			pose, err = transform.Transform(input)
			if err != nil {
				return nil, err
			}

			composedTransformation = spatialmath.DualQuaternion{
				Number: composedTransformation.Transformation(pose.(*spatialmath.DualQuaternion).Number),
			}
		}
	}

	return &composedTransformation, nil
}

// floatsToString turns a float array into a serializable binary representation
// This is very fast, about 100ns per call.
func floatsToString(inputs []Input) string {
	b := make([]byte, len(inputs)*8)
	for i, input := range inputs {
		binary.BigEndian.PutUint64(b[8*i:8*i+8], math.Float64bits(input))
	}
	return string(b)
}

// New2DMobileModelFrame builds the kinematic model associated with the kinematicWheeledBase
// This model is intended to be used with a mobile base and has either 2DOF corresponding to  a state of x, y
// or has 3DOF corresponding to a state of x, y, and theta, where x and y are the positional coordinates
// the base is located about and theta is the rotation about the z axis.
func New2DMobileModelFrame(name string, limits []Limit, collisionGeometry spatialmath.Geometry) (Model, error) {
	if len(limits) != 2 && len(limits) != 3 {
		return nil,
			errors.Errorf("Must have 2DOF state (x, y) or 3DOF state (x, y, theta) to create 2DMobileModelFrame, have %d dof", len(limits))
	}

	// build the model - SLAM convention is that the XY plane is the ground plane
	x, err := NewTranslationalFrame("x", r3.Vector{X: 1}, limits[0])
	if err != nil {
		return nil, err
	}
	y, err := NewTranslationalFrame("y", r3.Vector{Y: 1}, limits[1])
	if err != nil {
		return nil, err
	}
	geometry, err := NewStaticFrameWithGeometry("geometry", spatialmath.NewZeroPose(), collisionGeometry)
	if err != nil {
		return nil, err
	}

	var frames []Frame
	if len(limits) == 3 {
		theta, err := NewRotationalFrame("theta", *spatialmath.NewR4AA(), limits[2])
		if err != nil {
			return nil, err
		}
		frames = []Frame{x, y, theta, geometry}
	} else {
		frames = []Frame{x, y, geometry}
	}

	return NewSerialModel(name, frames)
}
