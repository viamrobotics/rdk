package referenceframe

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelConfig represents all supported fields in a kinematics JSON file.
type ModelConfig struct {
	Name         string `json:"name"`
	KinParamType string `json:"kinematic_param_type"`
	Links        []struct {
		ID          string                    `json:"id"`
		Parent      string                    `json:"parent"`
		Translation spatial.TranslationConfig `json:"translation"`
		Orientation spatial.OrientationConfig `json:"orientation"`
		Geometry    spatial.GeometryConfig    `json:"geometry"`
	} `json:"links"`
	Joints []struct {
		ID     string             `json:"id"`
		Type   string             `json:"type"`
		Parent string             `json:"parent"`
		Axis   spatial.AxisConfig `json:"axis"`
		Max    float64            `json:"max"` // in mm or degs
		Min    float64            `json:"min"` // in mm or degs
	} `json:"joints"`
	DHParams []struct {
		ID       string                 `json:"id"`
		Parent   string                 `json:"parent"`
		A        float64                `json:"a"`
		D        float64                `json:"d"`
		Alpha    float64                `json:"alpha"`
		Max      float64                `json:"max"` // in mm or degs
		Min      float64                `json:"min"` // in mm or degs
		Geometry spatial.GeometryConfig `json:"geometry"`
	} `json:"dhParams"`
	RawFrames []FrameMapConfig `json:"frames"`
}

// RobotURDF represents all supported fields in a Universal Robot Description Format (URDF) file.
type RobotURDF struct {
	XMLName xml.Name `xml:"robot"`
	Name    string   `xml:"name,attr"`
	Links   []struct{
		XMLName   xml.Name `xml:"link"`
		Name      string   `xml:"name,attr"`
		Collision []struct{
			XMLName xml.Name `xml:"collision"`
			Name    string   `xml:"name,attr"`
			Origin  struct{
				XMLName xml.Name `xml:"origin"`
				RPY     string   `xml:"rpy,attr"`  // "r p y" format
				XYZ     string   `xml:"xyz,attr"`  // "x y z" format
			} `xml:"origin"`
			Geometry struct{
				XMLName xml.Name `xml:"geometry"`
				Box     struct{
					XMLName xml.Name `xml:"box"`
					Size    string   `xml:"size,attr"`  // "x y z" format
				} `xml:"box"`
				Sphere struct{
					XMLName xml.Name `xml:"sphere"`
					Radius  float64   `xml:"radius,attr"`
				} `xml:"sphere"`
			} `xml:"geometry"`
		} `xml:"collision"`
	} `xml:"link"`
	Joints            []struct{
		XMLName xml.Name `xml:"joint"`
		Name    string   `xml:"name,attr"`
		Type    string   `xml:"type,attr"`
		Origin  struct{
			XMLName xml.Name `xml:"origin"`
			RPY     string   `xml:"rpy,attr"`  // "r p y" format
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format
		} `xml:"origin"`
		Parent struct{
			XMLName xml.Name `xml:"parent"`
			Link    string   `xml:"link,attr"`
		} `xml:"parent"`
		Child struct{
			XMLName xml.Name `xml:"child"`
			Link    string   `xml:"link,attr"`
		} `xml:"child"`
		Axis struct{
			XMLName xml.Name `xml:"axis"`
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format
		} `xml:"axis"`
		Limit struct{
			XMLName xml.Name `xml:"limit"`
			Lower   float64  `xml:"lower,attr"`
			Upper   float64  `xml:"upper,attr"`
		} `xml:"limit"`
	} `xml:"joint"`
}

