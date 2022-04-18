// This is the older prototype resetbox.
// In this, the arm controlled by a Pi and this code
// The arduino board (restbox.ino) controls the other motors and functions
// Communication between the two is via two GPIO lines.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gripper"
	componentpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/robot"
	webserver "go.viam.com/rdk/web/server"
)

var (
	logger   = golog.NewDevelopmentLogger("resetbox1")
	startPos = []*componentpb.JointPosition{
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{13},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-42},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{45},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	safeDumpPos = []*componentpb.JointPosition{
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-43},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-71},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{98},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	grabReadyPos = []*componentpb.JointPosition{
		{
			Parameters: []float64{-180},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-26.8},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-33},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{51},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	cube1grab = []*componentpb.JointPosition{
		{
			Parameters: []float64{-183},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{16.9},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-41.1},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{26.75},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{0},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	cube2grab = []*componentpb.JointPosition{
		{
			Parameters: []float64{-184.8},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{20},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-30.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-5.7},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-5.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-0.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	cube1place = []*componentpb.JointPosition{
		{
			Parameters: []float64{-84.75},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{26.5},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-29.9},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-80.3},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-23.27},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-2.75},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	cube1placePost = []*componentpb.JointPosition{
		{
			Parameters: []float64{-84.75},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{26.5},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-29.9},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-80.3},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-32.27},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-2.75},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	cube2place = []*componentpb.JointPosition{
		{
			Parameters: []float64{21.4},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{41.3},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-30.35},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-5.7},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-53.27},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-0.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}

	duckgrabFW = []*componentpb.JointPosition{
		{
			Parameters: []float64{-181.9},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{20.45},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-53.85},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-3.5},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{44.4},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-0.08},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	duckplaceFW = []*componentpb.JointPosition{
		{
			Parameters: []float64{-3.2},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{32.8},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-70.65},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-9.3},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{49},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{165.12},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	duckgrabREV = []*componentpb.JointPosition{
		{
			Parameters: []float64{-181.4},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{18.15},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-40.1},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-3.5},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{15.5},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-0.08},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}
	duckplaceREV = []*componentpb.JointPosition{
		{
			Parameters: []float64{-14.6},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{27.3},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-24.04},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-11.8},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-34.35},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
		{
			Parameters: []float64{-9.7},
			JointType:  componentpb.JointPosition_JOINT_TYPE_REVOLUTE,
		},
	}

	armName     = "xArm6"
	gripperName = "vg1"
	boardName   = "resetDriveBoard"
	readyPin    = "ready"
)

// ResetBox will dump the playing field,.
//nolint:deadcode
func ResetBox(ctx context.Context, theRobot robot.Robot) error {
	waitForResetReady(ctx, theRobot)

	rArm, err := arm.FromRobot(theRobot, armName)
	if err != nil {
		return err
	}
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	gGripper, err := gripper.FromRobot(theRobot, gripperName)
	if err != nil {
		return err
	}
	gGripper.Open(ctx)

	// Dump the platform,
	toggleTrigger(ctx, theRobot)

	// Wait for cubes to be available
	waitForReady(ctx, theRobot)

	// Grab the object where it ought to be and replace it onto the field
	resetCube(ctx, theRobot)

	toggleTrigger(ctx, theRobot)

	resetDuck(ctx, theRobot)
	toggleTrigger(ctx, theRobot)
	rArm.MoveToJointPositions(ctx, startPos)
	return nil
}

// toggleTrigger will set the pin on which the arduino listens to high for 100ms, then back to low, to signal that the
// arduino should proceed with whatever the next step.
func toggleTrigger(ctx context.Context, theRobot robot.Robot) error {
	resetBoard, err := board.FromRobot(theRobot, boardName)
	if err != nil {
		return err
	}
	p, err := resetBoard.GPIOPinByName("37")
	if err != nil {
		return err
	}
	if err := p.Set(ctx, true); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
	}
	return p.Set(ctx, false)
}

// waitForReady waits for the arduino controlling the reset box to signal it is an item is available (first cubes,
// then duck).
// This function will block until the "ready" pin is high.
func waitForReady(ctx context.Context, theRobot robot.Robot) error {
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(1500 * time.Millisecond):
	}
	resetBoard, err := board.FromRobot(theRobot, boardName)
	if err != nil {
		return err
	}
	p, err := resetBoard.GPIOPinByName("35")
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(100 * time.Millisecond):
		}
		ready, _ := p.Get(ctx)
		if ready {
			return nil
		}
	}
}

// waitForResetReady waits for the arduino controlling the reset box to signal it is ready for a new reset cycle.
// Strobing means it is ready for a new reset cycle to begin.
// This function will block until the "ready" pin has strobed 30 times.
func waitForResetReady(ctx context.Context, theRobot robot.Robot) error {
	resetBoard, err := board.FromRobot(theRobot, boardName)
	if err != nil {
		return err
	}
	interrupt, ok := resetBoard.DigitalInterruptByName("ready")
	if !ok {
		return fmt.Errorf("failed to find interrupt %s", readyPin)
	}
	ticks, err := interrupt.Value(ctx)
	if err != nil {
		return err
	}
	for {
		interruptVal, err := interrupt.Value(ctx)
		if err != nil {
			return err
		}
		if interruptVal >= ticks+30 {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

func resetCube(ctx context.Context, theRobot robot.Robot) error {
	rArm, err := arm.FromRobot(theRobot, armName)
	if err != nil {
		return err
	}
	rGripper, err := gripper.FromRobot(theRobot, gripperName)
	if err != nil {
		return err
	}

	// Grab cube 1 and reset it on the field
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube1grab)
	rGripper.Grab(ctx)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube1place)
	rGripper.Open(ctx)
	rArm.MoveToJointPositions(ctx, cube1placePost)

	// Grab cube 2 and reset it on the field
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube2grab)
	rGripper.Grab(ctx)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube2place)
	return rGripper.Open(ctx)
}

func resetDuck(ctx context.Context, theRobot robot.Robot) error {
	rArm, err := arm.FromRobot(theRobot, armName)
	if err != nil {
		return err
	}
	rGripper, err := gripper.FromRobot(theRobot, gripperName)
	if err != nil {
		return err
	}

	// We move into position while the box is resetting the duck to save time
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, duckgrabFW)
	// Wait for duck to be available
	waitForReady(ctx, theRobot)

	// Try to grab- this should succeed if the duck is facing forwards, and fail if facing backwards
	grabbed, _ := rGripper.Grab(ctx)
	if grabbed {
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		rArm.MoveToJointPositions(ctx, duckplaceFW)
		rGripper.Open(ctx)
	} else {
		// Duck was facing backwards. Grab where the backwards-facing head should be
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		rGripper.Open(ctx)
		rArm.MoveToJointPositions(ctx, duckgrabREV)
		rGripper.Grab(ctx)
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		rArm.MoveToJointPositions(ctx, duckplaceREV)
		rGripper.Open(ctx)
	}
	return nil
}

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}
