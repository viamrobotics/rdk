package referenceframe

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/spatialmath"
)

// Transformable is an interface to describe elements that can be transformed by the frame system.
type Transformable interface {
	Transform(*PoseInFrame) Transformable
	Parent() string
}

// PoseInFrame is a data structure that packages a pose with the name of the
// frame in which it was observed.
type PoseInFrame struct {
	parent string
	pose   spatialmath.Pose
	name   string

	// GoalCloud represents a "cloud" of poses that can be considered equivalent to this one. This
	// is only used for motion requests that can accept a range of goal poses.
	GoalCloud *PoseCloud
}

// NewPoseInFrame generates a new PoseInFrame.
func NewPoseInFrame(frame string, pose spatialmath.Pose) *PoseInFrame {
	return &PoseInFrame{
		parent: frame,
		pose:   pose,
	}
}

// NewZeroPoseInFrame is a convenience method that creates a PoseInFrame with the specified Frame and a zero pose.
func NewZeroPoseInFrame(frame string) *PoseInFrame {
	return &PoseInFrame{
		parent: frame,
		pose:   spatialmath.NewZeroPose(),
	}
}

// NewPoseInFrameWithGoalCloud creates a new pose in frame where the goal pose is fuzzy.
func NewPoseInFrameWithGoalCloud(frame string, pose spatialmath.Pose, goalCloud *PoseCloud) *PoseInFrame {
	return &PoseInFrame{
		parent:    frame,
		pose:      pose,
		GoalCloud: goalCloud,
	}
}

// Parent returns the name of the frame in which the pose was observed. Needed for Transformable interface.
func (pF *PoseInFrame) Parent() string {
	return pF.parent
}

// SetParent sets the name of the frame in which the pose was observed.
func (pF *PoseInFrame) SetParent(parent string) {
	pF.parent = parent
}

// Pose returns the pose that was observed.
func (pF *PoseInFrame) Pose() spatialmath.Pose {
	return pF.pose
}

// Name returns the name of the PoseInFrame.
func (pF *PoseInFrame) Name() string {
	return pF.name
}

// SetName sets the name of the PoseInFrame.
func (pF *PoseInFrame) SetName(name string) {
	pF.name = name
}

// Transform changes the PoseInFrame pF into the reference frame specified by the tf argument.
// The tf PoseInFrame represents the pose of the pF reference frame with respect to the destination reference frame.
func (pF *PoseInFrame) Transform(tf *PoseInFrame) Transformable {
	return NewPoseInFrameWithGoalCloud(
		tf.parent, spatialmath.Compose(tf.pose, pF.pose), pF.GoalCloud,
	)
}

// TransformOpt transforms the `pF` as a DualQuaternion in place.
func (pF *PoseInFrame) TransformOpt(tf *PoseInFrame) {
	pF.pose = &spatialmath.DualQuaternion{
		Number: spatialmath.DualQuaternionFromPose(tf.pose).
			Transformation(spatialmath.DualQuaternionFromPose(pF.pose).Number),
	}
}

// String returns the string representation of the PoseInFrame.
func (pF *PoseInFrame) String() string {
	return fmt.Sprintf("name: %v parent: %v, pose: %v", pF.name, pF.parent, pF.pose)
}

// MarshalJSON converts a PoseInFrame to JSON through its protobuf representation.
func (pF *PoseInFrame) MarshalJSON() ([]byte, error) {
	pFProto := PoseInFrameToProtobuf(pF)
	return protojson.Marshal(pFProto)
}

// UnmarshalJSON parses a PoseInFrame from its protobuf representation in JSON bytes.
func (pF *PoseInFrame) UnmarshalJSON(data []byte) error {
	var pFProto commonpb.PoseInFrame
	if err := protojson.Unmarshal(data, &pFProto); err != nil {
		return err
	}
	newPF := ProtobufToPoseInFrame(&pFProto)
	*pF = *newPF
	return nil
}

// LinkInFrame is a PoseInFrame plus a Geometry, along with the optional identity and metadata
// carried by a Transform protobuf message.
type LinkInFrame struct {
	*PoseInFrame
	geometry spatialmath.Geometry
	uuid     []byte
	metadata map[string]interface{}
}

// NewLinkInFrame generates a new LinkInFrame.
func NewLinkInFrame(frame string, pose spatialmath.Pose, name string, geometry spatialmath.Geometry) *LinkInFrame {
	return &LinkInFrame{
		PoseInFrame: &PoseInFrame{
			parent: frame,
			pose:   pose,
			name:   name,
		},
		geometry: geometry,
	}
}

// SetGeometry replaces the existing geometry with the input. This only exists to deal with the
// clunkiness of the `LinkConfig` type that only speaks `GeometryConfig`s while we also allow for
// resources to simply declare their `[]Geometry` objects. Which is a different kind of impedance at
// the moment.
func (lF *LinkInFrame) SetGeometry(geom spatialmath.Geometry) {
	lF.geometry = geom
}

