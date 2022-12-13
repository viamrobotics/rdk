package referenceframe

import (
	"strconv"

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
	geometry spatialmath.GeometryCreator
}

// Geometry returns the GeometryCreator of the LinkInFrame.
func (lF *LinkInFrame) Geometry() spatialmath.GeometryCreator {
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
		return NewStaticFrameWithGeometry(name, pose, lF.geometry)
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
func NewPoseInFrame(frame string, pose spatialmath.Pose) *PoseInFrame {
	// fmt.Println("frame: ", frame)
	return &PoseInFrame{
		parent: frame,
		pose:   pose,
	}
}

// NewLinkInFrame generates a new LinkInFrame.
func NewLinkInFrame(frame string, pose spatialmath.Pose, name string, geometry spatialmath.GeometryCreator) *LinkInFrame {
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
	var geometry spatialmath.GeometryCreator
	if proto.PhysicalObject != nil {
		geometry, err = spatialmath.NewGeometryCreatorFromProto(proto.PhysicalObject)
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
	geometries map[string]spatialmath.Geometry
}

// Parent returns the name of the frame in which the geometries were observed.
func (gF *GeometriesInFrame) Parent() string {
	return gF.frame
}

// Geometries returns the geometries observed.
func (gF *GeometriesInFrame) Geometries() map[string]spatialmath.Geometry {
	return gF.geometries
}

// Transform changes the GeometriesInFrame gF into the reference frame specified by the tf argument.
// The tf PoseInFrame represents the pose of the gF reference frame with respect to the destination reference frame.
func (gF *GeometriesInFrame) Transform(tf *PoseInFrame) Transformable {
	geometries := make(map[string]spatialmath.Geometry)
	for name, geometry := range gF.geometries {
		geometries[name] = geometry.Transform(tf.pose)
	}
	return NewGeometriesInFrame(tf.parent, geometries)
}

// NewGeometriesInFrame generates a new GeometriesInFrame.
func NewGeometriesInFrame(frame string, geometries map[string]spatialmath.Geometry) *GeometriesInFrame {
	return &GeometriesInFrame{
		frame:      frame,
		geometries: geometries,
	}
}

// GeometriesInFrameToProtobuf converts a GeometriesInFrame struct to a GeometriesInFrame message as specified in common.proto.
func GeometriesInFrameToProtobuf(framedGeometries *GeometriesInFrame) *commonpb.GeometriesInFrame {
	var geometries []*commonpb.Geometry
	for _, geometry := range framedGeometries.geometries {
		geometries = append(geometries, geometry.ToProtobuf())
	}
	return &commonpb.GeometriesInFrame{
		ReferenceFrame: framedGeometries.frame,
		Geometries:     geometries,
	}
}

// ProtobufToGeometriesInFrame converts a GeometriesInFrame message as specified in common.proto to a GeometriesInFrame struct.
func ProtobufToGeometriesInFrame(proto *commonpb.GeometriesInFrame) (*GeometriesInFrame, error) {
	geometries := make(map[string]spatialmath.Geometry)
	for i, geometry := range proto.GetGeometries() {
		g, err := spatialmath.NewGeometryFromProto(geometry)
		if err != nil {
			return nil, err
		}
		geometries[strconv.Itoa(i)] = g
	}
	return &GeometriesInFrame{
		frame:      proto.GetReferenceFrame(),
		geometries: geometries,
	}, nil
}
