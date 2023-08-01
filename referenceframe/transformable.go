package referenceframe

import (
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

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
}

// LinkInFrame is a PoseInFrame plus a Geometry.
type LinkInFrame struct {
	*PoseInFrame
	geometry spatialmath.Geometry
}

// Geometry returns the Geometry of the LinkInFrame.
func (lF *LinkInFrame) Geometry() spatialmath.Geometry {
	return lF.geometry
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
	return NewPoseInFrame(tf.parent, spatialmath.Compose(tf.pose, pF.pose))
}

// NewPoseInFrame generates a new PoseInFrame.
func NewPoseInFrame(parentFrame string, pose spatialmath.Pose) *PoseInFrame {
	return &PoseInFrame{
		parent: parentFrame,
		pose:   pose,
	}
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

// PoseInFrameToProtobuf converts a PoseInFrame struct to a PoseInFrame protobuf message.
func PoseInFrameToProtobuf(framedPose *PoseInFrame) *commonpb.PoseInFrame {
	poseProto := &commonpb.Pose{}
	if framedPose.pose != nil {
		poseProto = spatialmath.PoseToProtobuf(framedPose.pose)
	}
	return &commonpb.PoseInFrame{
		ReferenceFrame: framedPose.parent,
		Pose:           poseProto,
	}
}

// ProtobufToPoseInFrame converts a PoseInFrame protobuf message to a PoseInFrame struct.
func ProtobufToPoseInFrame(proto *commonpb.PoseInFrame) *PoseInFrame {
	result := &PoseInFrame{}
	result.pose = spatialmath.NewPoseFromProtobuf(proto.GetPose())
	result.parent = proto.GetReferenceFrame()
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
	}
	if framedLink.geometry != nil {
		tform.PhysicalObject = framedLink.geometry.ToProtobuf()
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
		geometry, err = spatialmath.NewGeometryFromProto(proto.PhysicalObject)
		if err != nil {
			return nil, err
		}
	}
	return NewLinkInFrame(parentFrame, pose, frameName, geometry), nil
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

	// This is an internal data structure used for O(1) access to named sub-geometries.
	// Do not access directly. This will not be accurate for unnamed geometries.
	nameIndexMap map[string]int
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
	if gF.nameIndexMap == nil {
		return nil
	}
	if i, ok := gF.nameIndexMap[name]; ok {
		return gF.geometries[i]
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

// NewGeometriesInFrame generates a new GeometriesInFrame.
func NewGeometriesInFrame(frame string, geometries []spatialmath.Geometry) *GeometriesInFrame {
	nameIndexMap := make(map[string]int)
	for i, geometry := range geometries {
		nameIndexMap[geometry.Label()] = i
	}
	return &GeometriesInFrame{
		frame:        frame,
		geometries:   geometries,
		nameIndexMap: nameIndexMap,
	}
}

// GeometriesInFrameToProtobuf converts a GeometriesInFrame struct to a GeometriesInFrame message as specified in common.proto.
func GeometriesInFrameToProtobuf(framedGeometries *GeometriesInFrame) *commonpb.GeometriesInFrame {
	return &commonpb.GeometriesInFrame{
		ReferenceFrame: framedGeometries.frame,
		Geometries:     spatialmath.NewGeometriesToProto(framedGeometries.Geometries()),
	}
}

// ProtobufToGeometriesInFrame converts a GeometriesInFrame message as specified in common.proto to a GeometriesInFrame struct.
func ProtobufToGeometriesInFrame(proto *commonpb.GeometriesInFrame) (*GeometriesInFrame, error) {
	geometries, err := spatialmath.NewGeometriesFromProto(proto.GetGeometries())
	if err != nil {
		return nil, err
	}
	return NewGeometriesInFrame(proto.GetReferenceFrame(), geometries), nil
}
