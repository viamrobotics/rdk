package referenceframe

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
)

// ErrNoModelInformation is used when there is no model information.
var ErrNoModelInformation = errors.New("no model information")

// ModelConfig represents all supported fields in a kinematics JSON file.
type ModelConfig struct {
	Name         string          `json:"name"`
	KinParamType string          `json:"kinematic_param_type,omitempty"`
	Links        []LinkConfig    `json:"links,omitempty"`
	Joints       []JointConfig   `json:"joints,omitempty"`
	DHParams     []DHParamConfig `json:"dhParams,omitempty"`
	OriginalFile *ModelFile
}

type ModelFile struct {
	Bytes     []byte
	Extension string
}

// UnmarshalModelJSON will parse the given JSON data into a kinematics model. modelName sets the name of the model,
// will use the name from the JSON if string is empty.
func UnmarshalModelJSON(jsonData []byte, modelName string) (Model, error) {
	m := &ModelConfig{OriginalFile: &ModelFile{Bytes: jsonData, Extension: "json"}}

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
func (cfg *ModelConfig) ParseConfig(modelName string) (Model, error) {
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
				return nil, NewReservedWordError("link", "world")
			}
		}
		for _, joint := range cfg.Joints {
			if joint.ID == World {
				return nil, NewReservedWordError("joint", "world")
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

	// Determine which transforms have no children
	parents := map[string]Frame{}
	// First create a copy of the map
	for id, trans := range transforms {
		parents[id] = trans
	}
	// Now remove all parents
	for _, trans := range transforms {
		delete(parents, parentMap[trans.Name()])
	}

	if len(parents) > 1 {
		return nil, errors.New("more than one end effector not supported")
	}
	if len(parents) < 1 {
		return nil, ErrAtLeastOneEndEffector
	}
	var eename string
	// TODO(pl): is there a better way to do all this? Annoying to iterate over a map three times. Maybe if we
	// implement Child() as well as Parent()?
	for id := range parents {
		eename = id
	}

	// Create an ordered list of transforms
	model.OrdTransforms, err = sortTransforms(transforms, parentMap, eename, World)
	if err != nil {
		return nil, err
	}

	return model, nil
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