// Geometry returns the Geometry of the LinkInFrame.
func (lF *LinkInFrame) Geometry() spatialmath.Geometry {
	return lF.geometry
}

// UUID returns the identifier of the LinkInFrame, or nil if none was set.
func (lF *LinkInFrame) UUID() []byte {
	return lF.uuid
}

// SetUUID sets an identifier that lets consumers correlate this LinkInFrame across calls.
func (lF *LinkInFrame) SetUUID(uuid []byte) {
	lF.uuid = uuid
}

// Metadata returns the metadata of the LinkInFrame, or nil if none was set.
func (lF *LinkInFrame) Metadata() map[string]interface{} {
	return lF.metadata
}

// SetMetadata sets arbitrary metadata, such as color or opacity, on the LinkInFrame.
func (lF *LinkInFrame) SetMetadata(metadata map[string]interface{}) {
	lF.metadata = metadata
}

// ToStaticFrame converts a LinkInFrame into a staticFrame with a new name.
func (lF *LinkInFrame) ToStaticFrame(name string) (Frame, error) {
	if name == "" {
		name = lF.name
	}
	pose := lF.pose
	if pose == nil {
		pose = spatialmath.NewZeroPose()
	}
	if lF.geometry != nil {
		// deep copy geometry
		newGeom := lF.geometry.Transform(spatialmath.NewZeroPose())
		newGeom.SetLabel(name)
		return NewStaticFrameWithGeometry(name, pose, newGeom)
	}

	return NewStaticFrame(name, pose)
}

// PoseInFrameToProtobuf converts a PoseInFrame struct to a PoseInFrame protobuf message.
func PoseInFrameToProtobuf(framedPose *PoseInFrame) *commonpb.PoseInFrame {
	if framedPose == nil {
		return &commonpb.PoseInFrame{}
	}

	poseProto := &commonpb.Pose{}
	if framedPose.pose != nil {
		poseProto = spatialmath.PoseToProtobuf(framedPose.pose)
	}
	return &commonpb.PoseInFrame{
		ReferenceFrame: framedPose.parent,
		Pose:           poseProto,
		GoalCloud:      framedPose.GoalCloud.ToProto(),
	}
}

// ProtobufToPoseInFrame converts a PoseInFrame protobuf message to a PoseInFrame struct.
func ProtobufToPoseInFrame(proto *commonpb.PoseInFrame) *PoseInFrame {
	result := &PoseInFrame{}
	result.pose = spatialmath.NewPoseFromProtobuf(proto.GetPose())
	result.parent = proto.GetReferenceFrame()
	result.GoalCloud = PoseCloudFromProto(proto.GetGoalCloud())
	return result
}

// LinkInFrameToTransformProtobuf converts a LinkInFrame struct to a Transform protobuf message.
func LinkInFrameToTransformProtobuf(framedLink *LinkInFrame) (*commonpb.Transform, error) {
	if framedLink.PoseInFrame == nil {
		return nil, ErrNilPoseInFrame
	}
	if framedLink.name == "" {
		return nil, ErrEmptyStringFrameName
	}
	tform := &commonpb.Transform{
		ReferenceFrame:      framedLink.name,
		PoseInObserverFrame: PoseInFrameToProtobuf(framedLink.PoseInFrame),
		Uuid:                framedLink.uuid,
	}
	if framedLink.geometry != nil {
		tform.PhysicalObject = framedLink.geometry.ToProtobuf()
	}
	// Metadata is an optional message, so a nil map stays unset rather than becoming an empty struct.
	if framedLink.metadata != nil {
		md, err := protoutils.StructToStructPb(framedLink.metadata)
		if err != nil {
			return nil, err
		}
		tform.Metadata = md
	}
	return tform, nil
}

// LinkInFrameFromTransformProtobuf converts a Transform protobuf message to a LinkInFrame struct.
func LinkInFrameFromTransformProtobuf(proto *commonpb.Transform) (*LinkInFrame, error) {
	var err error
	frameName := proto.GetReferenceFrame()
	if frameName == "" {
		return nil, ErrEmptyStringFrameName
	}
	poseInObserverFrame := proto.GetPoseInObserverFrame()
	parentFrame := poseInObserverFrame.GetReferenceFrame()
	if parentFrame == "" {
		return nil, ErrEmptyStringFrameName
	}
	poseMsg := poseInObserverFrame.GetPose()
	pose := spatialmath.NewPoseFromProtobuf(poseMsg)
	var geometry spatialmath.Geometry
	if proto.PhysicalObject != nil {
		geometry, err = NewGeometryFromProto(proto.PhysicalObject)
		if err != nil {
			return nil, err
		}
	}
	link := NewLinkInFrame(parentFrame, pose, frameName, geometry)
	link.uuid = proto.GetUuid()
	if proto.Metadata != nil {
		link.metadata = proto.Metadata.AsMap()
	}
	return link, nil
}

