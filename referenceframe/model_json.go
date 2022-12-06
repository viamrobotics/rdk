package referenceframe

import (
	"encoding/json"
	"os"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelConfig represents all supported fields in a kinematics JSON file.
type ModelConfig struct {
	Name         string           `json:"name"`
	KinParamType string           `json:"kinematic_param_type"`
	Links        []JsonLink       `json:"links"`
	Joints       []JsonJoint      `json:"joints"`
	DHParams     []JsonDHParam    `json:"dhParams"`
}

type JsonLink struct {
	ID          string                    `json:"id"`
	Parent      string                    `json:"parent"`
	Translation spatial.TranslationConfig `json:"translation"`
	Orientation spatial.OrientationConfig `json:"orientation"`
	Geometry    spatial.GeometryConfig    `json:"geometry"`
}

type JsonJoint struct {
	ID     string             `json:"id"`
	Type   string             `json:"type"`
	Parent string             `json:"parent"`
	Axis   spatial.AxisConfig `json:"axis"`
	Max    float64            `json:"max"` // in mm or degs
	Min    float64            `json:"min"` // in mm or degs
}

type JsonDHParam struct {
	ID       string                 `json:"id"`
	Parent   string                 `json:"parent"`
	A        float64                `json:"a"`
	D        float64                `json:"d"`
	Alpha    float64                `json:"alpha"`
	Max      float64                `json:"max"` // in mm or degs
	Min      float64                `json:"min"` // in mm or degs
	Geometry spatial.GeometryConfig `json:"geometry"`
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
				return nil, errors.New("reserved word: cannot name a link 'world'")
			}
		}
		for _, joint := range cfg.Joints {
			if joint.ID == World {
				return nil, errors.New("reserved word: cannot name a joint 'world'")
			}
		}

		for _, link := range cfg.Links {
			parentMap[link.ID] = link.Parent
			orientation, err := link.Orientation.ParseConfig()
			if err != nil {
				return nil, err
			}
			pose := spatial.NewPoseFromOrientation(link.Translation.ParseConfig(), orientation)
			geometryCreator, err := link.Geometry.ParseConfig()
			if err == nil {
				transforms[link.ID], err = NewStaticFrameWithGeometry(link.ID, pose, geometryCreator)
			} else {
				transforms[link.ID], err = NewStaticFrame(link.ID, pose)
			}
			if err != nil {
				return nil, err
			}
		}

		// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
		for _, joint := range cfg.Joints {
			parentMap[joint.ID] = joint.Parent
			switch joint.Type {
			case "revolute":
				transforms[joint.ID], err = NewRotationalFrame(joint.ID, joint.Axis.ParseConfig(),
					Limit{Min: utils.DegToRad(joint.Min), Max: utils.DegToRad(joint.Max)})
			case "prismatic":
				transforms[joint.ID], err = NewTranslationalFrame(joint.ID, r3.Vector(joint.Axis),
					Limit{Min: joint.Min, Max: joint.Max})
			default:
				return nil, NewUnsupportedJointTypeError(joint.Type)
			}
			if err != nil {
				return nil, err
			}
		}

	case "DH":
		for _, dh := range cfg.DHParams {
			// Joint part of DH param
			jointID := dh.ID + "_j"
			parentMap[jointID] = dh.Parent
			transforms[jointID], err = NewRotationalFrame(jointID, spatial.R4AA{RX: 0, RY: 0, RZ: 1},
				Limit{Min: utils.DegToRad(dh.Min), Max: utils.DegToRad(dh.Max)})
			if err != nil {
				return nil, err
			}

			// Link part of DH param
			linkID := dh.ID
			pose := spatial.NewPoseFromDH(dh.A, dh.D, utils.DegToRad(dh.Alpha))
			parentMap[linkID] = jointID
			geometryCreator, err := dh.Geometry.ParseConfig()
			if err == nil {
				transforms[dh.ID], err = NewStaticFrameWithGeometry(dh.ID, pose, geometryCreator)
			} else {
				transforms[dh.ID], err = NewStaticFrame(dh.ID, pose)
			}
			if err != nil {
				return nil, err
			}
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
		return nil, errors.New("need at least one end effector")
	}
	var eename string
	// TODO(pl): is there a better way to do all this? Annoying to iterate over a map three times. Maybe if we
	// implement Child() as well as Parent()?
	for id := range parents {
		eename = id
	}

	// Create an ordered list of transforms
	orderedTransforms, err := sortTransforms(transforms, parentMap, eename, World)
	if err != nil {
		return nil, err
	}

	model.OrdTransforms = orderedTransforms
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

// ErrNoModelInformation is used when there is no model information.
var ErrNoModelInformation = errors.New("no model information")

// UnmarshalModelJSON will parse the given JSON data into a kinematics model. modelName sets the name of the model,
// will use the name from the JSON if string is empty.
func UnmarshalModelJSON(jsonData []byte, modelName string) (Model, error) {
	m := &ModelConfig{}

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
