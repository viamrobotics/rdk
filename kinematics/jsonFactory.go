package kinematics

import (
	"encoding/json"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

var world = "world"

// AutoGenerated represents all supported fields in a kinematics JSON file.
type AutoGenerated struct {
	Model struct {
		Manufacturer string `json:"manufacturer"`
		Name         string `json:"name"`
		Links        []struct {
			ID             string             `json:"id"`
			Parent         string             `json:"parent"`
			Translation    config.Translation `json:"translation"`
			Orientation    config.Orientation `json:"orientation"`
			SetOrientation bool               `json:"setorientation"`
		} `json:"links"`
		Joints []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Parent string `json:"parent"`
			Axis   struct {
				X int `json:"x"`
				Y int `json:"y"`
				Z int `json:"z"`
			} `json:"axis"`
			Max       float64 `json:"max"`
			Min       float64 `json:"min"`
			Direction string  `json:"direction"`
		} `json:"joints"`
		Tolerances *SolverDistanceWeights `json:"tolerances"`
	} `json:"model"`
}

// ParseJSONFile will read a given file and then parse the contained JSON data.
func ParseJSONFile(filename string) (*Model, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Errorf("failed to read json file: %w", err)
	}
	return ParseJSON(jsonData)
}

// ParseJSON will parse the given JSON data into a kinematics model.
func ParseJSON(jsonData []byte) (*Model, error) {
	model := NewModel()
	m := AutoGenerated{}

	err := json.Unmarshal(jsonData, &m)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshall json file %w", err)
	}

	model.manufacturer = m.Model.Manufacturer
	model.name = m.Model.Name
	transforms := make(map[string]Transform)

	for _, link := range m.Model.Links {
		if link.ID == world {
			return model, errors.New("reserved word: cannot name a link 'world'")
		}
	}
	for _, joint := range m.Model.Joints {
		if joint.ID == world {
			return model, errors.New("reserved word: cannot name a joint 'world'")
		}
	}

	for _, link := range m.Model.Links {

		linkT := NewLink(link.Parent)

		if link.SetOrientation {
			newOV := &spatialmath.OrientationVec{utils.DegToRad(link.Orientation.TH), link.Orientation.X, link.Orientation.Y, link.Orientation.Z}
			linkT.quat = spatialmath.NewDualQuaternionFromRotation(newOV)
		}

		linkT.quat.SetTranslation(link.Translation.X, link.Translation.Y, link.Translation.Z)
		transforms[link.ID] = linkT
	}

	// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
	for _, joint := range m.Model.Joints {

		// TODO(pl): Make this a switch once we support more than one joint type
		if joint.Type == "revolute" {
			// TODO(pl): Add speed, wraparound, etc
			dir := joint.Direction
			// Check for valid joint direction
			if dir != "" && dir != "cw" && dir != "ccw" {
				return nil, errors.Errorf("unsupported joint direction: %s", dir)
			}

			rev := NewJoint([]int{joint.Axis.X, joint.Axis.Y, joint.Axis.Z}, dir, joint.Parent)

			rev.max = append(rev.max, joint.Max*math.Pi/180)
			rev.min = append(rev.min, joint.Min*math.Pi/180)

			transforms[joint.ID] = rev
		} else {
			return nil, errors.Errorf("unsupported joint type detected: %v", joint.Type)
		}
	}

	// Determine which transforms have no children
	parents := make(map[string]Transform)
	// First create a copy of the map
	for id, trans := range transforms {
		parents[id] = trans
	}
	// Now remove all parents
	for _, trans := range transforms {
		delete(parents, trans.Parent())
	}
	if len(parents) > 1 {
		return nil, errors.New("more than one end effector not supported")
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
	orderedTransforms := []Transform{nextTransform}
	seen[eename] = true
	for {
		parent := nextTransform.Parent()
		if seen[parent] {
			return nil, errors.New("infinite loop finding path from end effector to world")
		}
		// Reserved word, we reached the end of the chain
		if parent == world {
			break
		}
		seen[parent] = true
		nextTransform = transforms[parent]
		orderedTransforms = append(orderedTransforms, nextTransform)
	}
	model.OrdTransforms = orderedTransforms

	if m.Model.Tolerances != nil {
		model.SolveWeights = *m.Model.Tolerances
	}

	return model, err
}
