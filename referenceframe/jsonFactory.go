package referenceframe

import (
	"encoding/json"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"

	"go.viam.com/core/spatialmath"
)

type vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// ModelJSON represents all supported fields in a kinematics JSON file.
type ModelJSON struct {
	Name         string `json:"name"`
	KinParamType string `json:"kinematic_param_type"`
	Links        []struct {
		ID          string                     `json:"id"`
		Parent      string                     `json:"parent"`
		Translation spatialmath.Translation    `json:"translation"`
		Orientation spatialmath.RawOrientation `json:"orientation"`
		Volume      vector3                    `json:"volume"`
	} `json:"links"`
	Joints []struct {
		ID     string  `json:"id"`
		Type   string  `json:"type"`
		Parent string  `json:"parent"`
		Axis   vector3 `json:"axis"`
		Max    float64 `json:"max"`
		Min    float64 `json:"min"`
	} `json:"joints"`
	DHParams []struct {
		ID     string  `json:"id"`
		Parent string  `json:"parent"`
		A      float64 `json:"a"`
		D      float64 `json:"d"`
		Alpha  float64 `json:"alpha"`
		Max    float64 `json:"max"`
		Min    float64 `json:"min"`
		Volume vector3 `json:"volume"`
	} `json:"dhParams"`
	RawFrames []map[string]interface{} `json:"frames"`
}

// Model turns the ModelJSON struct into a full Model with the name modelName
func (m *ModelJSON) Model(modelName string) (*Model, error) {
	model := &Model{}
	if modelName == "" {
		model.name = m.Name
	} else {
		model.name = modelName
	}

	transforms := map[string]Frame{}

	// Make a map of parents for each element for post-process, to allow items to be processed out of order
	parentMap := map[string]string{}

	if m.KinParamType == "SVA" || m.KinParamType == "" {
		for _, link := range m.Links {
			if link.ID == World {
				return nil, errors.New("reserved word: cannot name a link 'world'")
			}
		}
		for _, joint := range m.Joints {
			if joint.ID == World {
				return nil, errors.New("reserved word: cannot name a joint 'world'")
			}
		}

		for _, link := range m.Links {
			parentMap[link.ID] = link.Parent
			orientation, err := spatialmath.ParseOrientation(link.Orientation)
			if err != nil {
				return nil, err
			}
			pt := r3.Vector{link.Translation.X, link.Translation.Y, link.Translation.Z}
			pose := spatialmath.NewPoseFromOrientationVector(pt, orientation.OrientationVectorRadians())

			var vol spatialmath.VolumeCreator
			if link.Volume.X != 0 && link.Volume.Y != 0 && link.Volume.Z != 0 {
				dims := r3.Vector{link.Volume.X, link.Volume.Y, link.Volume.Z}.Mul(0.5)
				offset := spatialmath.NewPoseFromPoint(pt.Mul(-0.5))
				vol = spatialmath.NewBoxFromOffset(dims, offset)
			}
			transforms[link.ID], err = NewStaticFrameWithVolume(link.ID, pose, vol)

			if err != nil {
				return nil, err
			}
		}

		// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
		for _, joint := range m.Joints {

			// TODO(pl): Make this a switch once we support more than one joint type
			if joint.Type == "revolute" {
				aa := spatialmath.R4AA{RX: joint.Axis.X, RY: joint.Axis.Y, RZ: joint.Axis.Z}

				rev, err := NewRotationalFrame(joint.ID, aa, Limit{Min: joint.Min * math.Pi / 180, Max: joint.Max * math.Pi / 180})
				if err != nil {
					return nil, err
				}
				parentMap[joint.ID] = joint.Parent

				transforms[joint.ID] = rev
			} else {
				return nil, errors.Errorf("unsupported joint type detected: %v", joint.Type)
			}
		}
	} else if m.KinParamType == "DH" {
		for _, dh := range m.DHParams {

			// Joint part of DH param
			jointID := dh.ID + "_j"
			parentMap[jointID] = dh.Parent
			j, err := NewRotationalFrame(jointID, spatialmath.R4AA{RX: 0, RY: 0, RZ: 1}, Limit{Min: dh.Min * math.Pi / 180, Max: dh.Max * math.Pi / 180})
			if err != nil {
				return nil, err
			}
			transforms[jointID] = j

			// Link part of DH param
			linkID := dh.ID
			linkQuat := spatialmath.NewPoseFromDH(dh.A, dh.D, dh.Alpha)

			transforms[linkID], err = NewStaticFrame(linkID, linkQuat)
			if err != nil {
				return nil, err
			}
			parentMap[linkID] = jointID
		}
	} else if m.KinParamType == "frames" {
		orderedTransforms := []Frame{}
		for _, x := range m.RawFrames {
			f, err := UnmarshalFrameMap(x)
			if err != nil {
				return nil, err
			}
			orderedTransforms = append(orderedTransforms, f)
		}
		model.OrdTransforms = orderedTransforms
		return model, nil
	} else {
		return nil, errors.Errorf("unsupported param type: %s, supported params are SVA and DH", m.KinParamType)
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

// ParseJSONFile will read a given file and then parse the contained JSON data.
func ParseJSONFile(filename, modelName string) (*Model, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Errorf("failed to read json file: %w", err)
	}
	return ParseJSON(jsonData, modelName)
}

// ParseJSON will parse the given JSON data into a kinematics model. modelName sets the name of the model,
// will use the name from the JSON if string is empty.
func ParseJSON(jsonData []byte, modelName string) (*Model, error) {
	m := &ModelJSON{}

	// empty data probably means that the robot component has no model information
	if len(jsonData) == 0 {
		return nil, nil
	}

	err := json.Unmarshal(jsonData, m)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshall json file %w", err)
	}

	return m.Model(modelName)

}
