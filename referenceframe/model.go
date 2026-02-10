package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

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
		resp.MeshesByUrdfFilepath = extractMeshMapFromModelConfig(cfg)
	default:
		resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED
	}
	return resp
}

func extractMeshMapFromModelConfig(cfg *ModelConfigJSON) map[string]*commonpb.Mesh {
	meshMap := make(map[string]*commonpb.Mesh)
	for _, link := range cfg.Links {
		if link.Geometry == nil {
			continue
		}
		if link.Geometry.Type == spatialmath.MeshType && len(link.Geometry.MeshData) > 0 {
			meshPath := link.Geometry.MeshFilePath
			if meshPath == "" {
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

// SimpleModel is a model that uses an internal FrameSystem to represent its kinematic tree.
// It supports both serial chains and branching tree topologies (e.g. grippers with branching fingers).
// A user-specified "primary output frame" determines what Transform() returns.
type SimpleModel struct {
	baseFrame
	internalFS         *FrameSystem        // tree of frames
	primaryOutputFrame string              // frame whose world-pose Transform() returns
	inputSchema        *LinearInputsSchema // canonical flat-input ↔ per-frame mapping
	modelConfig        *ModelConfigJSON
}

// NewSimpleModel constructs a new empty model with no kinematics.
func NewSimpleModel(name string) *SimpleModel {
	return &SimpleModel{
		baseFrame: baseFrame{name: name},
	}
}

// NewModel constructs a model from a FrameSystem and a primary output frame.
// The primary output frame must exist in fs and determines what Transform() returns.
func NewModel(name string, fs *FrameSystem, primaryOutputFrame string) (*SimpleModel, error) {
	if fs.Frame(primaryOutputFrame) == nil {
		return nil, fmt.Errorf("primary output frame %q not found in frame system", primaryOutputFrame)
	}

	m := &SimpleModel{
		baseFrame:          baseFrame{name: name},
		internalFS:         fs,
		primaryOutputFrame: primaryOutputFrame,
	}

	zeroInputs := newZeroLinearInputsTopological(fs)
	schema, err := zeroInputs.GetSchema(fs)
	if err != nil {
		return nil, err
	}
	m.inputSchema = schema
	m.limits = schema.GetLimits()

	return m, nil
}

// NewModelWithLimitOverrides constructs a new model identical to base but with the specified
// joint limits overridden. Overrides are keyed by frame name. Each override replaces the
// first DoF limit of the matching frame.
func NewModelWithLimitOverrides(base *SimpleModel, overrides map[string]Limit) (*SimpleModel, error) {
	newFS, err := cloneFrameSystem(base.internalFS)
	if err != nil {
		return nil, err
	}

	for name, limit := range overrides {
		frame := newFS.Frame(name)
		if frame == nil || len(frame.DoF()) == 0 {
			return nil, fmt.Errorf("frame %q not found or has no DoF", name)
		}
		frame.DoF()[0] = limit
	}

	m, err := NewModel(base.name, newFS, base.primaryOutputFrame)
	if err != nil {
		return nil, err
	}
	m.modelConfig = base.modelConfig
	return m, nil
}

// MoveableFrameNames returns the names of frames with non-zero DoF, in schema order.
func (m *SimpleModel) MoveableFrameNames() []string {
	if m.inputSchema == nil {
		return nil
	}
	var names []string
	for _, name := range m.inputSchema.FrameNamesInOrder() {
		frame := m.internalFS.Frame(name)
		if frame != nil && len(frame.DoF()) > 0 {
			names = append(names, name)
		}
	}
	return names
}

// NewSerialFrameSystem builds a FrameSystem from a serial chain of frames.
// frame[0] parent=world, frame[i] parent=frame[i-1].
// Duplicate frame names are automatically made unique.
// Returns the FrameSystem and the name of the last frame (for use as primaryOutputFrame).
func NewSerialFrameSystem(frames []Frame) (*FrameSystem, string, error) {
	fs := NewEmptyFrameSystem("internal")
	parentFrame := fs.World()
	nameCounts := map[string]int{}

	for _, f := range frames {
		nameCounts[f.Name()]++
		if nameCounts[f.Name()] > 1 {
			f = NewNamedFrame(f, fmt.Sprintf("%s_%d", f.Name(), nameCounts[f.Name()]))
		}
		if err := fs.AddFrame(f, parentFrame); err != nil {
			return nil, "", err
		}
		parentFrame = f
	}

	return fs, parentFrame.Name(), nil
}

// framesInOrder returns the Frame objects in schema order.
func (m *SimpleModel) framesInOrder() []Frame {
	if m.internalFS == nil || m.inputSchema == nil {
		return nil
	}
	names := m.inputSchema.FrameNamesInOrder()
	frames := make([]Frame, 0, len(names))
	for _, name := range names {
		f := m.internalFS.Frame(name)
		if f != nil {
			frames = append(frames, f)
		}
	}
	return frames
}

// toLinearInputs converts flat []Input to a *LinearInputs via the model's schema.
func (m *SimpleModel) toLinearInputs(inputs []Input) (*LinearInputs, error) {
	if len(m.DoF()) != len(inputs) {
		return nil, NewIncorrectDoFError(len(inputs), len(m.DoF()))
	}
	return m.inputSchema.FloatsToInputs(inputs)
}

// GenerateRandomConfiguration generates a list of radian joint positions that are random but valid for each joint.
func GenerateRandomConfiguration(m Model, randSeed *rand.Rand) []float64 {
	limits := m.DoF()
	jointPos := make([]float64, 0, len(limits))
	for i := 0; i < len(limits); i++ {
		jRange := math.Abs(limits[i].Max - limits[i].Min)
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
	h += hashString(m.name)
	for _, f := range m.framesInOrder() {
		h += f.Hash()
	}
	h += hashString(m.primaryOutputFrame)
	return h
}

// Transform returns the pose of the primary output frame given the flat input vector.
func (m *SimpleModel) Transform(inputs []Input) (spatialmath.Pose, error) {
	li, err := m.toLinearInputs(inputs)
	if err != nil {
		return nil, err
	}

	primaryFrame := m.internalFS.Frame(m.primaryOutputFrame)
	if primaryFrame == nil {
		return nil, NewFrameMissingError(m.primaryOutputFrame)
	}

	dq, err := m.internalFS.GetFrameToWorldTransform(li, primaryFrame)
	if err != nil {
		return nil, err
	}

	return &spatialmath.DualQuaternion{Number: dq}, nil
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (m *SimpleModel) Interpolate(from, to []Input, by float64) ([]Input, error) {
	interp := make([]Input, 0, len(from))
	posIdx := 0
	for _, transform := range m.framesInOrder() {
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
	for _, transform := range m.framesInOrder() {
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
	for _, transform := range m.framesInOrder() {
		dof := len(transform.DoF()) + posIdx
		jPos.Values = append(jPos.Values, transform.ProtobufFromInput(input[posIdx:dof]).Values...)
		posIdx = dof
	}
	return jPos
}

// Geometries returns the geometries for all frames in the model, placed in world coordinates.
func (m *SimpleModel) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	li, err := m.toLinearInputs(inputs)
	if err != nil {
		return nil, err
	}

	allGeomsMap, err := FrameSystemGeometriesLinearInputs(m.internalFS, li)
	if err != nil && len(allGeomsMap) == 0 {
		return nil, err
	}

	geometries := make([]spatialmath.Geometry, 0)
	for _, gif := range allGeomsMap {
		for _, geom := range gif.Geometries() {
			geom.SetLabel(m.name + ":" + geom.Label())
			geometries = append(geometries, geom)
		}
	}
	return NewGeometriesInFrame(m.name, geometries), err
}

// DoF returns the number of degrees of freedom within a model.
func (m *SimpleModel) DoF() []Limit {
	return m.limits
}

// MarshalJSON serializes a Model.
func (m *SimpleModel) MarshalJSON() ([]byte, error) {
	type serialized struct {
		Name   string           `json:"name"`
		Model  *ModelConfigJSON `json:"model"`
		Limits []Limit          `json:"limits"`
	}
	return json.Marshal(serialized{
		Name:   m.name,
		Model:  m.modelConfig,
		Limits: m.limits,
	})
}

// UnmarshalJSON deserializes a Model.
func (m *SimpleModel) UnmarshalJSON(data []byte) error {
	type serialized struct {
		Name   string           `json:"name"`
		Model  *ModelConfigJSON `json:"model"`
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
		m.internalFS = newModel.internalFS
		m.primaryOutputFrame = newModel.primaryOutputFrame
		m.inputSchema = newModel.inputSchema
	}
	m.baseFrame = baseFrame{name: frameName, limits: ser.Limits}
	m.modelConfig = ser.Model
	return nil
}

// New2DMobileModelFrame builds the kinematic model associated with the kinematicWheeledBase.
func New2DMobileModelFrame(name string, limits []Limit, collisionGeometry spatialmath.Geometry) (Model, error) {
	if len(limits) != 2 && len(limits) != 3 {
		return nil,
			errors.Errorf("Must have 2DOF state (x, y) or 3DOF state (x, y, theta) to create 2DMobileModelFrame, have %d dof", len(limits))
	}

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

	fs, lastFrame, err := NewSerialFrameSystem(frames)
	if err != nil {
		return nil, err
	}
	return NewModel(name, fs, lastFrame)
}

// ComputeOOBPosition takes a frame and a slice of Inputs and returns the cartesian position of the frame after
// transforming it by the given inputs even when if the inputs given would violate the Limits of the frame.
func ComputeOOBPosition(frame Frame, inputs []Input) (spatialmath.Pose, error) {
	if inputs == nil {
		return nil, errors.New("cannot compute position for nil joints")
	}
	if frame == nil {
		return nil, errors.New("cannot compute position for nil frame")
	}

	pose, err := frame.Transform(inputs)
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}

	return pose, nil
}
