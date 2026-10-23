package referenceframe

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelToProto converts a model into the KinematicModel message. The message carries the limits
// the model enforces, which an override may have narrowed since its file was read, along with any
// visual geometries and properties set on it. A model built from frames with no configuration
// cannot be converted yet and returns an error.
func ModelToProto(m Model) (*commonpb.KinematicModel, error) {
	if m == nil {
		return nil, errors.New("cannot convert a nil model")
	}
	cfg := m.ModelConfig()
	if cfg == nil {
		return nil, fmt.Errorf("model %q has no configuration to convert", m.Name())
	}
	sva, err := svaConfig(cfg)
	if err != nil {
		return nil, err
	}
	pb, err := ModelConfigToProto(sva)
	if err != nil {
		return nil, err
	}
	pb.Name = m.Name()
	sm, ok := m.(*SimpleModel)
	if !ok {
		return pb, nil
	}
	// the frames hold the limits the model actually enforces, so they win over the configuration
	for i, joint := range pb.Joints {
		if joint.Mimic != nil {
			continue
		}
		frame := sm.internalFS.Frame(joint.GetId())
		if frame == nil || len(frame.DoF()) != 1 {
			continue
		}
		jointType, limits, err := liveLimitsToProto(&sva.Joints[i], frame.DoF()[0])
		if err != nil {
			return nil, errors.Wrapf(err, "joint %q", joint.GetId())
		}
		joint.Type, joint.Limits = jointType, limits
	}
	for _, link := range pb.Links {
		for i := range sm.visual[link.GetId()] {
			g, err := geometryConfigToProto(&sm.visual[link.GetId()][i])
			if err != nil {
				return nil, errors.Wrapf(err, "link %q visual geometry %d", link.GetId(), i)
			}
			link.Visual = append(link.Visual, g)
		}
	}
	pb.Properties = propertiesToProto(sm.properties)
	return pb, nil
}

// ModelFromProto builds a model from the KinematicModel message. An empty name keeps the name
// carried in the message. Visual geometries and properties are set on the model after its frames
// are built.
func ModelFromProto(pb *commonpb.KinematicModel, name string) (Model, error) {
	cfg, visual, err := modelConfigFromProto(pb)
	if err != nil {
		return nil, err
	}
	model, err := cfg.ParseConfig(name)
	if err != nil {
		return nil, err
	}
	sm, ok := model.(*SimpleModel)
	if !ok {
		return model, nil
	}
	for link, geometries := range visual {
		if err := sm.SetVisualGeometries(link, geometries); err != nil {
			return nil, err
		}
	}
	sm.SetKinematicProperties(propertiesFromProto(pb.GetProperties()))
	return sm, nil
}

// ModelConfigToProto converts a model configuration into the KinematicModel message. DH
// parameters are expanded into the joint and link each one stands for, since the message carries
// links and joints only.
func ModelConfigToProto(cfg *ModelConfigJSON) (*commonpb.KinematicModel, error) {
	if cfg == nil {
		return nil, errors.New("cannot convert a nil model config")
	}
	sva, err := svaConfig(cfg)
	if err != nil {
		return nil, err
	}
	pb := &commonpb.KinematicModel{
		Name:         sva.Name,
		OutputFrames: sva.OutputFrames,
	}
	for i := range sva.Links {
		pbLink, err := linkConfigToProto(&sva.Links[i])
		if err != nil {
			return nil, errors.Wrapf(err, "link %q", sva.Links[i].ID)
		}
		pb.Links = append(pb.Links, pbLink)
	}
	for i := range sva.Joints {
		pbJoint, err := jointConfigToProto(&sva.Joints[i])
		if err != nil {
			return nil, errors.Wrapf(err, "joint %q", sva.Joints[i].ID)
		}
		pb.Joints = append(pb.Joints, pbJoint)
	}
	return pb, nil
}

// ModelConfigFromProto converts the KinematicModel message into an SVA model configuration. The
// SVA schema has no place for visual geometry, so any is an error rather than a silent drop.
// ModelFromProto keeps it.
func ModelConfigFromProto(pb *commonpb.KinematicModel) (*ModelConfigJSON, error) {
	cfg, visual, err := modelConfigFromProto(pb)
	if err != nil {
		return nil, err
	}
	for link, geometries := range visual {
		return nil, fmt.Errorf("link %q: %d visual geometries cannot be carried by an SVA model config", link, len(geometries))
	}
	return cfg, nil
}

