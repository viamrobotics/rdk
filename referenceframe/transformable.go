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
	FrameName() string
}

// PoseInFrame is a data structure that packages a pose with the name of the
// frame in which it was observed.
type PoseInFrame struct {
	frame string
	pose  spatialmath.Pose
	name  string
}

// FrameName returns the name of the frame in which the pose was observed.
func (pF *PoseInFrame) FrameName() string {
	return pF.frame
}

// Pose returns the pose that was observed.
func (pF *PoseInFrame) Pose() spatialmath.Pose {
	return pF.pose
}

// Name returns the name of the PoseInFrame.
func (pF *PoseInFrame) Name() string {
	return pF.name
}

// Transform changes the PoseInFrame pF into the reference frame specified by the tf argument.
// The tf PoseInFrame represents the pose of the pF reference frame with respect to the destination reference frame.
func (pF *PoseInFrame) Transform(tf *PoseInFrame) Transformable {
	return NewPoseInFrame(tf.frame, spatialmath.Compose(tf.pose, pF.pose))
}

// NewPoseInFrame generates a new PoseInFrame.
func NewPoseInFrame(frame string, pose spatialmath.Pose) *PoseInFrame {
	// fmt.Println("frame: ", frame)
	return &PoseInFrame{
		frame: frame,
		pose:  pose,
	}
}

// NewNamedPoseInFrame generates a new PoseInFrame and gives it the specified name.
func NewNamedPoseInFrame(frame string, pose spatialmath.Pose, name string) *PoseInFrame {
	return &PoseInFrame{
		frame: frame,
		pose:  pose,
		name:  name,
	}
}

// PoseInFrameToProtobuf converts a PoseInFrame struct to a PoseInFrame protobuf message.
func PoseInFrameToProtobuf(framedPose *PoseInFrame) *commonpb.PoseInFrame {
	poseProto := spatialmath.PoseToProtobuf(framedPose.pose)
	return &commonpb.PoseInFrame{
		ReferenceFrame: framedPose.frame,
		Pose:           poseProto,
	}
}

// ProtobufToPoseInFrame converts a PoseInFrame protobuf message to a PoseInFrame struct.
func ProtobufToPoseInFrame(proto *commonpb.PoseInFrame) *PoseInFrame {
	result := &PoseInFrame{}
	result.pose = spatialmath.NewPoseFromProtobuf(proto.GetPose())
	result.frame = proto.GetReferenceFrame()
	return result
}

// PoseInFrameToTransformProtobuf converts a PoseInFrame struct to a Transform protobuf message.
func PoseInFrameToTransformProtobuf(framedPose *PoseInFrame) (*commonpb.Transform, error) {
	if framedPose.name == "" {
		return nil, ErrEmptyStringFrameName
	}
	return &commonpb.Transform{
		ReferenceFrame:      framedPose.name,
		PoseInObserverFrame: PoseInFrameToProtobuf(framedPose),
	}, nil
}

// PoseInFrameFromTransformProtobuf converts a Transform protobuf message to a PoseInFrame struct.
func PoseInFrameFromTransformProtobuf(proto *commonpb.Transform) (*PoseInFrame, error) {
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
	return NewNamedPoseInFrame(parentFrame, pose, frameName), nil
}

// PoseInFramesToTransformProtobuf converts a slice of PoseInFrame structs to a slice of Transform protobuf messages.
// TODO(rb): use generics to operate on lists of arbirary types.
func PoseInFramesToTransformProtobuf(poseSlice []*PoseInFrame) ([]*commonpb.Transform, error) {
	protoTransforms := make([]*commonpb.Transform, 0, len(poseSlice))
	for i, transform := range poseSlice {
		protoTf, err := PoseInFrameToTransformProtobuf(transform)
		if err != nil {
			return nil, errors.Wrapf(err, "conversion error at index %d", i)
		}
		protoTransforms = append(protoTransforms, protoTf)
	}
	return protoTransforms, nil
}

// PoseInFramesFromTransformProtobuf converts a slice of Transform protobuf messages to a slice of PoseInFrame structs.
// TODO(rb): use generics to operate on lists of arbirary proto types.
func PoseInFramesFromTransformProtobuf(protoSlice []*commonpb.Transform) ([]*PoseInFrame, error) {
	transforms := make([]*PoseInFrame, 0, len(protoSlice))
	for i, protoTransform := range protoSlice {
		transform, err := PoseInFrameFromTransformProtobuf(protoTransform)
		if err != nil {
			return nil, errors.Wrapf(err, "conversion error at index %d", i)
		}
		transforms = append(transforms, transform)
	}
	return transforms, nil
}

// GeometriesInFrame is a data structure that packages geometries with the name of the frame in which it was observed.
type GeometriesInFrame struct {
	frame      string
	geometries map[string]spatialmath.Geometry
}

// FrameName returns the name of the frame in which the geometries were observed.
func (gF *GeometriesInFrame) FrameName() string {
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
	return NewGeometriesInFrame(tf.frame, geometries)
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
