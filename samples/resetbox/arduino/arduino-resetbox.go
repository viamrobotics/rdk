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
	logger         = golog.NewDevelopmentLogger("resetbox1")
	startPos       = &componentpb.JointPositions{Degrees: []float64{0, -13, -42, 0, 45, 0}}
	safeDumpPos    = &componentpb.JointPositions{Degrees: []float64{0, -43, -71, 0, 98, 0}}
	grabReadyPos   = &componentpb.JointPositions{Degrees: []float64{-180, -26.8, -33, 0.2, 51, 0}}
	cube1grab      = &componentpb.JointPositions{Degrees: []float64{-183, 16.9, -41.1, 2, 26.75, 0}}
	cube2grab      = &componentpb.JointPositions{Degrees: []float64{-184.8, 20, -30.2, -5.7, -5.7, -0.2}}
	cube1place     = &componentpb.JointPositions{Degrees: []float64{-84.75, 26.5, -29.9, -80.3, -23.27, -2.75}}
	cube1placePost = &componentpb.JointPositions{Degrees: []float64{-84.75, 26.5, -29.9, -80.3, -32.27, -2.75}}
	cube2place     = &componentpb.JointPositions{Degrees: []float64{21.4, 41.3, -30.35, -5.7, -53.27, -0.2}}

	duckgrabFW   = &componentpb.JointPositions{Degrees: []float64{-181.9, 20.45, -53.85, -3.5, 44.4, -0.08}}
	duckplaceFW  = &componentpb.JointPositions{Degrees: []float64{-3.2, 32.8, -70.65, -9.3, 49, 165.12}}
	duckgrabREV  = &componentpb.JointPositions{Degrees: []float64{-181.4, 18.15, -40.1, -3.5, 15.5, -0.08}}
	duckplaceREV = &componentpb.JointPositions{Degrees: []float64{-14.6, 27.3, -24.04, -11.8, -34.35, -9.7}}

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
