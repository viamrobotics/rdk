package referenceframe

import (
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/spatialmath"
)

type Transformable interface {
	Transform(*PoseInFrame) Transformable
	FrameName() string
	AlmostEqual(Transformable) bool
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

func (pF *PoseInFrame) Transform(tf *PoseInFrame) Transformable {
	return NewPoseInFrame(tf.frame, spatialmath.Compose(tf.pose, pF.pose))
}

func (pF *PoseInFrame) AlmostEqual(other Transformable) bool {
	if pF2, ok := other.(*PoseInFrame); ok {
		return pF.FrameName() == pF2.FrameName() && spatialmath.PoseAlmostEqual(pF.Pose(), pF2.Pose())
	}
	return false
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
	geometries []spatialmath.Geometry
}

// FrameName returns the name of the frame in which the geometries were observed.
func (gF *GeometriesInFrame) FrameName() string {
	return gF.frame
}

// Geometries returns the geometries observed.
func (gF *GeometriesInFrame) Geometries() []spatialmath.Geometry {
	return gF.geometries
}

func (gF *GeometriesInFrame) Transform(tf *PoseInFrame) Transformable {
	var geometries []spatialmath.Geometry
	for _, geometry := range gF.geometries {
		geometries = append(geometries, geometry.Transform(tf.pose))
	}
	return NewGeometriesInFrame(tf.frame, geometries)
}

func (gF *GeometriesInFrame) AlmostEqual(other Transformable) bool {
	if gF2, ok := other.(*GeometriesInFrame); ok {
		gs := gF.Geometries()
		gs2 := gF2.Geometries()
		if len(gs) != len(gs2) {
			return false
		}
		for i := 0; i < len(gs); i++ {
			if !gs[i].AlmostEqual(gs2[i]) {
				return false
			}
		}
		return gF.FrameName() == gF2.FrameName()
	}
	return false
}

// NewGeometriesInFrame generates a new GeometriesInFrame.
func NewGeometriesInFrame(frame string, geometries []spatialmath.Geometry) *GeometriesInFrame {
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
	var geometries []spatialmath.Geometry
	for _, geometry := range proto.GetGeometries() {
		g, err := spatialmath.NewGeometryFromProto(geometry)
		if err != nil {
			return nil, err
		}
		geometries = append(geometries, g)
	}
	return &GeometriesInFrame{
		frame:      proto.GetReferenceFrame(),
		geometries: geometries,
	}, nil
}
