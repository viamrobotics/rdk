package referenceframe

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
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
	OutputFrames []string        `json:"output_frames,omitempty"`
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

	// Build the internal frame system from the transforms and parent map.
	// When no output_frames are specified, exactly one leaf (end effector)
	// is required so we can determine the output unambiguously.
	requireSingleLeaf := len(cfg.OutputFrames) == 0
	fs, leaves, err := buildModelFrameSystem(transforms, parentMap, requireSingleLeaf)
	if err != nil {
		return nil, err
	}

	if len(cfg.OutputFrames) > 1 {
		return nil, fmt.Errorf("multiple output frames are not yet supported, got %v", cfg.OutputFrames)
	}

	var primaryOutput string
	if len(cfg.OutputFrames) == 0 {
		primaryOutput = leaves[0]
	} else {
		primaryOutput = cfg.OutputFrames[0]
	}

	// Build mimic mappings if any SVA joints have mimic configs.
	var builtModel *SimpleModel
	if cfg.KinParamType == "SVA" || cfg.KinParamType == "" {
		mimicMappings, mimicErr := buildMimicMappings(cfg.Joints, fs)
		if mimicErr != nil {
			return nil, mimicErr
		}
		if len(mimicMappings) > 0 {
			builtModel, err = NewModelWithMimics(modelName, fs, primaryOutput, mimicMappings)
		} else {
			builtModel, err = NewModel(modelName, fs, primaryOutput)
		}
	} else {
		builtModel, err = NewModel(modelName, fs, primaryOutput)
	}
	if err != nil {
		return nil, err
	}
	builtModel.modelConfig = cfg

	return builtModel, nil
}

// buildMimicMappings identifies mimic joints from the config, resolves chains (A mimics B mimics C),
// detects cycles, validates that source frames exist in the FrameSystem and have DoF, and returns
// a map of frame name -> mimicMapping. The sourceInputIdx is set to -1 as a placeholder;
// it is resolved in NewModelWithMimics after the input schema is built.
func buildMimicMappings(joints []JointConfig, fs *FrameSystem) (map[string]*mimicMapping, error) {
	// Collect joints with mimic config.
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
		visited := map[string]bool{jointID: true}
		composedMultiplier := mc.EffectiveMultiplier()
		composedOffset := mc.Offset

		sourceJoint := mc.Joint
		for {
			nextMC, ok := mimicConfigs[sourceJoint]
			if !ok {
				break // sourceJoint is not a mimic, it's the ultimate source
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

	// Validate source frames exist in FS and have DoF, then build the mapping.
	result := map[string]*mimicMapping{}
	for jointID, mc := range resolvedMimics {
		sourceFrame := fs.Frame(mc.Joint)
		if sourceFrame == nil {
			return nil, fmt.Errorf("%w: joint %q references source %q", ErrMimicSourceNotFound, jointID, mc.Joint)
		}
		if len(sourceFrame.DoF()) == 0 {
			return nil, fmt.Errorf("%w: joint %q references source %q which has no DoF", ErrMimicSourceNotFound, jointID, mc.Joint)
		}

		result[jointID] = &mimicMapping{
			sourceFrameName: mc.Joint,
			sourceInputIdx:  -1, // placeholder, resolved in NewModelWithMimics
			multiplier:      mc.EffectiveMultiplier(),
			offset:          mc.Offset,
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

// buildModelFrameSystem builds a FrameSystem from a map of frames and their parent relationships.
// It performs BFS from the root (frames whose parent is "" or not in the map, i.e., world).
// It returns the FrameSystem and the list of leaf frame names.
// When requireSingleLeaf is true, the function errors if the model does not have exactly one leaf
// (end effector). Pass false when output_frames is explicitly specified in the config, allowing
// branching topologies with multiple leaves.
func buildModelFrameSystem(transforms map[string]Frame, parents map[string]string, requireSingleLeaf bool) (*FrameSystem, []string, error) {
	// Build children map
	childrenOf := map[string][]string{}
	for child, parent := range parents {
		childrenOf[parent] = append(childrenOf[parent], child)
	}

	// Find leaves (frames with no children) before BFS. This must be checked first
	// because cycles (e.g. worldDH.json) can prevent BFS from visiting any nodes,
	// and we want to report the leaf-count error rather than a circular reference error.
	var leaves []string
	for name := range transforms {
		if len(childrenOf[name]) == 0 {
			leaves = append(leaves, name)
		}
	}
	if requireSingleLeaf && len(leaves) != 1 {
		return nil, nil, fmt.Errorf("%w, have %v", ErrNeedOneEndEffector, leaves)
	}

	fs := NewEmptyFrameSystem("internal")

	// BFS from root frames (those whose parent is not in the transforms map, e.g. "world" or "")
	seen := map[string]bool{}
	queue := []string{}
	for child, parent := range parents {
		if _, inTransforms := transforms[parent]; !inTransforms {
			queue = append(queue, child)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur] {
			return nil, nil, ErrCircularReference
		}
		seen[cur] = true

		frame, ok := transforms[cur]
		if !ok {
			return nil, nil, NewFrameNotInListOfTransformsError(cur)
		}

		parentName := parents[cur]
		var parentFrame Frame
		if _, inTransforms := transforms[parentName]; !inTransforms {
			// Parent is not a frame in the transforms map (e.g., "world" or ""), treat as world
			parentFrame = fs.World()
		} else {
			parentFrame = fs.Frame(parentName)
			if parentFrame == nil {
				return nil, nil, NewParentFrameNotInMapOfParentsError(cur)
			}
		}

		if err := fs.AddFrame(frame, parentFrame); err != nil {
			return nil, nil, err
		}

		queue = append(queue, childrenOf[cur]...)
	}

	// Check all transforms were visited
	if len(seen) != len(transforms) {
		return nil, nil, ErrCircularReference
	}

	return fs, leaves, nil
}
