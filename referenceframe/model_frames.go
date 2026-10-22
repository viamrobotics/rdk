package referenceframe

import (
	"fmt"

	"github.com/pkg/errors"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// configFromFrames describes a model that was built from frames in code the way a kinematics
// file would, so the converters and the deprecated bytes serve it like a model read from a file.
// Each static frame becomes a link, each rotational or translational frame a joint, and a mimic
// mapping a mimic joint. A pose frame has no file form and is refused.
func (m *SimpleModel) configFromFrames() (*ModelConfigJSON, error) {
	cfg := &ModelConfigJSON{Name: m.name, KinParamType: "SVA"}
	if m.primaryOutputFrame != "" {
		cfg.OutputFrames = []string{m.primaryOutputFrame}
	}
	for _, name := range bfsFrameNames(m.internalFS) {
		frame := m.internalFS.Frame(name)
		if frame == nil {
			continue
		}
		parent := m.internalFS.parents[name]
		switch f := unwrapNamed(frame).(type) {
		case *staticFrame, *tailGeometryStaticFrame:
			lc, err := NewLinkConfig(frame)
			if err != nil {
				return nil, errors.Wrapf(err, "frame %q", name)
			}
			lc.ID, lc.Parent = name, parent
			cfg.Links = append(cfg.Links, *lc)
		case *rotationalFrame:
			jc, err := frameJointConfig(
				name, parent, RevoluteJoint, f.rotAxis.X, f.rotAxis.Y, f.rotAxis.Z, frame.DoF(), utils.RadToDeg,
			)
			if err != nil {
				return nil, err
			}
			m.setMimic(name, jc)
			cfg.Joints = append(cfg.Joints, *jc)
		case *translationalFrame:
			jc, err := frameJointConfig(
				name, parent, PrismaticJoint, f.transAxis.X, f.transAxis.Y, f.transAxis.Z, frame.DoF(), nil,
			)
			if err != nil {
				return nil, err
			}
			// a prismatic joint in a file has room for one geometry
			if len(f.geometries) > 1 {
				return nil, fmt.Errorf("joint %q has %d geometries and a kinematics file joint holds one", name, len(f.geometries))
			}
			if len(f.geometries) == 1 {
				gc, err := spatial.NewGeometryConfig(f.geometries[0])
				if err != nil {
					return nil, errors.Wrapf(err, "joint %q", name)
				}
				jc.Geometry = gc
			}
			m.setMimic(name, jc)
			cfg.Joints = append(cfg.Joints, *jc)
		default:
			return nil, fmt.Errorf("frame %q is a %T, which has no kinematics file form", name, f)
		}
	}
	// a model with no frames has nothing to describe, and a message or a file with no links in
	// it does not parse, so a reader is better off with the empty response it always got
	if len(cfg.Links) == 0 && len(cfg.Joints) == 0 {
		return nil, errors.New("model has no frames")
	}
	return cfg, nil
}

// frameJointConfig writes a one degree of freedom frame as a file joint. The limits keep an
// infinite bound as it is, since the converters read that as unbounded, and convert turns the
// frame's radians into the degrees a file carries for a revolute joint.
func frameJointConfig(
	name, parent, jointType string, x, y, z float64, limits []Limit, convert func(float64) float64,
) (*JointConfig, error) {
	if len(limits) != 1 {
		return nil, fmt.Errorf("joint %q has %d degrees of freedom and a kinematics file joint has one", name, len(limits))
	}
	l := limits[0]
	jc := &JointConfig{ID: name, Parent: parent, Type: jointType, Axis: spatial.AxisConfig{x, y, z}, Min: l.Min, Max: l.Max}
	if convert != nil {
		jc.Min, jc.Max = convert(l.Min), convert(l.Max)
	}
	jc.MaxVelocity = limitPtr(l.MaxVelocity, convert)
	jc.MaxAcceleration = limitPtr(l.MaxAcceleration, convert)
	return jc, nil
}

// setMimic marks the joint as following another when the model drives it that way. A mimic joint
// has no limits of its own in a file, its source's apply.
func (m *SimpleModel) setMimic(name string, jc *JointConfig) {
	mm := m.mimicMappings[name]
	if mm == nil {
		return
	}
	jc.Min, jc.Max, jc.MaxVelocity, jc.MaxAcceleration = 0, 0, nil, nil
	jc.Mimic = &MimicConfig{Joint: mm.sourceFrameName, ValueMultiplier: mm.valueMultiplier, ValueOffset: mm.valueOffset}
}

func unwrapNamed(f Frame) Frame {
	for {
		nf, ok := f.(*namedFrame)
		if !ok {
			return f
		}
		f = nf.Frame
	}
}
