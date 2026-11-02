package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
)

// ErrMimicSourceNotFound is returned when a mimic joint references a source joint that doesn't exist.
var ErrMimicSourceNotFound = errors.New("mimic joint references non-existent source joint")

// ErrMimicWithLimits is returned when a mimic joint specifies its own limits.
var ErrMimicWithLimits = errors.New(
	"mimic joint must not specify min/max or velocity/acceleration limits; limits are determined by the source joint",
)

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
	OriginalFile *ModelFile      `json:"original_file,omitempty"`
}

// ModelFile is a struct that stores the raw bytes of the file used to create the model as well as its extension,
// which is useful for knowing how to unmarhsal it.
type ModelFile struct {
	Bytes     []byte `json:"bytes"`
	Extension string `json:"extension"`
}

// UnmarshalModelJSON will parse the given JSON data into a kinematics model. modelName sets the name of the model,
// will use the name from the JSON if string is empty. The data may be an SVA or DH document or the proto JSON of a
// KinematicModel, see ReadKinematicModelFile. A kinematic model whose meshes live in files beside it has to be read
// through ParseModelJSONFile, since bytes alone give no directory to find them in.
func UnmarshalModelJSON(jsonData []byte, modelName string) (Model, error) {
	return unmarshalModelJSON(jsonData, modelName, "")
}

// unmarshalModelJSON reads either encoding. dir is where a kinematic model's mesh files are
// looked for, and is empty when the caller has nothing but bytes.
func unmarshalModelJSON(jsonData []byte, modelName, dir string) (Model, error) {
	// empty data probably means that the robot component has no model information
	if len(jsonData) == 0 {
		return nil, ErrNoModelInformation
	}
	if isKinematicModelDocument(jsonData) {
		pb, err := unmarshalKinematicModel(jsonData, dir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse kinematic model")
		}
		return ModelFromProto(pb, modelName)
	}

	m := &ModelConfigJSON{OriginalFile: &ModelFile{Bytes: jsonData, Extension: "json"}}
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
			parentMap[link.ID] = link.Parent
			transforms[link.ID], err = link.ToFrame()
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

	if err := validateGeometryLabels(transforms); err != nil {
		return nil, err
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
			if joints[i].Min != 0 || joints[i].Max != 0 ||
				joints[i].MaxVelocity != nil || joints[i].MaxAcceleration != nil {
				return nil, fmt.Errorf("%w: joint %q", ErrMimicWithLimits, joints[i].ID)
			}
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
		composedOffset := mc.ValueOffset

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
			composedOffset = composedMultiplier*nextMC.ValueOffset + composedOffset
			composedMultiplier *= nextMC.EffectiveMultiplier()
			sourceJoint = nextMC.Joint
		}

		resolvedMimics[jointID] = &MimicConfig{
			Joint:           sourceJoint,
			ValueMultiplier: composedMultiplier,
			ValueOffset:     composedOffset,
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
			valueMultiplier: mc.EffectiveMultiplier(),
			valueOffset:     mc.ValueOffset,
		}
	}

	return result, nil
}

// ParseModelJSONFile will read a given file and then parse the contained JSON data, which may be an SVA or DH
// document or the proto JSON of a KinematicModel. Mesh files a kinematic model names are read relative to the file.
func ParseModelJSONFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file")
	}
	model, err := unmarshalModelJSON(jsonData, modelName, filepath.Dir(filename))
	if err != nil {
		return nil, errors.Wrapf(err, "model file %s", filename)
	}
	return model, nil
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

// validateGeometryLabels refuses a model whose geometry labels could be mistaken for one another.
// Collision checking resolves a label to a frame by name first and by geometry label second, so a
// duplicate label, or a label that names a different frame, would charge a collision to the wrong
// link or lose it.
func validateGeometryLabels(transforms map[string]Frame) error {
	names := make([]string, 0, len(transforms))
	for name := range transforms {
		names = append(names, name)
	}
	sort.Strings(names)
	owner := map[string]string{}
	for _, name := range names {
		frame := transforms[name]
		gif, err := frame.Geometries(inputsWithinLimits(frame.DoF()))
		if err != nil {
			return err
		}
		for _, g := range gif.Geometries() {
			label := g.Label()
			if label == World {
				return errors.Errorf("geometry on frame %q is labelled %q, which is reserved", name, World)
			}
			if other, ok := transforms[label]; ok && other != frame {
				return errors.Errorf("geometry on frame %q is labelled %q, the name of another frame", name, label)
			}
			if prev, dup := owner[label]; dup {
				return errors.Errorf("geometry label %q is used on frames %q and %q", label, prev, name)
			}
			owner[label] = name
		}
	}
	return nil
}

// inputsWithinLimits returns one input per degree of freedom that the frame accepts, since a frame
// refuses inputs outside its limits and zero is not always inside them.
func inputsWithinLimits(limits []Limit) []Input {
	inputs := make([]Input, len(limits))
	for i, l := range limits {
		switch {
		case !math.IsInf(l.Min, 0) && !math.IsInf(l.Max, 0):
			inputs[i] = (l.Min + l.Max) / 2
		case !math.IsInf(l.Min, 0):
			inputs[i] = l.Min
		case !math.IsInf(l.Max, 0):
			inputs[i] = l.Max
		}
	}
	return inputs
}
