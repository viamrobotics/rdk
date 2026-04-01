package worksheet

import (
	"fmt"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
)

// MakeLevels builds all game levels with pre-computed answers from real spatialmath calls.
func MakeLevels() []Level {
	return []Level{
		makeLevel1(),
		makeLevel2(),
		makeLevel3(),
		makeLevel4(),
		makeLevel5(),
	}
}

// ovd is a shorthand for creating an OrientationVectorDegrees.
func ovd(theta, ox, oy, oz float64) *spatialmath.OrientationVectorDegrees {
	return &spatialmath.OrientationVectorDegrees{Theta: theta, OX: ox, OY: oy, OZ: oz}
}

// Level 1: Pure Translation
func makeLevel1() Level {
	a1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: 100, Z: 0})
	b1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 100})
	r1 := spatialmath.Compose(a1, b1)

	a2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
	b2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 50, Z: 0})
	c2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 50})
	r2 := spatialmath.Compose(spatialmath.Compose(a2, b2), c2)

	a3 := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: -200, Z: 300})
	r3inv := spatialmath.PoseInverse(a3)

	a4 := spatialmath.NewPoseFromPoint(r3.Vector{X: 42, Y: 99, Z: -7})
	r4 := spatialmath.Compose(a4, spatialmath.PoseInverse(a4))

	return Level{
		Number:      1,
		Title:       "Pure Translation",
		Description: "Composing point-only poses — no rotation involved.\nWhat does Compose do when there is no orientation?",
		Questions: []Question{
			{
				Setup: `  a := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: 100, Z: 0})
  b := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 100})
  result := spatialmath.Compose(a, b)`,
				Answer:      FormatPose(r1),
				Explanation: "With no orientation on either pose, Compose adds the translation vectors.",
				InputPoses:  map[string]spatialmath.Pose{"a": a1, "b": b1},
				ResultPose:  r1,
			},
			{
				Setup: `  a := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
  b := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 50, Z: 0})
  c := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 50})
  result := spatialmath.Compose(spatialmath.Compose(a, b), c)`,
				Answer:      FormatPose(r2),
				Explanation: "Chaining three translations: each Compose adds the next vector.",
				InputPoses:  map[string]spatialmath.Pose{"a": a2, "b": b2, "c": c2},
				ResultPose:  r2,
			},
			{
				Setup: `  a := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: -200, Z: 300})
  result := spatialmath.PoseInverse(a)`,
				Answer:      FormatPose(r3inv),
				Explanation: "PoseInverse of a pure translation negates the vector.",
				InputPoses:  map[string]spatialmath.Pose{"a": a3},
				ResultPose:  r3inv,
			},
			{
				Setup: `  a := spatialmath.NewPoseFromPoint(r3.Vector{X: 42, Y: 99, Z: -7})
  result := spatialmath.Compose(a, spatialmath.PoseInverse(a))`,
				Answer:      FormatPose(r4),
				Explanation: "Composing any pose with its inverse always returns the zero pose (identity).",
				InputPoses:  map[string]spatialmath.Pose{"a": a4},
				ResultPose:  r4,
			},
		},
	}
}

// Level 2: Rotation from Origin
func makeLevel2() Level {
	angles := []float64{0, 90, 180, 270}
	point := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: 0, Z: 0})

	questions := make([]Question, 0, len(angles))
	for _, deg := range angles {
		rot := spatialmath.NewPose(r3.Vector{}, ovd(deg, 1, 0, 0))
		result := spatialmath.Compose(rot, point)

		setup := fmt.Sprintf(`  rot := spatialmath.NewPose(r3.Vector{}, &spatialmath.OrientationVectorDegrees{Theta: %g, OX: 1, OY: 0, OZ: 0})
  point := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: 0, Z: 0})
  result := spatialmath.Compose(rot, point)`, deg)

		questions = append(questions, Question{
			Setup:       setup,
			Answer:      FormatPose(result),
			Explanation: fmt.Sprintf("Rotating %g° around X, then translating 100 along local X.", deg),
			InputPoses:  map[string]spatialmath.Pose{"a": rot, "b": point},
			ResultPose:  result,
		})
	}

	return Level{
		Number:      2,
		Title:       "Rotation from Origin",
		Description: "How orientation affects where a subsequent translation ends up.\nThe first pose rotates the frame, the second translates in the rotated frame.",
		Questions:   questions,
	}
}

