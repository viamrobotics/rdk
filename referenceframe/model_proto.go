package referenceframe

import (
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelToProto converts a model into the typed KinematicModel message. It works from the
// model's parsed configuration rather than from its frames, so a model that was built in code
// without one cannot be converted yet; that is the typed-first step this prototype leaves out.
func ModelToProto(m Model) (*commonpb.KinematicModel, error) {
	if m == nil {
		return nil, errors.New("cannot convert a nil model")
	}
	cfg := m.ModelConfig()
	if cfg == nil {
		return nil, fmt.Errorf("model %q has no configuration to convert", m.Name())
	}
	return ModelConfigToProto(cfg)
}

// ModelFromProto builds a model from the typed KinematicModel message. An empty name keeps the
// name carried in the message.
func ModelFromProto(pb *commonpb.KinematicModel, name string) (Model, error) {
	cfg, err := ModelConfigFromProto(pb)
	if err != nil {
		return nil, err
	}
	return cfg.ParseConfig(name)
}

// ModelConfigToProto converts an SVA model configuration into the typed message. DH
// configurations are refused, since the message carries links and joints only; expanding DH
// parameters into those belongs to the file to schema helper, not here.
func ModelConfigToProto(cfg *ModelConfigJSON) (*commonpb.KinematicModel, error) {
	if cfg == nil {
		return nil, errors.New("cannot convert a nil model config")
	}
	if cfg.KinParamType != "" && cfg.KinParamType != "SVA" {
		return nil, fmt.Errorf("kinematic_param_type %q cannot be converted, only SVA models can", cfg.KinParamType)
	}
	pb := &commonpb.KinematicModel{
		Name:         cfg.Name,
		OutputFrames: cfg.OutputFrames,
	}
	for _, link := range cfg.Links {
		pbLink, err := linkConfigToProto(&link)
		if err != nil {
			return nil, errors.Wrapf(err, "link %q", link.ID)
		}
		pb.Links = append(pb.Links, pbLink)
	}
	for _, joint := range cfg.Joints {
		pbJoint, err := jointConfigToProto(&joint)
		if err != nil {
			return nil, errors.Wrapf(err, "joint %q", joint.ID)
		}
		pb.Joints = append(pb.Joints, pbJoint)
	}
	return pb, nil
}

// ModelConfigFromProto converts the typed message back into an SVA model configuration. The v1
// configuration is narrower than the message, so a link with more than one collision geometry,
// any visual geometry, user limits, or an unbounded position limit is an error rather than a
// silent drop.
func ModelConfigFromProto(pb *commonpb.KinematicModel) (*ModelConfigJSON, error) {
	if pb == nil {
		return nil, errors.New("cannot convert a nil kinematic model")
	}
	cfg := &ModelConfigJSON{
		Name:         pb.GetName(),
		KinParamType: "SVA",
		OutputFrames: pb.GetOutputFrames(),
	}
	for _, link := range pb.GetLinks() {
		lc, err := linkConfigFromProto(link)
		if err != nil {
			return nil, errors.Wrapf(err, "link %q", link.GetId())
		}
		cfg.Links = append(cfg.Links, *lc)
	}
	for _, joint := range pb.GetJoints() {
		jc, err := jointConfigFromProto(joint)
		if err != nil {
			return nil, errors.Wrapf(err, "joint %q", joint.GetId())
		}
		cfg.Joints = append(cfg.Joints, *jc)
	}
	return cfg, nil
}

func linkConfigToProto(link *LinkConfig) (*commonpb.KinematicLink, error) {
	pose := &commonpb.LinkPose{Translation: vectorToProto(link.Translation)}
	if link.Orientation != nil && link.Orientation.Type != spatial.NoOrientationType {
		if err := setOrientationOnProto(pose, link.Orientation); err != nil {
			return nil, err
		}
	}
	pbLink := &commonpb.KinematicLink{Id: link.ID, Parent: link.Parent, Pose: pose}
	if link.Geometry != nil {
		g, err := geometryConfigToProto(link.Geometry)
		if err != nil {
			return nil, err
		}
		pbLink.Collision = []*commonpb.Geometry{g}
	}
	return pbLink, nil
}

func linkConfigFromProto(link *commonpb.KinematicLink) (*LinkConfig, error) {
	lc := &LinkConfig{ID: link.GetId(), Parent: link.GetParent()}
	if pose := link.GetPose(); pose != nil {
		lc.Translation = vectorFromProto(pose.GetTranslation())
		oc, err := orientationFromProto(pose)
		if err != nil {
			return nil, err
		}
		lc.Orientation = oc
	}
	if n := len(link.GetVisual()); n > 0 {
		return nil, fmt.Errorf("%d visual geometries cannot be carried by a v1 model config", n)
	}
	switch n := len(link.GetCollision()); n {
	case 0:
	case 1:
		gc, err := geometryConfigFromProto(link.GetCollision()[0])
		if err != nil {
			return nil, err
		}
		lc.Geometry = gc
	default:
		return nil, fmt.Errorf("%d collision geometries cannot be carried by a v1 model config, which holds one per link", n)
	}
	return lc, nil
}

func jointConfigToProto(joint *JointConfig) (*commonpb.KinematicJoint, error) {
	pbJoint := &commonpb.KinematicJoint{
		Id:     joint.ID,
		Parent: joint.Parent,
		Axis:   vectorToProto(r3.Vector(joint.Axis)),
	}
	switch joint.Type {
	case RevoluteJoint:
		pbJoint.Type = commonpb.JointType_JOINT_TYPE_REVOLUTE
	case PrismaticJoint:
		pbJoint.Type = commonpb.JointType_JOINT_TYPE_PRISMATIC
	default:
		return nil, NewUnsupportedJointTypeError(joint.Type)
	}
	if joint.Mimic != nil {
		// a mimic joint follows its source, so it carries no limits of its own, and we write
		// the multiplier out explicitly rather than relying on the reader knowing zero means one
		pbJoint.Mimic = &commonpb.MimicJoint{
			Joint:      joint.Mimic.Joint,
			Multiplier: joint.Mimic.EffectiveMultiplier(),
			Offset:     joint.Mimic.ValueOffset,
		}
	} else {
		pbJoint.HardwareLimits = &commonpb.JointLimits{
			Min:             proto.Float64(joint.Min),
			Max:             proto.Float64(joint.Max),
			MaxVelocity:     copyFloat(joint.MaxVelocity),
			MaxAcceleration: copyFloat(joint.MaxAcceleration),
		}
	}
	if joint.Geometry != nil {
		g, err := geometryConfigToProto(joint.Geometry)
		if err != nil {
			return nil, err
		}
		pbJoint.Geometry = g
	}
	return pbJoint, nil
}

func jointConfigFromProto(joint *commonpb.KinematicJoint) (*JointConfig, error) {
	jc := &JointConfig{
		ID:     joint.GetId(),
		Parent: joint.GetParent(),
		Axis:   spatial.AxisConfig(vectorFromProto(joint.GetAxis())),
	}
	switch joint.GetType() {
	case commonpb.JointType_JOINT_TYPE_REVOLUTE:
		jc.Type = RevoluteJoint
	case commonpb.JointType_JOINT_TYPE_PRISMATIC:
		jc.Type = PrismaticJoint
	case commonpb.JointType_JOINT_TYPE_UNSPECIFIED:
		return nil, NewUnsupportedJointTypeError(joint.GetType().String())
	default:
		return nil, NewUnsupportedJointTypeError(joint.GetType().String())
	}
	if joint.GetUserLimits() != nil {
		return nil, errors.New("user limits cannot be carried by a v1 model config")
	}
	if mimic := joint.GetMimic(); mimic != nil {
		jc.Mimic = &MimicConfig{
			Joint:           mimic.GetJoint(),
			ValueMultiplier: mimic.GetMultiplier(),
			ValueOffset:     mimic.GetOffset(),
		}
	} else {
		hw := joint.GetHardwareLimits()
		if hw == nil || hw.Min == nil || hw.Max == nil {
			return nil, errors.New("an unbounded position cannot be carried by a v1 model config")
		}
		jc.Min = hw.GetMin()
		jc.Max = hw.GetMax()
		jc.MaxVelocity = copyFloat(hw.MaxVelocity)
		jc.MaxAcceleration = copyFloat(hw.MaxAcceleration)
	}
	if joint.GetGeometry() != nil {
		gc, err := geometryConfigFromProto(joint.GetGeometry())
		if err != nil {
			return nil, err
		}
		jc.Geometry = gc
	}
	return jc, nil
}

// setOrientationOnProto keeps the representation the file author chose. Only the units change,
// since the message carries every angle in degrees.
func setOrientationOnProto(pose *commonpb.LinkPose, oc *spatial.OrientationConfig) error {
	o, err := oc.ParseConfig()
	if err != nil {
		return err
	}
	switch oc.Type {
	case spatial.NoOrientationType:
		// identity, and the oneof stays unset
		return nil
	case spatial.QuaternionType:
		q := o.Quaternion()
		pose.Orientation = &commonpb.LinkPose_Quaternion{Quaternion: &commonpb.Quaternion{
			W: q.Real, X: q.Imag, Y: q.Jmag, Z: q.Kmag,
		}}
	case spatial.OrientationVectorDegreesType, spatial.OrientationVectorRadiansType:
		ov := o.OrientationVectorDegrees()
		pose.Orientation = &commonpb.LinkPose_OrientationVector{OrientationVector: &commonpb.Orientation{
			OX: ov.OX, OY: ov.OY, OZ: ov.OZ, Theta: ov.Theta,
		}}
	case spatial.EulerAnglesType:
		e := o.EulerAngles()
		pose.Orientation = &commonpb.LinkPose_EulerAngles{EulerAngles: &commonpb.EulerAngles{
			Roll: utils.RadToDeg(e.Roll), Pitch: utils.RadToDeg(e.Pitch), Yaw: utils.RadToDeg(e.Yaw),
		}}
	case spatial.AxisAnglesType:
		aa := o.AxisAngles()
		pose.Orientation = &commonpb.LinkPose_AxisAngles{AxisAngles: &commonpb.AxisAngles{
			Rx: aa.RX, Ry: aa.RY, Rz: aa.RZ, Theta: utils.RadToDeg(aa.Theta),
		}}
	default:
		return fmt.Errorf("orientation type %q cannot be converted", oc.Type)
	}
	return nil
}

func orientationFromProto(pose *commonpb.LinkPose) (*spatial.OrientationConfig, error) {
	var o spatial.Orientation
	switch v := pose.GetOrientation().(type) {
	case nil:
		return nil, nil
	case *commonpb.LinkPose_Quaternion:
		q := v.Quaternion
		o = &spatial.Quaternion{Real: q.GetW(), Imag: q.GetX(), Jmag: q.GetY(), Kmag: q.GetZ()}
	case *commonpb.LinkPose_OrientationVector:
		ov := v.OrientationVector
		o = &spatial.OrientationVectorDegrees{Theta: ov.GetTheta(), OX: ov.GetOX(), OY: ov.GetOY(), OZ: ov.GetOZ()}
	case *commonpb.LinkPose_EulerAngles:
		e := v.EulerAngles
		o = &spatial.EulerAngles{Roll: utils.DegToRad(e.GetRoll()), Pitch: utils.DegToRad(e.GetPitch()), Yaw: utils.DegToRad(e.GetYaw())}
	case *commonpb.LinkPose_AxisAngles:
		aa := v.AxisAngles
		o = &spatial.R4AA{Theta: utils.DegToRad(aa.GetTheta()), RX: aa.GetRx(), RY: aa.GetRy(), RZ: aa.GetRz()}
	default:
		return nil, fmt.Errorf("orientation %T cannot be converted", v)
	}
	return spatial.NewOrientationConfig(o)
}

// geometryConfigToProto maps a geometry by hand. Going through spatialmath.Geometry would lose
// a mesh that is referenced by path only, and Cylinder has no proto form at all.
func geometryConfigToProto(gc *spatial.GeometryConfig) (*commonpb.Geometry, error) {
	center := &commonpb.Pose{X: gc.TranslationOffset.X, Y: gc.TranslationOffset.Y, Z: gc.TranslationOffset.Z, OZ: 1}
	if gc.OrientationOffset.Type != spatial.NoOrientationType {
		o, err := gc.OrientationOffset.ParseConfig()
		if err != nil {
			return nil, err
		}
		ov := o.OrientationVectorDegrees()
		center.OX, center.OY, center.OZ, center.Theta = ov.OX, ov.OY, ov.OZ, ov.Theta
	}
	pb := &commonpb.Geometry{Center: center, Label: gc.Label}

	// v1 lets a file leave the type out and infers it from which dimensions are set, in the same
	// order GeometryConfig.ParseConfig uses
	gType := gc.Type
	if gType == spatial.UnknownType {
		switch {
		case gc.X != 0 || gc.Y != 0 || gc.Z != 0:
			gType = spatial.BoxType
		case gc.L != 0:
			gType = spatial.CapsuleType
		case gc.R != 0:
			gType = spatial.SphereType
		default:
			return nil, errors.New("geometry has neither a type nor dimensions")
		}
	}
	switch gType {
	case spatial.BoxType:
		pb.GeometryType = &commonpb.Geometry_Box{Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: gc.X, Y: gc.Y, Z: gc.Z}}}
	case spatial.SphereType:
		pb.GeometryType = &commonpb.Geometry_Sphere{Sphere: &commonpb.Sphere{RadiusMm: gc.R}}
	case spatial.CapsuleType:
		pb.GeometryType = &commonpb.Geometry_Capsule{Capsule: &commonpb.Capsule{RadiusMm: gc.R, LengthMm: gc.L}}
	case spatial.MeshType:
		pb.GeometryType = &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{
			ContentType: gc.MeshContentType,
			Mesh:        gc.MeshData,
			SourcePath:  gc.MeshFilePath,
		}}
	case spatial.CylinderType, spatial.PointType, spatial.UnknownType:
		return nil, fmt.Errorf("geometry type %q has no representation in common.v1.Geometry", gType)
	default:
		return nil, fmt.Errorf("geometry type %q has no representation in common.v1.Geometry", gType)
	}
	return pb, nil
}

