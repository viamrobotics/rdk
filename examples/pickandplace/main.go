// Package main implements a vision-guided pick-and-place program.
//
// The program connects to a Viam robot that has:
//   - A robot arm ("myArm")
//   - A gripper mounted at the arm's end ("myGripper")
//   - A camera mounted on the wrist pointing downward ("wristCamera")
//   - A vision service segmenter configured to detect cups ("cupDetector")
//   - The builtin motion service
//   - A frame system with the camera properly parented to the gripper/arm frame
//
// Execution flow:
//  1. Open the gripper and move to a known scan pose (joints pre-configured).
//  2. Ask the vision service to segment objects visible in the wrist camera.
//  3. Find the first object whose geometry label is "cup".
//  4. Transform that 3D pose from the camera frame to the world frame.
//  5. Move the gripper to a pregrasp (approach) pose directly above the cup.
//  6. Lower to the grasp pose and grab.
//  7. Lift, travel to a hardcoded place position, lower, release.
//  8. Retreat upward and return to the scan pose.
//
// All position constants are in millimetres; angles are in radians.
// Update the configuration block at the top of this file for your setup.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	vizvision "go.viam.com/rdk/vision"
)

// ── Robot configuration ──────────────────────────────────────────────────────
// Update these constants to match your machine's component names and address.
const (
	robotAddress      = "localhost:8080" // or your machine's cloud address
	armName           = "myArm"
	gripperName       = "myGripper"
	cameraName        = "wristCamera"
	visionServiceName = "cupDetector" // segmenter vision service
	motionServiceName = "builtin"

	// cupLabel is the geometry label the vision segmenter assigns to cups.
	cupLabel = "cup"
)

// scanJointPositionsRad are the joint angles (radians) for the initial scan
// pose. At this pose the wrist camera should face downward over the workspace.
// The example below is for a 7-DOF arm; adjust the slice length and values
// for your specific arm model.
var scanJointPositionsRad = []referenceframe.Input{
	0, -0.785, 0, -2.356, 0, 1.571, 0.785,
}

// ── Motion offset constants (mm) ─────────────────────────────────────────────
const (
	pregraspOffsetMM = 150.0 // hover height above the cup centroid before grasping
	liftHeightMM     = 200.0 // how far to lift the cup above its pick location
)

// ── Hardcoded place location (world frame, mm) ────────────────────────────────
// Update these to a location that is within reach of your arm.
const (
	placeX = 400.0
	placeY = 0.0
	placeZ = 100.0 // height of the surface where the cup will be set down
)

// ─────────────────────────────────────────────────────────────────────────────