// LinkInFramesToTransformsProtobuf converts a slice of LinkInFrame structs to a slice of Transform protobuf messages.
// TODO(rb): use generics to operate on lists of arbirary types.
func LinkInFramesToTransformsProtobuf(linkSlice []*LinkInFrame) ([]*commonpb.Transform, error) {
	protoTransforms := make([]*commonpb.Transform, 0, len(linkSlice))
	for i, link := range linkSlice {
		protoTf, err := LinkInFrameToTransformProtobuf(link)
		if err != nil {
			return nil, errors.Wrapf(err, "conversion error at index %d", i)
		}
		protoTransforms = append(protoTransforms, protoTf)
	}
	return protoTransforms, nil
}

// LinkInFramesFromTransformsProtobuf converts a slice of Transform protobuf messages to a slice of LinkInFrame structs.
// TODO(rb): use generics to operate on lists of arbirary proto types.
func LinkInFramesFromTransformsProtobuf(protoSlice []*commonpb.Transform) ([]*LinkInFrame, error) {
	links := make([]*LinkInFrame, 0, len(protoSlice))
	for i, protoTransform := range protoSlice {
		link, err := LinkInFrameFromTransformProtobuf(protoTransform)
		if err != nil {
			return nil, errors.Wrapf(err, "conversion error at index %d", i)
		}
		links = append(links, link)
	}
	return links, nil
}

// GeometriesInFrame is a data structure that packages geometries with the name of the frame in which it was observed.
type GeometriesInFrame struct {
	frame      string
	geometries []spatialmath.Geometry
}

// NewGeometriesInFrame generates a new GeometriesInFrame.
func NewGeometriesInFrame(frame string, geometries []spatialmath.Geometry) *GeometriesInFrame {
	return &GeometriesInFrame{
		frame:      frame,
		geometries: geometries,
	}
}

// Parent returns the name of the frame in which the geometries were observed.
func (gF *GeometriesInFrame) Parent() string {
	return gF.frame
}

// Geometries returns the geometries observed.
func (gF *GeometriesInFrame) Geometries() []spatialmath.Geometry {
	if gF.geometries == nil {
		return []spatialmath.Geometry{}
	}
	return gF.geometries
}

// GeometryByName returns the named geometry if it exists in the GeometriesInFrame, and nil otherwise.
// If multiple geometries exist with identical names one will be chosen at random.
func (gF *GeometriesInFrame) GeometryByName(name string) spatialmath.Geometry {
	for _, g := range gF.geometries {
		if g.Label() == name {
			return g
		}
	}
	return nil
}

// Transform changes the GeometriesInFrame gF into the reference frame specified by the tf argument.
// The tf PoseInFrame represents the pose of the gF reference frame with respect to the destination reference frame.
func (gF *GeometriesInFrame) Transform(tf *PoseInFrame) Transformable {
	geometries := make([]spatialmath.Geometry, 0, len(gF.geometries))
	for _, geometry := range gF.geometries {
		geometries = append(geometries, geometry.Transform(tf.pose))
	}
	return NewGeometriesInFrame(tf.parent, geometries)
}

// GeometriesInFrameToProtobuf converts a GeometriesInFrame struct to a GeometriesInFrame message as specified in common.proto.
func GeometriesInFrameToProtobuf(framedGeometries *GeometriesInFrame) *commonpb.GeometriesInFrame {
	return &commonpb.GeometriesInFrame{
		ReferenceFrame: framedGeometries.frame,
		Geometries:     NewGeometriesToProto(framedGeometries.Geometries()),
	}
}

// ProtobufToGeometriesInFrame converts a GeometriesInFrame message as specified in common.proto to a GeometriesInFrame struct.
func ProtobufToGeometriesInFrame(proto *commonpb.GeometriesInFrame) (*GeometriesInFrame, error) {
	geometries, err := NewGeometriesFromProto(proto.GetGeometries())
	if err != nil {
		return nil, err
	}
	return NewGeometriesInFrame(proto.GetReferenceFrame(), geometries), nil
}

type geometriesInFrameJSON struct {
	Frame      string                        `json:"frame"`
	Geometries []*spatialmath.GeometryConfig `json:"geometries"`
}

// MarshalJSON implements the json.Marshaler interface.
func (gF *GeometriesInFrame) MarshalJSON() ([]byte, error) {
	configs := make([]*spatialmath.GeometryConfig, 0, len(gF.geometries))
	for _, geometry := range gF.geometries {
		config, err := spatialmath.NewGeometryConfig(geometry)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return json.Marshal(geometriesInFrameJSON{
		Frame:      gF.frame,
		Geometries: configs,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (gF *GeometriesInFrame) UnmarshalJSON(data []byte) error {
	var raw geometriesInFrameJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	geometries := make([]spatialmath.Geometry, 0, len(raw.Geometries))
	for _, config := range raw.Geometries {
		geometry, err := config.ParseConfig()
		if err != nil {
			return err
		}
		geometries = append(geometries, geometry)
	}
	*gF = *NewGeometriesInFrame(raw.Frame, geometries)
	return nil
}
