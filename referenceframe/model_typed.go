package referenceframe

import (
	"fmt"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/utils"
)

// SetUserLimits records what this component's configuration asks of each joint, keyed by joint
// id, in the document's units, degrees for revolute joints and millimetres for prismatic ones.
// Every value has to sit inside the hardware limit the model was built with, and a value that
// widens it is an error, since a widened limit is the one that reaches hardware. DoF() reports
// the effective limit afterwards, the user value where one is set and the hardware value
// otherwise, so planners see the tighter bound without knowing which layer it came from. Calling
// it again replaces the previous set rather than narrowing further.
func (m *SimpleModel) SetUserLimits(limits map[string]JointLimits) error {
	if m.modelConfig == nil {
		return errors.New("user limits need a model built from a configuration, so the hardware limits are known")
	}
	joints := map[string]*JointConfig{}
	for i := range m.modelConfig.Joints {
		joints[m.modelConfig.Joints[i].ID] = &m.modelConfig.Joints[i]
	}
	for id, ul := range limits {
		jc, ok := joints[id]
		if !ok {
			return fmt.Errorf("joint %q does not exist in model %q", id, m.name)
		}
		if jc.Mimic != nil {
			return fmt.Errorf("joint %q is a mimic joint and follows %q, it cannot carry limits of its own", id, jc.Mimic.Joint)
		}
		if err := ul.validate(id); err != nil {
			return err
		}
		if err := checkInsideHardware(id, ul, jc); err != nil {
			return err
		}
	}

	// rebuild every joint's effective limit from the hardware baseline, so a second call cannot
	// accumulate on top of the first
	for _, jc := range joints {
		frame := m.internalFS.Frame(jc.ID)
		if frame == nil || len(frame.DoF()) == 0 {
			continue
		}
		hardware, err := jc.ToFrame()
		if err != nil {
			return err
		}
		effective := hardware.DoF()[0]
		if ul, ok := limits[jc.ID]; ok {
			effective = applyUserLimit(effective, ul, jc.Type == RevoluteJoint)
		}
		frame.DoF()[0] = effective
	}
	m.limits = m.inputSchema.GetLimits()

	m.userLimits = make(map[string]JointLimits, len(limits))
	for id, ul := range limits {
		m.userLimits[id] = ul.clone()
	}
	return nil
}

// UserLimits returns a copy of the user limits set on this model, keyed by joint id, in document
// units. Joints with no user limit are absent.
func (m *SimpleModel) UserLimits() map[string]JointLimits {
	out := make(map[string]JointLimits, len(m.userLimits))
	for id, ul := range m.userLimits {
		out[id] = ul.clone()
	}
	return out
}

// SetVisualGeometries attaches display geometry to a link. Nothing in RDK reads it; it travels on
// the wire for viewers. Meshes here may be any format a viewer can load, unlike collision meshes.
func (m *SimpleModel) SetVisualGeometries(link string, geometries []*commonpb.Geometry) error {
	if m.modelConfig == nil {
		return errors.New("visual geometries need a model built from a configuration")
	}
	found := false
	for _, lc := range m.modelConfig.Links {
		if lc.ID == link {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("link %q does not exist in model %q", link, m.name)
	}
	if m.visual == nil {
		m.visual = map[string][]*commonpb.Geometry{}
	}
	cloned := make([]*commonpb.Geometry, 0, len(geometries))
	for _, g := range geometries {
		if cloned0, ok := proto.Clone(g).(*commonpb.Geometry); ok {
			cloned = append(cloned, cloned0)
		}
	}
	m.visual[link] = cloned
	return nil
}

// SetKinematicProperties records the component level properties, such as the trajectory
// sampling frequency, that describe the component as a whole rather than any joint.
func (m *SimpleModel) SetKinematicProperties(p *commonpb.KinematicProperties) {
	if p == nil {
		m.properties = nil
		return
	}
	if cloned, ok := proto.Clone(p).(*commonpb.KinematicProperties); ok {
		m.properties = cloned
	}
}

// KinematicProperties returns the component level properties, or nil when none were set.
func (m *SimpleModel) KinematicProperties() *commonpb.KinematicProperties {
	if m.properties == nil {
		return nil
	}
	cloned, _ := proto.Clone(m.properties).(*commonpb.KinematicProperties)
	return cloned
}

// SetGeneration records the counter a component bumps whenever its model changes.
func (m *SimpleModel) SetGeneration(generation uint64) {
	m.generation = generation
}

// Generation returns the model's change counter.
func (m *SimpleModel) Generation() uint64 {
	return m.generation
}

// checkInsideHardware refuses a user limit that reaches outside the hardware limit. A hardware
// velocity or acceleration that is absent is unbounded, so any user value is fine there.
func checkInsideHardware(id string, ul JointLimits, jc *JointConfig) error {
	if ul.Min != nil && *ul.Min < jc.Min {
		return fmt.Errorf("joint %q: user min %v is below the hardware min %v", id, *ul.Min, jc.Min)
	}
	if ul.Max != nil && *ul.Max > jc.Max {
		return fmt.Errorf("joint %q: user max %v is above the hardware max %v", id, *ul.Max, jc.Max)
	}
	if ul.MaxVelocity != nil && jc.MaxVelocity != nil && *ul.MaxVelocity > *jc.MaxVelocity {
		return fmt.Errorf("joint %q: user max velocity %v is above the hardware max velocity %v", id, *ul.MaxVelocity, *jc.MaxVelocity)
	}
	if ul.MaxAcceleration != nil && jc.MaxAcceleration != nil && *ul.MaxAcceleration > *jc.MaxAcceleration {
		return fmt.Errorf("joint %q: user max acceleration %v is above the hardware max acceleration %v",
			id, *ul.MaxAcceleration, *jc.MaxAcceleration)
	}
	return nil
}

// applyUserLimit narrows a hardware limit, already in the frame's internal units, by a user limit
// given in document units.
func applyUserLimit(hardware Limit, ul JointLimits, revolute bool) Limit {
	convert := func(v float64) float64 { return v }
	if revolute {
		convert = utils.DegToRad
	}
	out := hardware
	if ul.Min != nil {
		out.Min = convert(*ul.Min)
	}
	if ul.Max != nil {
		out.Max = convert(*ul.Max)
	}
	if ul.MaxVelocity != nil {
		v := convert(*ul.MaxVelocity)
		out.MaxVelocity = &v
	}
	if ul.MaxAcceleration != nil {
		a := convert(*ul.MaxAcceleration)
		out.MaxAcceleration = &a
	}
	return out
}

func (jl JointLimits) clone() JointLimits {
	return JointLimits{
		Min:             copyFloat(jl.Min),
		Max:             copyFloat(jl.Max),
		MaxVelocity:     copyFloat(jl.MaxVelocity),
		MaxAcceleration: copyFloat(jl.MaxAcceleration),
	}
}

func jointLimitsToProto(jl JointLimits) *commonpb.JointLimits {
	return &commonpb.JointLimits{
		Min:             copyFloat(jl.Min),
		Max:             copyFloat(jl.Max),
		MaxVelocity:     copyFloat(jl.MaxVelocity),
		MaxAcceleration: copyFloat(jl.MaxAcceleration),
	}
}

func jointLimitsFromProto(pb *commonpb.JointLimits) JointLimits {
	return JointLimits{
		Min:             copyFloat(pb.Min),
		Max:             copyFloat(pb.Max),
		MaxVelocity:     copyFloat(pb.MaxVelocity),
		MaxAcceleration: copyFloat(pb.MaxAcceleration),
	}
}
