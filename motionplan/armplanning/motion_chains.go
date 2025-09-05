package armplanning

import (
	"errors"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type motionChains struct {
	inner []*motionChain
}

func motionChainsFromPlanState(fs *referenceframe.FrameSystem, to *PlanState) (*motionChains, error) {
	// create motion chains for each goal
	inner := make([]*motionChain, 0, len(to.poses)+len(to.configuration))

	for frame, pif := range to.poses {
		chain, err := motionChainFromGoal(fs, frame, pif.Parent())
		if err != nil {
			return nil, err
		}
		inner = append(inner, chain)
	}
	for frame := range to.configuration {
		chain, err := motionChainFromGoal(fs, frame, frame)
		if err != nil {
			return nil, err
		}
		inner = append(inner, chain)
	}

	if len(inner) < 1 {
		return nil, errors.New("must have at least one motion chain")
	}

	return &motionChains{
		inner: inner,
	}, nil
}

func (mC *motionChains) geometries(
	fs *referenceframe.FrameSystem,
	frameSystemGeometries map[string]*referenceframe.GeometriesInFrame,
) (movingRobotGeometries, staticRobotGeometries []spatialmath.Geometry) {
	// find all geometries that are not moving but are in the frame system
	for name, geometries := range frameSystemGeometries {
		moving := false
		for _, chain := range mC.inner {
			if chain.movingFS.Frame(name) != nil {
				moving = true
				movingRobotGeometries = append(movingRobotGeometries, geometries.Geometries()...)
				break
			}
		}
		if !moving {
			// Non-motion-chain frames with nonzero DoF can still move out of the way
			if len(fs.Frame(name).DoF()) > 0 {
				movingRobotGeometries = append(movingRobotGeometries, geometries.Geometries()...)
			} else {
				staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
			}
		}
	}
	return movingRobotGeometries, staticRobotGeometries
}

// If a motion chain is worldrooted, then goals are translated to their position in `World` before solving.
// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper.
func (mC *motionChains) translateGoalsToWorldPosition(
	fs *referenceframe.FrameSystem,
	start referenceframe.FrameSystemInputs,
	goal *PlanState,
) (*PlanState, error) {
	alteredGoals := referenceframe.FrameSystemPoses{}
	if goal.poses != nil {
		for _, chain := range mC.inner {
			// chain solve frame may only be in the goal configuration, in which case we skip as the configuration will be passed through
			if goalPif, ok := goal.poses[chain.solveFrameName]; ok {
				if chain.worldRooted {
					tf, err := fs.Transform(start, goalPif, referenceframe.World)
					if err != nil {
						return nil, err
					}
					alteredGoals[chain.solveFrameName] = tf.(*referenceframe.PoseInFrame)
				} else {
					alteredGoals[chain.solveFrameName] = goalPif
				}
			}
		}
		return &PlanState{poses: alteredGoals, configuration: goal.configuration}, nil
	}
	return goal, nil
}

func (mC *motionChains) framesFilteredByMovingAndNonmoving(fs *referenceframe.FrameSystem) (moving, nonmoving []string) {
	movingMap := map[string]referenceframe.Frame{}
	for _, chain := range mC.inner {
		for _, frame := range chain.frames {
			movingMap[frame.Name()] = frame
		}
	}

	// Here we account for anything in the framesystem that is not part of a motion chain
	for _, frameName := range fs.FrameNames() {
		if _, ok := movingMap[frameName]; ok {
			moving = append(moving, frameName)
		} else {
			nonmoving = append(nonmoving, frameName)
		}
	}
	return moving, nonmoving
}

// motionChain structs are meant to be ephemerally created for each individual goal in a motion request, and calculates the shortest
// path between components in the framesystem allowing knowledge of which frames may move.
type motionChain struct {
	// List of names of all frames that could move, used for collision detection
	// As an example a gripper attached to an arm which is moving relative to World, would not be in frames below but in this object
	movingFS       *referenceframe.FrameSystem
	frames         []referenceframe.Frame // all frames directly between and including solveFrame and goalFrame. Order not important.
	solveFrameName string
	goalFrameName  string
	// If this is true, then goals are translated to their position in `World` before solving.
	// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper
	// TODO(pl): explore allowing this to be frames other than world
	worldRooted bool
}

func motionChainFromGoal(fs *referenceframe.FrameSystem, moveFrame, goalFrameName string) (*motionChain, error) {
	// get goal frame
	goalFrame := fs.Frame(goalFrameName)
	if goalFrame == nil {
		return nil, referenceframe.NewFrameMissingError(goalFrameName)
	}
	goalFrameList, err := fs.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}

	// get solve frame
	solveFrame := fs.Frame(moveFrame)
	if solveFrame == nil {
		return nil, referenceframe.NewFrameMissingError(moveFrame)
	}
	solveFrameList, err := fs.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}
	if len(solveFrameList) == 0 {
		return nil, errors.New("solveFrameList was empty")
	}

	movingFS := func(frameList []referenceframe.Frame) (*referenceframe.FrameSystem, error) {
		// Find first moving frame
		var moveF referenceframe.Frame
		for i := len(frameList) - 1; i >= 0; i-- {
			if len(frameList[i].DoF()) != 0 {
				moveF = frameList[i]
				break
			}
		}
		if moveF == nil {
			return referenceframe.NewEmptyFrameSystem(""), nil
		}
		return fs.FrameSystemSubset(moveF)
	}

	// find pivot frame between goal and solve frames
	var moving *referenceframe.FrameSystem
	var frames []referenceframe.Frame
	worldRooted := false
	pivotFrame, err := findPivotFrame(solveFrameList, goalFrameList)
	if err != nil {
		return nil, err
	}
	if pivotFrame.Name() == referenceframe.World {
		frames = uniqInPlaceSlice(append(solveFrameList, goalFrameList...))
		moving, err = movingFS(solveFrameList)
		if err != nil {
			return nil, err
		}
		movingSubset2, err := movingFS(goalFrameList)
		if err != nil {
			return nil, err
		}
		if err = moving.MergeFrameSystem(movingSubset2, moving.World()); err != nil {
			return nil, err
		}
	} else {
		dof := 0
		var solveMovingList []referenceframe.Frame
		var goalMovingList []referenceframe.Frame

		// Get minimal set of frames from solve frame to goal frame
		for _, frame := range solveFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			frames = append(frames, frame)
			solveMovingList = append(solveMovingList, frame)
		}
		for _, frame := range goalFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			frames = append(frames, frame)
			goalMovingList = append(goalMovingList, frame)
		}

		// If shortest path has 0 dof (e.g. a camera attached to a gripper), translate goal to world frame
		if dof == 0 {
			worldRooted = true
			frames = solveFrameList
			moving, err = movingFS(solveFrameList)
			if err != nil {
				return nil, err
			}
		} else {
			// Get all child nodes of pivot node
			moving, err = movingFS(solveMovingList)
			if err != nil {
				return nil, err
			}
			movingSubset2, err := movingFS(goalMovingList)
			if err != nil {
				return nil, err
			}
			if err = moving.MergeFrameSystem(movingSubset2, moving.World()); err != nil {
				return nil, err
			}
		}
	}

	return &motionChain{
		movingFS:       moving,
		frames:         frames,
		solveFrameName: solveFrame.Name(),
		goalFrameName:  goalFrame.Name(),
		worldRooted:    worldRooted,
	}, nil
}