func geometryConfigFromProto(pb *commonpb.Geometry) (*spatial.GeometryConfig, error) {
	gc := &spatial.GeometryConfig{Label: pb.GetLabel()}
	if c := pb.GetCenter(); c != nil {
		gc.TranslationOffset = r3.Vector{X: c.GetX(), Y: c.GetY(), Z: c.GetZ()}
		// an all zero axis is an unset orientation, and identity needs no config at all
		if c.GetOX() != 0 || c.GetOY() != 0 || c.GetOZ() != 0 {
			oc, err := spatial.NewOrientationConfig(&spatial.OrientationVectorDegrees{
				Theta: c.GetTheta(), OX: c.GetOX(), OY: c.GetOY(), OZ: c.GetOZ(),
			})
			if err != nil {
				return nil, err
			}
			gc.OrientationOffset = *oc
		}
	}
	switch g := pb.GetGeometryType().(type) {
	case *commonpb.Geometry_Box:
		gc.Type = spatial.BoxType
		d := g.Box.GetDimsMm()
		gc.X, gc.Y, gc.Z = d.GetX(), d.GetY(), d.GetZ()
	case *commonpb.Geometry_Sphere:
		gc.Type = spatial.SphereType
		gc.R = g.Sphere.GetRadiusMm()
	case *commonpb.Geometry_Capsule:
		gc.Type = spatial.CapsuleType
		gc.R = g.Capsule.GetRadiusMm()
		gc.L = g.Capsule.GetLengthMm()
	case *commonpb.Geometry_Mesh:
		gc.Type = spatial.MeshType
		gc.MeshData = g.Mesh.GetMesh()
		gc.MeshContentType = g.Mesh.GetContentType()
		gc.MeshFilePath = g.Mesh.GetSourcePath()
	default:
		return nil, fmt.Errorf("geometry %T cannot be carried by a v1 model config", g)
	}
	return gc, nil
}

func vectorToProto(v r3.Vector) *commonpb.Vector3 {
	return &commonpb.Vector3{X: v.X, Y: v.Y, Z: v.Z}
}

func vectorFromProto(v *commonpb.Vector3) r3.Vector {
	return r3.Vector{X: v.GetX(), Y: v.GetY(), Z: v.GetZ()}
}

func copyFloat(f *float64) *float64 {
	if f == nil {
		return nil
	}
	return proto.Float64(*f)
}
