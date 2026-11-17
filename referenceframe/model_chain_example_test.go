package referenceframe_test

import (
	"fmt"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// exampleMachine is a ur5e named arm with a two finger gripper named hand on its flange, built the
// way a frame system service builds a machine's model.
func exampleMachine() referenceframe.Model {
	arm, _ := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	hand, _ := referenceframe.ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	onTable := referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{Z: 100}), "arm", nil)
	onFlange := referenceframe.NewLinkInFrame("arm", spatialmath.NewPoseFromPoint(r3.Vector{Z: 20}), "hand", nil)
	fs, _ := referenceframe.NewFrameSystem("machine", []*referenceframe.FrameSystemPart{
		{FrameConfig: onTable, ModelFrame: arm},
		{FrameConfig: onFlange, ModelFrame: hand},
	}, nil)
	machine, _ := referenceframe.NewModel("machine", fs, "hand")
	return machine
}

//nolint:forbidigo // an example test prints what godoc shows as its output
func printChain(chain []referenceframe.ChainFrame) {
	for _, cf := range chain {
		switch {
		case cf.Mimic != nil:
			fmt.Println(cf.Frame.Name(), "follows input", cf.Mimic.Source)
		case len(cf.Frame.DoF()) > 0:
			fmt.Println(cf.Frame.Name(), "input", cf.Offset)
		default:
			fmt.Println(cf.Frame.Name())
		}
	}
}

// The chain of a model built over parts carries each part's frames under the names its own file
// gives them, with a static frame between parts named after the part it leads to.
func ExampleSimpleModel_Chain() {
	chain, _ := exampleMachine().Chain()
	printChain(chain)
	// Output:
	// arm_origin
	// base_link
	// shoulder_pan_joint input 0
	// shoulder_link
	// shoulder_lift_joint input 1
	// upper_arm_link
	// elbow_joint input 2
	// forearm_link
	// wrist_1_joint input 3
	// wrist_1_link
	// wrist_2_joint input 4
	// wrist_2_link
	// wrist_3_joint input 5
	// ee_link
	// hand_origin
	// base
	// tcp
}

// A frame inside a part is named part:frame, and a chain between two of them is cut inside the
// parts.
func ExampleSimpleModel_ChainFrom() {
	chain, _ := exampleMachine().ChainFrom("arm:wrist_3_joint", "hand:right_finger")
	printChain(chain)
	// Output:
	// ee_link
	// hand_origin
	// base
	// right_joint follows input 6
	// right_finger
}
