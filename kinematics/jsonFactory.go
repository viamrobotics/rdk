package kinematics

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"strconv"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type AutoGenerated struct {
	Model struct {
		Manufacturer string `json:"manufacturer"`
		Name         string `json:"name"`
		Links        []struct {
			ID       int    `json:"id"`
			Parent   string `json:"parent"`
			Rotation struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
				Z float64 `json:"z"`
			} `json:"rotation"`
			Translation struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
				Z float64 `json:"z"`
			} `json:"translation"`
		} `json:"links"`
		Joints []struct {
			ID     int    `json:"id"`
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
		// Home position of joints. Optional, if not provided will be set to all 0.
		Home []float64 `json:"home"`
	} `json:"model"`
}

func ParseJSONFile(filename string, logger golog.Logger) (*Model, error) {
	model := NewModel()
	id2frame := make(map[string]*Frame)
	m := AutoGenerated{}

	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		logger.Error("failed to read json file")
	}

	err = json.Unmarshal(jsonData, &m)
	if err != nil {
		logger.Error("failed to unmarshall json file")
	}

	model.manufacturer = m.Model.Manufacturer
	model.name = m.Model.Name

	// Create world frame
	wFrame := NewFrame()
	wFrame.IsWorld = true
	model.Add(wFrame)
	id2frame["world"] = wFrame

	for _, fixed := range m.Model.Links {
		frame := NewFrame()
		model.Add(frame)
		nodeID := "fixed" + strconv.Itoa(fixed.ID)
		id2frame[nodeID] = frame
		frame.Name = nodeID
	}
	for _, joint := range m.Model.Joints {
		frame := NewFrame()
		model.Add(frame)
		nodeID := "joint" + strconv.Itoa(joint.ID)
		id2frame[nodeID] = frame
		frame.Name = nodeID
	}

	for _, fixed := range m.Model.Links {
		frameA := id2frame[fixed.Parent]
		frameB := id2frame["fixed"+strconv.Itoa(fixed.ID)]

		fixedT := NewTransform()
		fixedT.SetName("fixed" + strconv.Itoa(fixed.ID))

		fixedT.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
		model.Edges[fixedT.GetEdgeDescriptor()] = fixedT
		fixedT.t = kinmath.NewQuatTransFromRotation(fixed.Rotation.X, fixed.Rotation.Y, fixed.Rotation.Z)
		fixedT.t.SetX(fixed.Translation.X / 2)
		fixedT.t.SetY(fixed.Translation.Y / 2)
		fixedT.t.SetZ(fixed.Translation.Z / 2)
	}

	// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
	for _, joint := range m.Model.Joints {

		// TODO(pl): Make this a switch once we support more than one joint type
		if joint.Type == "revolute" {
			// TODO(pl): Add speed, wraparound, etc
			frameA := id2frame[joint.Parent]
			frameB := id2frame["joint"+strconv.Itoa(joint.ID)]

			rev := NewJoint(1, 1)
			rev.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
			model.Edges[rev.GetEdgeDescriptor()] = rev

			rev.max = append(rev.max, joint.Max*math.Pi/180)
			rev.min = append(rev.min, joint.Min*math.Pi/180)

			// TODO(pl): Add default on z
			// TODO(pl): Enforce between 0 and 1
			rev.SpatialMat.Set(0, 0, joint.Axis.X)
			rev.SpatialMat.Set(1, 0, joint.Axis.Y)
			rev.SpatialMat.Set(2, 0, joint.Axis.Z)
			rev.SetAxesFromSpatial()

			rev.SetName("joint" + strconv.Itoa(joint.ID))
		} else {
			logger.Error("Unsupported joint type detected:", joint.Type)
		}
	}

	model.Update()
	if m.Model.Home != nil {
		model.Home = m.Model.Home
	} else {
		for i := 0; i < len(model.Joints); i++ {
			model.Home = append(model.Home, 0)
		}
	}

	return model, err
}