// ParseConfig converts the ModelConfig struct into a full Model with the name modelName.
func (config *ModelConfig) ParseConfig(modelName string) (Model, error) {
	var err error
	if modelName == "" {
		modelName = config.Name
	}

	model := NewSimpleModel(modelName)
	transforms := map[string]Frame{}

	// Make a map of parents for each element for post-process, to allow items to be processed out of order
	parentMap := map[string]string{}

	switch config.KinParamType {
	case "SVA", "":
		for _, link := range config.Links {
			if link.ID == World {
				return nil, errors.New("reserved word: cannot name a link 'world'")
			}
		}
		for _, joint := range config.Joints {
			if joint.ID == World {
				return nil, errors.New("reserved word: cannot name a joint 'world'")
			}
		}

		for _, link := range config.Links {
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
		for _, joint := range config.Joints {
			parentMap[joint.ID] = joint.Parent
			switch joint.Type {
			case "revolute":
				transforms[joint.ID], err = NewRotationalFrame(joint.ID, joint.Axis.ParseConfig(),
					Limit{Min: utils.DegToRad(joint.Min), Max: utils.DegToRad(joint.Max)})
			case "prismatic":
				transforms[joint.ID], err = NewTranslationalFrame(joint.ID, r3.Vector(joint.Axis),
					Limit{Min: joint.Min, Max: joint.Max})
			default:
				return nil, errors.Errorf("unsupported joint type detected: %v", joint.Type)
			}
			if err != nil {
				return nil, err
			}
		}

	case "DH":
		for _, dh := range config.DHParams {
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

	case "frames":
		for _, x := range config.RawFrames {
			f, err := x.ParseConfig()
			if err != nil {
				return nil, err
			}
			model.OrdTransforms = append(model.OrdTransforms, f)
		}
		return model, nil

	default:
		return nil, errors.Errorf("unsupported param type: %s, supported params are SVA and DH", config.KinParamType)
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
	seen := map[string]bool{}
	nextTransform := transforms[eename]
	orderedTransforms := []Frame{nextTransform}
	seen[eename] = true
	for {
		parent := parentMap[nextTransform.Name()]
		if seen[parent] {
			return nil, errors.New("infinite loop finding path from end effector to world")
		}
		// Reserved word, we reached the end of the chain
		if parent == World {
			break
		}
		seen[parent] = true
		nextTransform = transforms[parent]
		orderedTransforms = append(orderedTransforms, nextTransform)
	}
	// After the above loop, the transforms are in reverse order, so we reverse the list.
	for i, j := 0, len(orderedTransforms)-1; i < j; i, j = i+1, j-1 {
		orderedTransforms[i], orderedTransforms[j] = orderedTransforms[j], orderedTransforms[i]
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

// ParseURDFFile will read a given file and parse the contained URDF XML data.
func ParseURDFFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read URDF file")
	}
	return ConvertURDFToConfig(xmlData, modelName)
}

// ConvertURDFToConfig will transfer the given URDF XML data into an equivalent Config. Direct conversion to a model in
// the same fashion as ModelJSON is not possible, as URDF data will need to be reconfigured to accomodate differences
// between the two kinematics encoding schemes.
func ConvertURDFToConfig(xmlData []byte, modelName string) (Model, error) {
	// empty data probably means that the read URDF has no actionable information
	if len(xmlData) == 0 {
		return nil, ErrNoModelInformation
	}

	urdf := &RobotURDF{}
	err := xml.Unmarshal(xmlData, urdf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert URDF data to equivalent internal struct")
	}
	fmt.Println(urdf)  // DEBUG(wspies)

	// Code below this point could be split off into another function, similarly to ParseConfig
	if modelName == "" {
		modelName = urdf.Name
	}
	model := NewSimpleModel(modelName)
	transforms := map[string]Frame{}

	// TODO(wspies): 

	// Handle joints first
	for _, joint := range urdf.Joints {
		// TODO(wspies): Keep Track of Parent and Child relationships to links

		// Parse Axis and Origin combination
		axes, _ := convStringAttr(joint.Axis.XYZ)

		// TODO(wspies): Can probably combine logic for continuous and revolute case, change limit handling appropriately
		switch joint.Type {
		case "continuous":
			fmt.Println(axes)

			transforms[joint.Name], err = NewRotationalFrame(joint.Name, spatial.R4AA{RX: axes[0], RY: axes[1], RZ: axes[2]}, Limit{Min: -math.Pi, Max: math.Pi})
			fmt.Println(transforms)

			fmt.Println("continuous type")
		case "revolute":
			transforms[joint.Name], err = NewRotationalFrame(joint.Name, spatial.R4AA{RX: axes[0], RY: axes[1], RZ: axes[2]}, Limit{Min: joint.Limit.Lower, Max: joint.Limit.Upper})
			fmt.Println(transforms)

			fmt.Println("revolute type")
		case "prismatic":
			transforms[joint.Name], err = NewTranslationalFrame(joint.Name, r3.Vector{X: axes[0], Y: axes[1], Z: axes[2]}, Limit{Min: joint.Limit.Lower, Max: joint.Limit.Upper})
			fmt.Println("prismatic type")
		case "fixed":
			fmt.Println("fixed type")
		default:
			fmt.Println("Unsupported joint type")
		}
		fmt.Println(joint)

		if err != nil {
			return nil, err
		}
	}

	for _, link := range urdf.Links {
		fmt.Println(link)
		// TODO(wspies): Implement
	}

	return model, nil
	//return nil, errors.New("Unimplemented method")  // DEBUG(wspies)
}

// TODO(wspies): Add documentation for this method
// TODO(wspies): Add error if it fails to convert for some reason?
func convStringAttr(attr string) ([]float64, error) {
	var converted []float64
	attr_slice := strings.Fields(attr)

	for _, value := range attr_slice {
		value, _ := strconv.ParseFloat(value, 64)
		converted = append(converted, value)
	}

	return converted, nil
}