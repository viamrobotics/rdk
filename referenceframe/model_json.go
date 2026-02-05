package referenceframe

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

// ErrMimicSourceNotFound is returned when a mimic joint references a source joint that doesn't exist.
var ErrMimicSourceNotFound = errors.New("mimic joint references non-existent source joint")

// ErrCircularMimicReference is returned when mimic joint references form a cycle.
var ErrCircularMimicReference = errors.New("circular mimic joint reference detected")

// ErrNoModelInformation is used when there is no model information.
var ErrNoModelInformation = errors.New("no model information")

// ModelConfigJSON represents all supported fields in a kinematics JSON file.
type ModelConfigJSON struct {
	Name         string          `json:"name"`
	KinParamType string          `json:"kinematic_param_type,omitempty"`
	Links        []LinkConfig    `json:"links,omitempty"`
	Joints       []JointConfig   `json:"joints,omitempty"`
	DHParams     []DHParamConfig `json:"dhParams,omitempty"`
	OriginalFile *ModelFile
}

// ModelFile is a struct that stores the raw bytes of the file used to create the model as well as its extension,
// which is useful for knowing how to unmarhsal it.
type ModelFile struct {
	Bytes     []byte
	Extension string
}

// UnmarshalModelJSON will parse the given JSON data into a kinematics model. modelName sets the name of the model,
// will use the name from the JSON if string is empty.
func UnmarshalModelJSON(jsonData []byte, modelName string) (Model, error) {
	m := &ModelConfigJSON{OriginalFile: &ModelFile{Bytes: jsonData, Extension: "json"}}

	// empty data probably means that the robot component has no model information
	if len(jsonData) == 0 {
		return nil, ErrNoModelInformation
	}

	err := json.Unmarshal(jsonData, m)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json file")
	}

	return m.ParseConfig(modelName)
}

// ParseConfig converts the ModelConfig struct into a full Model with the name modelName.
func (cfg *ModelConfigJSON) ParseConfig(modelName string) (Model, error) {
	var err error
	if modelName == "" {
		modelName = cfg.Name
	}

	model := NewSimpleModel(modelName)
	model.modelConfig = cfg
	transforms := map[string]Frame{}

	// Make a map of parents for each element for post-process, to allow items to be processed out of order
	parentMap := map[string]string{}

	switch cfg.KinParamType {
	case "SVA", "":
		for _, link := range cfg.Links {
			if link.ID == World {
				return nil, NewReservedWordError("link", World)
			}
		}
		for _, joint := range cfg.Joints {
			if joint.ID == World {
				return nil, NewReservedWordError("joint", World)
			}
		}

		for _, link := range cfg.Links {
			lif, err := link.ParseConfig()
			if err != nil {
				return nil, err
			}
			parentMap[link.ID] = link.Parent
			transforms[link.ID], err = lif.ToStaticFrame(link.ID)
			if err != nil {
				return nil, err
			}
		}

		// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
		for _, joint := range cfg.Joints {
			parentMap[joint.ID] = joint.Parent
			transforms[joint.ID], err = joint.ToFrame()
			if err != nil {
				return nil, err
			}
		}

	case "DH":
		for _, dh := range cfg.DHParams {
			rFrame, lFrame, err := dh.ToDHFrames()
			if err != nil {
				return nil, err
			}
			// Joint part of DH param
			jointID := dh.ID + "_j"
			parentMap[jointID] = dh.Parent
			transforms[jointID] = rFrame

			// Link part of DH param
			linkID := dh.ID
			parentMap[linkID] = jointID
			transforms[dh.ID] = lFrame
		}

	default:
		return nil, errors.Errorf("unsupported param type: %s, supported params are SVA and DH", cfg.KinParamType)
	}

	// Create an ordered list of transforms
	ot, err := sortTransforms(transforms, parentMap)
	if err != nil {
		return nil, err
	}

	model.SetOrdTransforms(ot)

	// Build mimic mappings if any joints have mimic configs
	if cfg.KinParamType == "SVA" || cfg.KinParamType == "" {
		mimicMappings, err := buildMimicMappings(cfg.Joints, model.ordTransforms)
		if err != nil {
			return nil, err
		}
		if len(mimicMappings) > 0 {
			model.setMimicMappings(mimicMappings)
		}
	}

	return model, nil
}