// Level 3: Compose with Orientations
func makeLevel3() Level {
	// Two poses with theta rotations around OZ
	a1 := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, ovd(45, 0, 0, 1))
	b1 := spatialmath.NewPose(r3.Vector{X: 50, Y: 0, Z: 0}, ovd(30, 0, 0, 1))
	r1 := spatialmath.Compose(a1, b1)

	// Non-commutativity: Compose(a, b) vs Compose(b, a)
	r2 := spatialmath.Compose(b1, a1)

	// PoseBetween
	a3 := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, ovd(45, 0, 0, 1))
	r3target := spatialmath.Compose(a3, b1)
	r3between := spatialmath.PoseBetween(a3, r3target)

	return Level{
		Number:      3,
		Title:       "Compose with Orientations",
		Description: "Both poses have orientations. Order matters!\nTheta rotates around the OV axis (in-line rotation).",
		Questions: []Question{
			{
				Setup: `  a := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 45, OX: 0, OY: 0, OZ: 1})
  b := spatialmath.NewPose(r3.Vector{X: 50, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 30, OX: 0, OY: 0, OZ: 1})
  result := spatialmath.Compose(a, b)`,
				Answer:      FormatPose(r1),
				Explanation: "a's 45° rotation around Z rotates b's translation. The thetas add (45+30=75°).",
				InputPoses:  map[string]spatialmath.Pose{"a": a1, "b": b1},
				ResultPose:  r1,
			},
			{
				Setup: `  // Same a and b as before, but reversed!
  a := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 45, OX: 0, OY: 0, OZ: 1})
  b := spatialmath.NewPose(r3.Vector{X: 50, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 30, OX: 0, OY: 0, OZ: 1})
  result := spatialmath.Compose(b, a)  // NOTE: b first, then a!`,
				Answer:      FormatPose(r2),
				Explanation: "Compose(a,b) != Compose(b,a)! Spatial composition is NOT commutative.\n  The orientation of the first pose rotates the second's translation differently.",
				InputPoses:  map[string]spatialmath.Pose{"a": a1, "b": b1},
				ResultPose:  r2,
			},
			{
				Setup: fmt.Sprintf(`  a := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 45, OX: 0, OY: 0, OZ: 1})
  target := <the result from question 1>  // %s
  result := spatialmath.PoseBetween(a, target)
  // PoseBetween finds b such that Compose(a, b) = target`, FormatPoint(r3target.Point())),
				Answer:      FormatPose(r3between),
				Explanation: "PoseBetween(a, target) recovers b — the transform that takes you from a to target.",
				InputPoses:  map[string]spatialmath.Pose{"a": a3, "b": r3target},
				ResultPose:  r3between,
			},
		},
	}
}

// Level 4: Full 3D Composition
func makeLevel4() Level {
	// Compose with different orientation axes
	a1 := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 100}, ovd(0, 1, 0, 0))
	b1 := spatialmath.NewPose(r3.Vector{X: 50, Y: 0, Z: 0}, ovd(0, 0, 1, 0))
	r1 := spatialmath.Compose(a1, b1)

	// PoseBetween: given A and C, find B
	a2 := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, ovd(0, 0, 0, 1))
	c2 := spatialmath.NewPose(r3.Vector{X: 100, Y: 100, Z: 0}, ovd(90, 0, 0, 1))
	r2 := spatialmath.PoseBetween(a2, c2)

	// PoseInverse of an oriented pose
	a3 := spatialmath.NewPose(r3.Vector{X: 50, Y: 50, Z: 50}, ovd(45, 0, 0, 1))
	r3inv := spatialmath.PoseInverse(a3)

	// Verify: Compose(a, PoseInverse(a)) = identity
	r4 := spatialmath.Compose(a3, r3inv)

	return Level{
		Number:      4,
		Title:       "Full 3D Composition",
		Description: "Multi-axis orientations — OX, OY, OZ all in play.\nThese are the transformations that control robot arm movements.",
		Questions: []Question{
			{
				Setup: `  a := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 100}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 1, OY: 0, OZ: 0})
  b := spatialmath.NewPose(r3.Vector{X: 50, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 1, OZ: 0})
  result := spatialmath.Compose(a, b)`,
				Answer:      FormatPose(r1),
				Explanation: "a points along X-axis (OX=1), b points along Y-axis (OY=1).\n  a's orientation rotates b's translation into a different direction.",
				InputPoses:  map[string]spatialmath.Pose{"a": a1, "b": b1},
				ResultPose:  r1,
			},
			{
				Setup: `  a := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})
  c := spatialmath.NewPose(r3.Vector{X: 100, Y: 100, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 90, OX: 0, OY: 0, OZ: 1})
  result := spatialmath.PoseBetween(a, c)
  // What transform b makes Compose(a, b) = c?`,
				Answer:      FormatPose(r2),
				Explanation: "PoseBetween finds the relative transform between two poses.\n  Think of it as: 'what do I need to apply from a's frame to reach c?'",
				InputPoses:  map[string]spatialmath.Pose{"a": a2, "b": c2},
				ResultPose:  r2,
			},
			{
				Setup: `  a := spatialmath.NewPose(r3.Vector{X: 50, Y: 50, Z: 50}, &spatialmath.OrientationVectorDegrees{Theta: 45, OX: 0, OY: 0, OZ: 1})
  result := spatialmath.PoseInverse(a)`,
				Answer:      FormatPose(r3inv),
				Explanation: "PoseInverse of an oriented pose is NOT just negating the point!\n  The inverse orientation also rotates the negated translation.",
				InputPoses:  map[string]spatialmath.Pose{"a": a3},
				ResultPose:  r3inv,
			},
			{
				Setup: `  a := spatialmath.NewPose(r3.Vector{X: 50, Y: 50, Z: 50}, &spatialmath.OrientationVectorDegrees{Theta: 45, OX: 0, OY: 0, OZ: 1})
  result := spatialmath.Compose(a, spatialmath.PoseInverse(a))`,
				Answer:      FormatPose(r4),
				Explanation: "Always true: Compose(a, PoseInverse(a)) = zero pose, regardless of orientation.",
				InputPoses:  map[string]spatialmath.Pose{"a": a3},
				ResultPose:  r4,
			},
		},
	}
}

