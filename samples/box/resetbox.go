package main

import (
	"context"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	webserver "go.viam.com/core/web/server"
	_ "go.viam.com/core/board"
	_ "go.viam.com/core/board/detector"
	_ "go.viam.com/core/robots/xarm"

	pb "go.viam.com/core/proto/api/v1"
	_ "go.viam.com/core/rimage/imagesource"
	"go.viam.com/core/robot"
	
	"github.com/edaniels/golog"
)

var(
	logger = golog.NewDevelopmentLogger("resetbox1")
	startPos = &pb.JointPositions{Degrees: []float64{0, -13, -42, 0, 45, 0}}
	safeDumpPos = &pb.JointPositions{Degrees: []float64{0, -43, -71, 0, 98, 0}}
	grabReadyPos = &pb.JointPositions{Degrees: []float64{-180, -26.8, -33, 0.2, 51, 0}}
	cube1grab = &pb.JointPositions{Degrees: []float64{-183, 16.9, -41.1, 2, 26.75, 0}}
	cube2grab = &pb.JointPositions{Degrees: []float64{-184.8, 20, -30.2, -5.7, -5.7, -0.2}}
	cube1place = &pb.JointPositions{Degrees: []float64{-84.75, 26.5, -29.9, -80.3, -23.27, -2.75}}
	cube1placePost = &pb.JointPositions{Degrees: []float64{-84.75, 26.5, -29.9, -80.3, -32.27, -2.75}}
	cube2place = &pb.JointPositions{Degrees: []float64{21.4, 41.3, -30.35, -5.7, -53.27, -0.2}}
	
	duckgrabFW = &pb.JointPositions{Degrees: []float64{-181.9, 20.45, -53.85, -3.5, 44.4, -0.08}}
	duckplaceFW = &pb.JointPositions{Degrees: []float64{-3.2, 32.8, -70.65, -9.3, 49, 165.12}}
	duckgrabREV = &pb.JointPositions{Degrees: []float64{-181.4, 18.15, -40.1, -3.5, 15.5, -0.08}}
	duckplaceREV = &pb.JointPositions{Degrees: []float64{-14.6, 27.3, -24.04, -11.8, -34.35, -9.7}}
)

func init() {
	action.RegisterAction("ResetBox", ResetBox)
}

// ResetBox will dump the playing field, 
func ResetBox(ctx context.Context, theRobot robot.Robot) {
	
	waitForResetReady(ctx, theRobot)
	
	rArm := theRobot.ArmByName("xArm6")
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	gripper := theRobot.GripperByName("vg1")
	gripper.Open(ctx)
	
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
}

// toggleTrigger will set the pin on which the arduino listens to high for 100ms, then back to low, to signal that the
// arduino should proceed with whatever the next step 
func toggleTrigger(ctx context.Context, theRobot robot.Robot) {
	resetBoard := theRobot.BoardByName("resetDriveBoard")
	resetBoard.GPIOSet("37", true)
	select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
	}
	resetBoard.GPIOSet("37", false)
}

// waitForReady waits for the arduino controlling the reset box to signal it is an item is available (first cubes,
// then duck).
// This function will block until the "ready" pin is high.
func waitForReady(ctx context.Context, theRobot robot.Robot) {
	select {
		case <-ctx.Done():
			return
		case <-time.After(1500 * time.Millisecond):
	}
	resetBoard := theRobot.BoardByName("resetDriveBoard")
	for{
		select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
		}
		ready, _ := resetBoard.GPIOGet("35")
		if ready{
			return
		}
	}
}

// waitForResetReady waits for the arduino controlling the reset box to signal it is ready for a new reset cycle.
// Strobing means it is ready for a new reset cycle to begin.
// This function will block until the "ready" pin has strobed 30 times.
func waitForResetReady(ctx context.Context, theRobot robot.Robot) {
	resetBoard := theRobot.BoardByName("resetDriveBoard")
	interrupt := resetBoard.DigitalInterrupt("ready")
	ticks := interrupt.Value()
	for interrupt.Value() < ticks + 30 {
		select {
			case <-ctx.Done():
			case <-time.After(100 * time.Millisecond):
		}
	}
}

func resetCube(ctx context.Context, theRobot robot.Robot) {
	rArm := theRobot.ArmByName("xArm6")
	gripper := theRobot.GripperByName("vg1")
	
	// Grab cube 1 and reset it on the field
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube1grab)
	gripper.Grab(ctx)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube1place)
	gripper.Open(ctx)
	rArm.MoveToJointPositions(ctx, cube1placePost)
	
	// Grab cube 2 and reset it on the field
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube2grab)
	gripper.Grab(ctx)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, cube2place)
	gripper.Open(ctx)
}

func resetDuck(ctx context.Context, theRobot robot.Robot) {
	rArm := theRobot.ArmByName("xArm6")
	gripper := theRobot.GripperByName("vg1")
	
	// We move into position while the box is resetting the duck to save time
	rArm.MoveToJointPositions(ctx, safeDumpPos)
	rArm.MoveToJointPositions(ctx, grabReadyPos)
	rArm.MoveToJointPositions(ctx, duckgrabFW)
	// Wait for duck to be available
	waitForReady(ctx, theRobot)
	
	// Try to grab- this should succeed if the duck is facing forwards, and fail if facing backwards
	grabbed, _ := gripper.Grab(ctx)
	if grabbed{
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		rArm.MoveToJointPositions(ctx, duckplaceFW)
		gripper.Open(ctx)
	}else{
		// Duck was facing backwards. Grab where the backwards-facing head should be
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		gripper.Open(ctx)
		rArm.MoveToJointPositions(ctx, duckgrabREV)
		gripper.Grab(ctx)
		rArm.MoveToJointPositions(ctx, grabReadyPos)
		rArm.MoveToJointPositions(ctx, duckplaceREV)
		gripper.Open(ctx)
	}
}

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}
