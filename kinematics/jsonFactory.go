package kinematics

import (
	"encoding/json"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

var world = "world"
var worldFrame = frame.NewStaticFrame("world", nil, nil)

// AutoGenerated represents all supported fields in a kinematics JSON file.
type AutoGenerated struct {
	Model struct {
		Manufacturer string `json:"manufacturer"`
		Name         string `json:"name"`
		KinParamType string `json:"kinematic_param_type"`
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
				X float64 `json:"x"`
				Y float64 `json:"y"`
				Z float64 `json:"z"`
			} `json:"axis"`
			Max float64 `json:"max"`
			Min float64 `json:"min"`
		} `json:"joints"`
		DHParams []struct {
			ID     string  `json:"id"`
			Parent string  `json:"parent"`
			A      float64 `json:"a"`
			D      float64 `json:"d"`
			Alpha  float64 `json:"alpha"`
			Max    float64 `json:"max"`
			Min    float64 `json:"min"`
		} `json:"dhParams"`
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
	transforms := make(map[string]frame.Frame)
	
	// Make a map of parents for each element for post-process, to allow items to be processed out of order
	parentMap := make(map[string]string)

	if m.Model.KinParamType == "SVA" || m.Model.KinParamType == "" {
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
			parentMap[link.ID] = link.Parent
			
			q := spatialmath.NewDualQuaternion()
			if link.SetOrientation {
				newOV := &spatialmath.OrientationVec{utils.DegToRad(link.Orientation.TH), link.Orientation.X, link.Orientation.Y, link.Orientation.Z}
				q = spatialmath.NewDualQuaternionFromRotation(newOV)
			}

			q.SetTranslation(link.Translation.X, link.Translation.Y, link.Translation.Z)
			transforms[link.ID] = frame.NewStaticFrame(link.ID, nil, frame.NewPoseFromTransform(q))
		}

		// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
		for _, joint := range m.Model.Joints {

			// TODO(pl): Make this a switch once we support more than one joint type
			if joint.Type == "revolute" {
				
				rev := frame.NewRevoluteFrame(joint.ID, nil, spatialmath.R4AA{RX: joint.Axis.X, RY: joint.Axis.Y, RZ: joint.Axis.Z})
				parentMap[joint.ID] = joint.Parent

				rev.SetLimits(joint.Min*math.Pi/180, joint.Max*math.Pi/180)

				transforms[joint.ID] = rev
			} else {
				return nil, errors.Errorf("unsupported joint type detected: %v", joint.Type)
			}
		}
	} else if m.Model.KinParamType == "DH" {
		for _, dh := range m.Model.DHParams {

			// Joint part of DH param
			jointID := dh.ID + "_j"
			parentMap[jointID] = dh.Parent
			j := frame.NewRevoluteFrame(jointID, nil, spatialmath.R4AA{RX: 0, RY: 0, RZ: 1})
			j.SetLimits(dh.Min*math.Pi/180, dh.Max*math.Pi/180)
			transforms[jointID] = j

			// Link part of DH param
			linkID := dh.ID
			linkQuat := spatialmath.NewDualQuaternionFromDH(dh.A, dh.D, dh.Alpha)
			
			transforms[linkID] = frame.NewStaticFrame(linkID, j, frame.NewPoseFromTransform(linkQuat))
		}
	} else {
		return nil, errors.Errorf("unsupported param type: %s, supported params are SVA and DH", m.Model.KinParamType)
	}
	
	for k,v := range(parentMap){
		if v == world{
			transforms[k] = frame.WrapFrame(transforms[k], worldFrame)
		}else{
			transforms[k] = frame.WrapFrame(transforms[k], transforms[v])
		}
	}
	

	// Determine which transforms have no children
	parents := make(map[string]frame.Frame)
	// First create a copy of the map
	for id, trans := range transforms {
		parents[id] = trans
	}
	// Now remove all parents
	for _, trans := range transforms {
		delete(parents, trans.Parent().Name())
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
	orderedTransforms := []frame.Frame{nextTransform}
	seen[eename] = true
	for {
		parent := nextTransform.Parent().Name()
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