// Level 5: Practical Scenarios
func makeLevel5() Level {
	// Scenario 1: End effector + camera offset
	endEffector := spatialmath.NewPose(
		r3.Vector{X: 500, Y: 0, Z: 300},
		ovd(0, 0, 0, 1),
	)
	cameraOffset := spatialmath.NewPose(
		r3.Vector{X: 0, Y: 0, Z: 50},
		ovd(180, 0, 0, 1),
	)
	objectInCamera := spatialmath.NewPose(
		r3.Vector{X: 100, Y: 0, Z: 0},
		ovd(0, 0, 0, 1),
	)
	cameraInWorld := spatialmath.Compose(endEffector, cameraOffset)
	objectInWorld := spatialmath.Compose(cameraInWorld, objectInCamera)

	// Scenario 2: Object in world, find relative to arm
	armPose := spatialmath.NewPose(
		r3.Vector{X: 200, Y: 0, Z: 400},
		ovd(90, 0, 0, 1),
	)
	objectWorld := spatialmath.NewPose(
		r3.Vector{X: 300, Y: 100, Z: 400},
		ovd(0, 0, 0, 1),
	)
	objectRelative := spatialmath.Compose(spatialmath.PoseInverse(armPose), objectWorld)

	// Scenario 3: Chain of frames (base -> shoulder -> elbow -> hand)
	basePose := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 100}, ovd(0, 0, 0, 1))
	shoulderOffset := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 200}, ovd(0, 1, 0, 0))
	elbowOffset := spatialmath.NewPose(r3.Vector{X: 150, Y: 0, Z: 0}, ovd(0, 0, 0, 1))
	handInWorld := spatialmath.Compose(spatialmath.Compose(basePose, shoulderOffset), elbowOffset)

	return Level{
		Number:      5,
		Title:       "Practical Scenarios",
		Description: "Real-world robotics problems using the same Compose/PoseInverse/PoseBetween operations.",
		Questions: []Question{
			{
				Setup: `  // A robot arm's end effector is at this world pose:
  endEffector := spatialmath.NewPose(r3.Vector{X: 500, Y: 0, Z: 300}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})
  // A camera is mounted on the end effector with this offset:
  cameraOffset := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 50}, &spatialmath.OrientationVectorDegrees{Theta: 180, OX: 0, OY: 0, OZ: 1})
  // The camera sees an object at this relative pose:
  objectInCamera := spatialmath.NewPose(r3.Vector{X: 100, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})

  // Where is the object in world frame?
  cameraInWorld := spatialmath.Compose(endEffector, cameraOffset)
  result := spatialmath.Compose(cameraInWorld, objectInCamera)`,
				Answer:      FormatPose(objectInWorld),
				Explanation: "Chain the transforms: world->endEffector->camera->object.\n  Each Compose moves from one frame to the next.",
				InputPoses:  map[string]spatialmath.Pose{"a": endEffector, "b": cameraInWorld, "c": objectInCamera},
				ResultPose:  objectInWorld,
			},
			{
				Setup: `  // An arm is at this world pose:
  armPose := spatialmath.NewPose(r3.Vector{X: 200, Y: 0, Z: 400}, &spatialmath.OrientationVectorDegrees{Theta: 90, OX: 0, OY: 0, OZ: 1})
  // An object is at this world pose:
  objectWorld := spatialmath.NewPose(r3.Vector{X: 300, Y: 100, Z: 400}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})

  // What is the object's pose RELATIVE to the arm?
  result := spatialmath.Compose(spatialmath.PoseInverse(armPose), objectWorld)`,
				Answer:      FormatPose(objectRelative),
				Explanation: "To express B in A's frame: Compose(PoseInverse(A), B).\n  PoseInverse(arm) 'undoes' the arm's transform, then objectWorld applies.",
				InputPoses:  map[string]spatialmath.Pose{"a": armPose, "b": objectWorld},
				ResultPose:  objectRelative,
			},
			{
				Setup: `  // A robot arm has joints connected in a chain:
  basePose := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 100}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})
  shoulderOffset := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 200}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 1, OY: 0, OZ: 0})
  elbowOffset := spatialmath.NewPose(r3.Vector{X: 150, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1})

  // Where is the hand (after elbow) in world frame?
  result := spatialmath.Compose(spatialmath.Compose(basePose, shoulderOffset), elbowOffset)`,
				Answer:      FormatPose(handInWorld),
				Explanation: "Forward kinematics in a nutshell: chain Compose from base to end effector.\n  Each joint's orientation changes the direction of all subsequent translations.",
				InputPoses:  map[string]spatialmath.Pose{"a": basePose, "b": shoulderOffset, "c": elbowOffset},
				ResultPose:  handInWorld,
			},
		},
	}
}
