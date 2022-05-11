package referenceframe

import (
	"strconv"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
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
}

// FrameName returns the name of the frame in which the pose was observed.
func (pF *PoseInFrame) FrameName() string {
	return pF.frame
}

// Pose returns the pose that was observed.
func (pF *PoseInFrame) Pose() spatialmath.Pose {
	return pF.pose
}

// Transform changes the PoseInFrame pF into the reference frame specified by the tf argument.
// The tf PoseInFrame represents the pose of the pF reference frame with respect to the destination reference frame
func (pF *PoseInFrame) Transform(tf *PoseInFrame) Transformable {
	return NewPoseInFrame(tf.frame, spatialmath.Compose(tf.pose, pF.pose))
}

// NewPoseInFrame generates a new PoseInFrame.
func NewPoseInFrame(frame string, pose spatialmath.Pose) *PoseInFrame {
	return &PoseInFrame{
		frame: frame,
		pose:  pose,
	}
}

// PoseInFrameToProtobuf converts a PoseInFrame struct to a
// PoseInFrame message as specified in common.proto.
func PoseInFrameToProtobuf(framedPose *PoseInFrame) *commonpb.PoseInFrame {
	poseProto := spatialmath.PoseToProtobuf(framedPose.pose)
	return &commonpb.PoseInFrame{
		ReferenceFrame: framedPose.frame,
		Pose:           poseProto,
	}
}

// ProtobufToPoseInFrame converts a PoseInFrame message as specified in
// common.proto to a PoseInFrame struct.
func ProtobufToPoseInFrame(proto *commonpb.PoseInFrame) *PoseInFrame {
	result := &PoseInFrame{}
	result.pose = spatialmath.NewPoseFromProtobuf(proto.GetPose())
	result.frame = proto.GetReferenceFrame()
	return result
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
// The tf PoseInFrame represents the pose of the gF reference frame with respect to the destination reference frame
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