func main() {
	logger := logging.NewLogger("pick-and-place")
	ctx := context.Background()

	// ── Connect to the robot ──────────────────────────────────────────────────
	robot, err := client.New(ctx, robotAddress, logger)
	if err != nil {
		logger.Fatalw("failed to connect to robot", "error", err)
	}
	defer func() {
		if err := robot.Close(ctx); err != nil {
			logger.Warnw("failed to close robot connection", "error", err)
		}
	}()
	logger.Info("connected to robot")

	// ── Acquire component and service handles ─────────────────────────────────
	myArm, err := arm.FromProvider(robot, armName)
	if err != nil {
		logger.Fatalw("failed to get arm", "error", err)
	}

	myGripper, err := gripper.FromProvider(robot, gripperName)
	if err != nil {
		logger.Fatalw("failed to get gripper", "error", err)
	}

	visService, err := vision.FromProvider(robot, visionServiceName)
	if err != nil {
		logger.Fatalw("failed to get vision service", "error", err)
	}

	motionService, err := motion.FromProvider(robot, motionServiceName)
	if err != nil {
		logger.Fatalw("failed to get motion service", "error", err)
	}

	fsService, err := framesystem.FromProvider(robot)
	if err != nil {
		logger.Fatalw("failed to get frame system service", "error", err)
	}

	// ── Step 1: Open gripper and move to scan pose ────────────────────────────
	logger.Info("opening gripper")
	if err := myGripper.Open(ctx, nil); err != nil {
		logger.Fatalw("failed to open gripper", "error", err)
	}

	logger.Info("moving to scan pose")
	if err := myArm.MoveToJointPositions(ctx, scanJointPositionsRad, nil); err != nil {
		logger.Fatalw("failed to move to scan pose", "error", err)
	}

	// Give the arm time to settle so the camera image is stable.
	time.Sleep(500 * time.Millisecond)

	// ── Step 2: Detect and locate the cup ─────────────────────────────────────
	logger.Info("detecting cup using vision service")
	cupPoseInWorld, err := findCupInWorldFrame(ctx, visService, fsService, logger)
	if err != nil {
		logger.Fatalw("failed to locate cup", "error", err)
	}
	logger.Infow("cup located", "worldPose", cupPoseInWorld)

	cupPos := cupPoseInWorld.Pose().Point()
	cupOri := cupPoseInWorld.Pose().Orientation()

	// ── Step 3: Move to pregrasp pose (above the cup) ─────────────────────────
	pregraspPose := spatialmath.NewPose(
		r3.Vector{X: cupPos.X, Y: cupPos.Y, Z: cupPos.Z + pregraspOffsetMM},
		cupOri,
	)
	logger.Info("moving to pregrasp pose")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, pregraspPose),
	}); err != nil {
		logger.Fatalw("failed to move to pregrasp pose", "error", err)
	}

	// ── Step 4: Lower to grasp pose ───────────────────────────────────────────
	graspPose := spatialmath.NewPose(cupPos, cupOri)
	logger.Info("lowering to grasp pose")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, graspPose),
	}); err != nil {
		logger.Fatalw("failed to move to grasp pose", "error", err)
	}

	// ── Step 5: Grab the cup ──────────────────────────────────────────────────
	logger.Info("grabbing cup")
	grabbed, err := myGripper.Grab(ctx, nil)
	if err != nil {
		logger.Fatalw("failed to grab", "error", err)
	}
	if !grabbed {
		logger.Warn("gripper reports nothing was grabbed; proceeding anyway")
	}

	// ── Step 6: Lift the cup ──────────────────────────────────────────────────
	liftPose := spatialmath.NewPose(
		r3.Vector{X: cupPos.X, Y: cupPos.Y, Z: cupPos.Z + liftHeightMM},
		cupOri,
	)
	logger.Info("lifting cup")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, liftPose),
	}); err != nil {
		logger.Fatalw("failed to lift cup", "error", err)
	}

	// ── Step 7: Move to place approach (above target position) ────────────────
	placeApproachPose := spatialmath.NewPose(
		r3.Vector{X: placeX, Y: placeY, Z: placeZ + pregraspOffsetMM},
		cupOri,
	)
	logger.Info("moving to place approach pose")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, placeApproachPose),
	}); err != nil {
		logger.Fatalw("failed to move to place approach pose", "error", err)
	}

	// ── Step 8: Lower to the place position ──────────────────────────────────
	placePose := spatialmath.NewPose(r3.Vector{X: placeX, Y: placeY, Z: placeZ}, cupOri)
	logger.Info("lowering to place position")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, placePose),
	}); err != nil {
		logger.Fatalw("failed to lower to place position", "error", err)
	}

	// ── Step 9: Release the cup ───────────────────────────────────────────────
	logger.Info("releasing cup")
	if err := myGripper.Open(ctx, nil); err != nil {
		logger.Fatalw("failed to open gripper after placing", "error", err)
	}

	// ── Step 10: Retreat upward from the place position ───────────────────────
	retreatPose := spatialmath.NewPose(
		r3.Vector{X: placeX, Y: placeY, Z: placeZ + pregraspOffsetMM},
		cupOri,
	)
	logger.Info("retreating from place position")
	if _, err := motionService.Move(ctx, motion.MoveReq{
		ComponentName: gripperName,
		Destination:   referenceframe.NewPoseInFrame(referenceframe.World, retreatPose),
	}); err != nil {
		// Non-fatal: if retreat fails, still try to return to scan pose.
		logger.Warnw("failed to retreat; will attempt to return to scan pose", "error", err)
	}

	// ── Step 11: Return to scan pose ──────────────────────────────────────────
	logger.Info("returning to scan pose")
	if err := myArm.MoveToJointPositions(ctx, scanJointPositionsRad, nil); err != nil {
		logger.Warnw("failed to return to scan pose", "error", err)
	}

	logger.Info("pick-and-place complete")
}

// findCupInWorldFrame calls the vision service to segment objects visible
// through the wrist camera, selects the first object labelled "cup", and
// transforms its centroid pose from the camera frame to the world frame.
//
// The vision service must be configured as a 3-D object segmenter (e.g. the
// built-in radius_clustering_segmenter or an ML-based detector that also
// returns point clouds via GetObjectPointClouds).  The returned geometry pose
// is the centroid of the detected point-cloud cluster in the camera frame.
func findCupInWorldFrame(
	ctx context.Context,
	visService vision.Service,
	fsService framesystem.Service,
	logger logging.Logger,
) (*referenceframe.PoseInFrame, error) {
	objects, err := visService.GetObjectPointClouds(ctx, cameraName, nil)
	if err != nil {
		return nil, fmt.Errorf("GetObjectPointClouds failed: %w", err)
	}
	if len(objects) == 0 {
		return nil, fmt.Errorf("no objects detected by vision service on camera %q", cameraName)
	}

	for _, obj := range objects {
		if obj.Geometry == nil {
			continue
		}
		label := obj.Geometry.Label()
		logger.Debugw("detected object", "label", label, "centroid", obj.Geometry.Pose().Point())

		if label != cupLabel {
			continue
		}

		// The geometry pose returned by GetObjectPointClouds is expressed in
		// the reference frame of the camera that was passed to the segmenter.
		cupPoseInCamera := referenceframe.NewPoseInFrame(cameraName, obj.Geometry.Pose())

		// Use the frame system to transform the cup's pose into the world frame,
		// taking into account the current joint positions of the arm.
		cupPoseInWorld, err := fsService.TransformPose(ctx, cupPoseInCamera, referenceframe.World, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to transform cup pose to world frame: %w", err)
		}
		return cupPoseInWorld, nil
	}

	return nil, fmt.Errorf("no object with label %q detected; found: %v", cupLabel, objectLabels(objects))
}

// objectLabels returns the geometry labels from a slice of vision objects,
// used to produce informative error messages when no matching object is found.
func objectLabels(objects []*vizvision.Object) []string {
	labels := make([]string, 0, len(objects))
	for _, obj := range objects {
		if obj.Geometry != nil {
			labels = append(labels, obj.Geometry.Label())
		}
	}
	return labels
}
