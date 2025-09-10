package armplanning

import (
	"encoding/json"
	"errors"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/referenceframe"
)

// PlanState is a struct which holds both a referenceframe.FrameSystemPoses and a configuration.
// This is intended to be used as start or goal states for plans. Either field may be nil.
type PlanState struct {
	poses         referenceframe.FrameSystemPoses
	configuration referenceframe.FrameSystemInputs
}

type planStateJSON struct {
	Poses         referenceframe.FrameSystemPoses  `json:"poses"`
	Configuration referenceframe.FrameSystemInputs `json:"configuration"`
}

// MarshalJSON serializes a PlanState to JSON.
func (p *PlanState) MarshalJSON() ([]byte, error) {
	stateJSON := planStateJSON{
		Poses:         p.poses,
		Configuration: p.configuration,
	}
	return json.Marshal(stateJSON)
}

// UnmarshalJSON deserializes a PlanState from JSON.
func (p *PlanState) UnmarshalJSON(data []byte) error {
	var stateJSON planStateJSON
	if err := json.Unmarshal(data, &stateJSON); err != nil {
		return err
	}
	p.poses = stateJSON.Poses
	p.configuration = stateJSON.Configuration
	return nil
}

// NewPlanState creates a PlanState from the given poses and configuration. Either or both may be nil.
func NewPlanState(poses referenceframe.FrameSystemPoses, configuration referenceframe.FrameSystemInputs) *PlanState {
	return &PlanState{poses: poses, configuration: configuration}
}

// Poses returns the poses of the PlanState.
func (p *PlanState) Poses() referenceframe.FrameSystemPoses {
	return p.poses
}

// Configuration returns the configuration of the PlanState.
func (p *PlanState) Configuration() referenceframe.FrameSystemInputs {
	return p.configuration
}

// ComputePoses returns the poses of a PlanState if they are populated, or computes them using the given FrameSystem if not.
func (p *PlanState) ComputePoses(fs *referenceframe.FrameSystem) (referenceframe.FrameSystemPoses, error) {
	if len(p.poses) > 0 {
		return p.poses, nil
	}

	if len(p.configuration) == 0 {
		return nil, errors.New("cannot computes poses, neither poses nor configuration are populated")
	}

	return p.configuration.ComputePoses(fs)
}

// Serialize turns a PlanState into a map[string]interface suitable for being transmitted over proto.
func (p PlanState) Serialize() map[string]interface{} {
	m := map[string]interface{}{}
	poseMap := map[string]interface{}{}
	confMap := map[string]interface{}{}
	for fName, pif := range p.poses {
		pifProto := referenceframe.PoseInFrameToProtobuf(pif)
		poseMap[fName] = pifProto
	}
	for fName, conf := range p.configuration {
		confMap[fName] = referenceframe.InputsToFloats(conf)
	}
	if p.poses != nil {
		m["poses"] = poseMap
	}
	if p.configuration != nil {
		m["configuration"] = confMap
	}
	return m
}

// DeserializePlanState turns a serialized PlanState back into a PlanState.
func DeserializePlanState(iface map[string]interface{}) (*PlanState, error) {
	ps := &PlanState{
		poses:         referenceframe.FrameSystemPoses{},
		configuration: referenceframe.FrameSystemInputs{},
	}
	if posesIface, ok := iface["poses"]; ok {
		if frameSystemPoseMap, ok := posesIface.(map[string]interface{}); ok {
			for fName, pifIface := range frameSystemPoseMap {
				pifJSON, err := json.Marshal(pifIface)
				if err != nil {
					return nil, err
				}
				pifPb := &commonpb.PoseInFrame{}
				err = json.Unmarshal(pifJSON, pifPb)
				if err != nil {
					return nil, err
				}
				pif := referenceframe.ProtobufToPoseInFrame(pifPb)
				ps.poses[fName] = pif
			}
		} else {
			return nil, errors.New("could not decode contents of poses")
		}
	} else {
		ps.poses = nil
	}
	if confIface, ok := iface["configuration"]; ok {
		if confMap, ok := confIface.(map[string]interface{}); ok {
			for fName, inputsArrIface := range confMap {
				if inputsArr, ok := inputsArrIface.([]interface{}); ok {
					floats := make([]float64, 0, len(inputsArr))
					for _, inputIface := range inputsArr {
						if val, ok := inputIface.(float64); ok {
							floats = append(floats, val)
						} else {
							return nil, errors.New("configuration input array did not contain floats")
						}
					}
					ps.configuration[fName] = referenceframe.FloatsToInputs(floats)
				} else {
					return nil, errors.New("configuration did not contain array of inputs")
				}
			}
		} else {
			return nil, errors.New("could not decode contents of configuration")
		}
	} else {
		ps.configuration = nil
	}
	return ps, nil
}
