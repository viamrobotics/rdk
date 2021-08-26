package referenceframe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/geo/r3"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/utils"
)

type FrameFile struct {
	Name        string `json:"id"`
	Type        string `json:"type"`
	Parent      string `json:"parent"`
	Translation struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"translation"`
	SetOrientation bool `json:"setorientation"`
	Orientation    struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
		T float64 `json:"th"`
	} `json:"orientation"`
	Axis struct { // for revolute frames
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"axis"`
	Axes struct { // for prismatic frames
		X bool `json:"x"`
		Y bool `json:"y"`
		Z bool `json:"z"`
	} `json:"axes"`
	Min []float64 `json:"min"`
	Max []float64 `json:"max"`
}

type FrameSystemFile struct {
	Name   string      `json:"name"`
	Frames []FrameFile `json:"frames"`
}

func NewFrameSystemFromJSON(jsonPath string) (FrameSystem, error) {
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("error opening JSON file %s - %w", jsonPath, err)
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON data - %w", err)
	}
	return NewFrameSystemFromBytes(byteValue)
}

func NewFrameSystemFromBytes(byteJSON []byte) (FrameSystem, error) {
	frameConfig := &FrameSystemFile{}
	// parse into struct
	err := json.Unmarshal(byteJSON, frameConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall json file - %w", err)
	}
	// get a map of the names of the Frames, and a map of the parent frames to the children Frames from the config file
	names, children, err := buildFramesFromConfig(frameConfig)
	if err != nil {
		return nil, err
	}
	// use a stack to populate the frame system
	stack := make([]string, 0)
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children["world"]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'.")
	}
	stack = append(stack, "world")
	// begin adding frames to the frame system
	fs := NewEmptySimpleFrameSystem(frameConfig.Name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		} else {
			visited[parent] = true
		}
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			err := fs.AddFrame(frame, fs.GetFrame(parent))
			if err != nil {
				return nil, err
			}
		}
	}
	// ensure that there are no disconnected frames
	if len(visited) != len(names) {
		return nil, fmt.Errorf("the system is not fully connected, expected %d frames but frame system has %d", len(names), len(visited))
	}
	return fs, nil
}

func buildFramesFromConfig(frameConfig *FrameSystemFile) (map[string]bool, map[string][]Frame, error) {
	// build each frame in the config file
	children := make(map[string][]Frame)
	names := make(map[string]bool)
	names["world"] = true
	for _, f := range frameConfig.Frames {
		var frame Frame
		switch f.Type {
		case "":
			frame = makeStaticFrame(f)
		case "static":
			frame = makeStaticFrame(f)
		case "prismatic":
			frame = makePrismaticFrame(f)
		case "revolute":
			frame = makeRevoluteFrame(f)
		default:
			return nil, nil, fmt.Errorf("do not know how to create Frame of type %s", f.Type)
		}
		// check to see if there are no repeated names
		if _, ok := names[frame.Name()]; ok {
			return nil, nil, fmt.Errorf("cannot have more than one Frame with name %s", frame.Name())
		}
		names[frame.Name()] = true
		// store the children Frames in a list to build the tree later
		children[f.Parent] = append(children[f.Parent], frame)
	}
	return names, children, nil

}

func makeStaticFrame(f FrameFile) Frame {
	var pose spatial.Pose

	// get the translation vector
	translation := r3.Vector{f.Translation.X, f.Translation.Y, f.Translation.Z}

	// get the orientation if there is one
	if f.SetOrientation {
		ov := &spatial.OrientationVec{f.Orientation.T, f.Orientation.X, f.Orientation.Y, f.Orientation.Z}
		pose = spatial.NewPoseFromOrientationVector(translation, ov)
	} else {
		pose = spatial.NewPoseFromPoint(translation)
	}
	// create and set the frame
	return NewStaticFrame(f.Name, pose)
}

func makePrismaticFrame(f FrameFile) Frame {
	// get the translation axes
	axes := []bool{f.Axes.X, f.Axes.Y, f.Axes.Z}

	// create and set the frame
	prism := NewPrismaticFrame(f.Name, axes)
	prism.SetLimits(f.Min, f.Max)
	return prism
}

func makeRevoluteFrame(f FrameFile) Frame {
	// get the rotation axis
	axis := spatial.R4AA{RX: f.Axis.X, RY: f.Axis.Y, RZ: f.Axis.Z}

	// create and set the frame
	rev := NewRevoluteFrame(f.Name, axis)
	rev.SetLimits(f.Min[0], f.Max[0])
	return rev
}