// modelConfigFromProto does the conversion and hands back the visual geometries by link id, so the
// caller decides whether having any is an error or something to set on the model.
func modelConfigFromProto(pb *commonpb.KinematicModel) (*ModelConfigJSON, map[string][]spatial.GeometryConfig, error) {
	if pb == nil {
		return nil, nil, errors.New("cannot convert a nil kinematic model")
	}
	cfg := &ModelConfigJSON{
		Name:         pb.GetName(),
		KinParamType: "SVA",
		OutputFrames: pb.GetOutputFrames(),
	}
	visual := map[string][]spatial.GeometryConfig{}
	for _, link := range pb.GetLinks() {
		lc, err := linkConfigFromProto(link)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "link %q", link.GetId())
		}
		cfg.Links = append(cfg.Links, *lc)
		for i, g := range link.GetVisual() {
			gc, err := geometryConfigFromProto(g)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "link %q visual geometry %d", link.GetId(), i)
			}
			visual[link.GetId()] = append(visual[link.GetId()], *gc)
		}
	}
	for _, joint := range pb.GetJoints() {
		jc, err := jointConfigFromProto(joint)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "joint %q", joint.GetId())
		}
		cfg.Joints = append(cfg.Joints, *jc)
	}
	return cfg, visual, nil
}

// svaConfig returns cfg with any DH parameters expanded into links and joints. The names match
// what ParseConfig builds for a DH file, a rotational frame named <id>_j and a static link named
// <id>, so the frames of a model read from either form line up.
func svaConfig(cfg *ModelConfigJSON) (*ModelConfigJSON, error) {
	switch cfg.KinParamType {
	case "SVA", "":
		return cfg, nil
	case "DH":
	default:
		return nil, errors.Errorf("unsupported param type: %s, supported params are SVA and DH", cfg.KinParamType)
	}
	sva := &ModelConfigJSON{Name: cfg.Name, KinParamType: "SVA", OutputFrames: cfg.OutputFrames}
	for _, dh := range cfg.DHParams {
		jointID := dh.ID + "_j"
		sva.Joints = append(sva.Joints, JointConfig{
			ID:     jointID,
			Type:   RevoluteJoint,
			Parent: dh.Parent,
			Axis:   spatial.AxisConfig{Z: 1},
			Min:    dh.Min,
			Max:    dh.Max,
		})
		pose := spatial.NewPoseFromDH(dh.A, dh.D, utils.DegToRad(dh.Alpha))
		orientation, err := spatial.NewOrientationConfig(pose.Orientation())
		if err != nil {
			return nil, errors.Wrapf(err, "dh parameter %q", dh.ID)
		}
		sva.Links = append(sva.Links, LinkConfig{
			ID:          dh.ID,
			Parent:      jointID,
			Translation: pose.Point(),
			Orientation: orientation,
			Geometry:    dh.Geometry,
		})
	}
	return sva, nil
}