// buildMimicMappings identifies mimic joints, resolves chains, detects cycles, and computes
// the sourceInputIdx for each mimic frame.
func buildMimicMappings(joints []JointConfig, ordTransforms []Frame) (map[int]*mimicMapping, error) {
	// Build a map of joint ID -> MimicConfig for joints that have mimic set
	mimicConfigs := map[string]*MimicConfig{}
	for i := range joints {
		if joints[i].Mimic != nil {
			mimicConfigs[joints[i].ID] = joints[i].Mimic
		}
	}
	if len(mimicConfigs) == 0 {
		return nil, nil
	}

	// Resolve mimic chains: if A mimics B and B mimics C, then A should mimic C
	// with composed multiplier and offset.
	resolvedMimics := map[string]*MimicConfig{}
	for jointID, mc := range mimicConfigs {
		// Walk the chain to find the ultimate source
		visited := map[string]bool{jointID: true}
		currentMC := mc
		composedMultiplier := currentMC.EffectiveMultiplier()
		composedOffset := currentMC.Offset

		sourceJoint := currentMC.Joint
		for {
			nextMC, ok := mimicConfigs[sourceJoint]
			if !ok {
				// sourceJoint is not a mimic, it's the ultimate source
				break
			}
			if visited[sourceJoint] {
				return nil, fmt.Errorf("%w: joint %q", ErrCircularMimicReference, jointID)
			}
			visited[sourceJoint] = true

			// Compose: if A = m1*B + o1, and B = m2*C + o2, then A = m1*(m2*C + o2) + o1 = m1*m2*C + m1*o2 + o1
			composedOffset = composedMultiplier*nextMC.Offset + composedOffset
			composedMultiplier *= nextMC.EffectiveMultiplier()
			sourceJoint = nextMC.Joint
		}

		mult := composedMultiplier
		resolvedMimics[jointID] = &MimicConfig{
			Joint:      sourceJoint,
			Multiplier: &mult,
			Offset:     composedOffset,
		}
	}

	// Build a map of frame name -> ordTransforms index
	frameNameToIdx := map[string]int{}
	for i, f := range ordTransforms {
		frameNameToIdx[f.Name()] = i
	}

	// Find the ordTransforms index of each source joint and compute its input index
	// (counting only non-mimic DoF frames before it)
	mimicFrameIndices := map[int]bool{}
	for jointID := range resolvedMimics {
		if idx, ok := frameNameToIdx[jointID]; ok {
			mimicFrameIndices[idx] = true
		}
	}

	// Compute the input index for each non-mimic DoF frame
	frameIdxToInputIdx := map[int]int{}
	inputIdx := 0
	for i, f := range ordTransforms {
		if len(f.DoF()) > 0 && !mimicFrameIndices[i] {
			frameIdxToInputIdx[i] = inputIdx
			inputIdx += len(f.DoF())
		}
	}

	// Build the final mimic mappings
	result := map[int]*mimicMapping{}
	for jointID, mc := range resolvedMimics {
		mimicIdx, ok := frameNameToIdx[jointID]
		if !ok {
			continue // joint not in ordTransforms (shouldn't happen)
		}

		sourceIdx, ok := frameNameToIdx[mc.Joint]
		if !ok {
			return nil, fmt.Errorf("%w: joint %q references source %q", ErrMimicSourceNotFound, jointID, mc.Joint)
		}

		sourceInputIdx, ok := frameIdxToInputIdx[sourceIdx]
		if !ok {
			return nil, fmt.Errorf("%w: joint %q references source %q which has no DoF", ErrMimicSourceNotFound, jointID, mc.Joint)
		}

		result[mimicIdx] = &mimicMapping{
			sourceInputIdx: sourceInputIdx,
			multiplier:     mc.EffectiveMultiplier(),
			offset:         mc.Offset,
		}
	}

	return result, nil
}

// ParseModelJSONFile will read a given file and then parse the contained JSON data.
func ParseModelJSONFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file")
	}
	return UnmarshalModelJSON(jsonData, modelName)
}

// Create an ordered list of transforms given a mapping of child to parent frames.
func sortTransforms(transforms map[string]Frame, parents map[string]string) ([]Frame, error) {
	// find the end effector first - determine which transforms have no children
	// copy the map of children -> parents
	ees := map[string]string{}
	for child, parent := range parents {
		ees[child] = parent
	}
	// now remove all parents
	for _, parent := range parents {
		delete(ees, parent)
	}
	// ensure there is only on end effector
	if len(ees) != 1 {
		return nil, fmt.Errorf("%w, have %v", ErrNeedOneEndEffector, ees)
	}

	// start the search from the end effector
	curr := maps.Keys(ees)[0]
	seen := map[string]bool{curr: true}
	orderedTransforms := []Frame{}
	for i := 0; i < len(parents); i++ {
		frame, ok := transforms[curr]
		if !ok {
			return nil, NewFrameNotInListOfTransformsError(curr)
		}
		orderedTransforms = append(orderedTransforms, frame)

		// find the parent of the current transform
		parent, ok := parents[curr]
		if !ok {
			return nil, NewParentFrameNotInMapOfParentsError(curr)
		}

		// make sure it wasn't seen, mark it seen, then add it to the list
		if seen[parent] {
			return nil, ErrCircularReference
		}
		seen[parent] = true

		// update the frame to add next
		curr = parent
	}

	// After the above loop, the transforms are in reverse order, so we reverse the list.
	for i, j := 0, len(orderedTransforms)-1; i < j; i, j = i+1, j-1 {
		orderedTransforms[i], orderedTransforms[j] = orderedTransforms[j], orderedTransforms[i]
	}

	return orderedTransforms, nil
}