func linkConfigToProto(link *LinkConfig) (*commonpb.KinematicLink, error) {
	pose := &commonpb.LinkPose{Translation: vectorToProto(link.Translation)}
	if link.Orientation != nil {
		if err := setOrientationOnProto(pose, link.Orientation); err != nil {
			return nil, err
		}
	}
	pbLink := &commonpb.KinematicLink{Id: link.ID, Parent: link.Parent, Pose: pose}
	configs := link.geometryConfigs()
	for i := range configs {
		g, err := geometryConfigToProto(&configs[i])
		if err != nil {
			return nil, err
		}
		// the wire carries the label the Go model would give it, so every reader sees the same name
		if g.Label == "" {
			g.Label = defaultGeometryLabel(link.ID, i, len(configs))
		}
		pbLink.Collision = append(pbLink.Collision, g)
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
	configs := make([]spatial.GeometryConfig, 0, len(link.GetCollision()))
	for i, g := range link.GetCollision() {
		gc, err := geometryConfigFromProto(g)
		if err != nil {
			return nil, errors.Wrapf(err, "collision geometry %d", i)
		}
		configs = append(configs, *gc)
	}
	lc.setGeometryConfigs(configs)
	return lc, nil
}

func jointConfigToProto(joint *JointConfig) (*commonpb.KinematicJoint, error) {
	pbJoint := &commonpb.KinematicJoint{
		Id:     joint.ID,
		Parent: joint.Parent,
		Axis:   vectorToProto(r3.Vector(joint.Axis)),
	}
	jointType, limits, err := jointLimitsToProto(joint.Type, joint.Min, joint.Max, joint.MaxVelocity, joint.MaxAcceleration)
	if err != nil {
		return nil, err
	}
	pbJoint.Type = jointType
	if joint.Mimic != nil {
		// a mimic joint follows its source and has no limits of its own, and we write the
		// multiplier out so a reader never has to know that zero means one
		pbJoint.Mimic = &commonpb.MimicJoint{
			Joint:      joint.Mimic.Joint,
			Multiplier: joint.Mimic.EffectiveMultiplier(),
			Offset:     joint.Mimic.ValueOffset,
		}
	} else {
		pbJoint.Limits = limits
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
	case commonpb.JointType_JOINT_TYPE_CONTINUOUS:
		// a revolute joint with no position bound, which is how the URDF reader has always
		// represented a continuous joint
		jc.Type = RevoluteJoint
	case commonpb.JointType_JOINT_TYPE_PRISMATIC:
		jc.Type = PrismaticJoint
	case commonpb.JointType_JOINT_TYPE_UNSPECIFIED:
		return nil, NewUnsupportedJointTypeError(joint.GetType().String())
	default:
		return nil, NewUnsupportedJointTypeError(joint.GetType().String())
	}
	if mimic := joint.GetMimic(); mimic != nil {
		jc.Mimic = &MimicConfig{
			Joint:           mimic.GetJoint(),
			ValueMultiplier: mimic.GetMultiplier(),
			ValueOffset:     mimic.GetOffset(),
		}
	} else {
		// an absent bound is an unbounded one
		jc.Min, jc.Max = math.Inf(-1), math.Inf(1)
		if limits := joint.GetLimits(); limits != nil {
			if limits.Min != nil {
				jc.Min = limits.GetMin()
			}
			if limits.Max != nil {
				jc.Max = limits.GetMax()
			}
			jc.MaxVelocity = copyFloat(limits.MaxVelocity)
			jc.MaxAcceleration = copyFloat(limits.MaxAcceleration)
		}
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

// jointLimitsToProto maps a joint type and its limits in document units, degrees or mm, onto the
// message. An infinite bound is left absent, and a revolute joint with neither bound is what the
// message calls continuous.
func jointLimitsToProto(jointType string, minimum, maximum float64, velocity, acceleration *float64,
) (commonpb.JointType, *commonpb.JointLimits, error) {
	limits := &commonpb.JointLimits{MaxVelocity: copyFloat(velocity), MaxAcceleration: copyFloat(acceleration)}
	if !math.IsInf(minimum, -1) {
		limits.Min = proto.Float64(minimum)
	}
	if !math.IsInf(maximum, 1) {
		limits.Max = proto.Float64(maximum)
	}
	switch jointType {
	case PrismaticJoint:
		return commonpb.JointType_JOINT_TYPE_PRISMATIC, limits, nil
	case RevoluteJoint, ContinuousJoint:
		if limits.Min == nil && limits.Max == nil {
			return commonpb.JointType_JOINT_TYPE_CONTINUOUS, limits, nil
		}
		return commonpb.JointType_JOINT_TYPE_REVOLUTE, limits, nil
	default:
		return commonpb.JointType_JOINT_TYPE_UNSPECIFIED, nil, NewUnsupportedJointTypeError(jointType)
	}
}

// liveLimitsToProto maps the limit a frame enforces, radians for a revolute joint, back into the
// document units the message uses.
func liveLimitsToProto(joint *JointConfig, limit Limit) (commonpb.JointType, *commonpb.JointLimits, error) {
	if joint.Type == PrismaticJoint {
		return jointLimitsToProto(joint.Type, limit.Min, limit.Max, limit.MaxVelocity, limit.MaxAcceleration)
	}
	return jointLimitsToProto(joint.Type, utils.RadToDeg(limit.Min), utils.RadToDeg(limit.Max),
		limitPtr(limit.MaxVelocity, utils.RadToDeg), limitPtr(limit.MaxAcceleration, utils.RadToDeg))
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
		// identity, so the oneof stays unset
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

// geometryConfigToProto maps a geometry by hand. Going through spatialmath.Geometry would
// re-encode every mesh as PLY and drop the path it was read from.
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

	// a file may leave the type out and have it inferred from which dimensions are set, in the
	// same order GeometryConfig.ParseConfig uses
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
		mesh := &commonpb.Mesh{ContentType: gc.MeshContentType, Mesh: gc.MeshData}
		if gc.MeshFilePath != "" {
			mesh.OriginalFileSourcePathDiagnostic = proto.String(gc.MeshFilePath)
		}
		pb.GeometryType = &commonpb.Geometry_Mesh{Mesh: mesh}
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
		// an unset orientation and the identity both need no config, and leaving one out keeps a
		// geometry written without an orientation unchanged after a round trip
		identity := c.GetOX() == 0 && c.GetOY() == 0 && c.GetOZ() >= 0 && c.GetTheta() == 0
		if !identity {
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
		gc.MeshFilePath = g.Mesh.GetOriginalFileSourcePathDiagnostic()
	default:
		return nil, fmt.Errorf("geometry %T cannot be carried by an SVA model config", g)
	}
	return gc, nil
}

// propertiesToProto leaves a zero property off the wire, which is how the message says unset.
func propertiesToProto(p KinematicProperties) *commonpb.KinematicProperties {
	if p == (KinematicProperties{}) {
		return nil
	}
	pb := &commonpb.KinematicProperties{}
	if p.OperatingFreqHz != 0 {
		pb.OperatingFreqHz = proto.Float64(p.OperatingFreqHz)
	}
	if p.MaxLinearTCPSpeedMMPerSec != 0 {
		pb.MaxLinearTcpSpeedMmPerSec = proto.Float64(p.MaxLinearTCPSpeedMMPerSec)
	}
	return pb
}

func propertiesFromProto(pb *commonpb.KinematicProperties) KinematicProperties {
	return KinematicProperties{
		OperatingFreqHz:           pb.GetOperatingFreqHz(),
		MaxLinearTCPSpeedMMPerSec: pb.GetMaxLinearTcpSpeedMmPerSec(),
	}
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
