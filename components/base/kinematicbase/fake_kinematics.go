package kinematicbase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type fakeKinematics struct {
	*fake.Base
	motion.Localizer
	planningFrame, executionFrame referenceframe.Frame
	inputs                        []referenceframe.Input
	options                       Options
	lock                          sync.Mutex
}

// WrapWithFakeKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func WrapWithFakeKinematics(
	ctx context.Context,
	b *fake.Base,
	localizer motion.Localizer,
	limits []referenceframe.Limit,
	options Options,
) (KinematicBase, error) {
	position, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	pt := position.Pose().Point()
	fk := &fakeKinematics{
		Base:      b,
		Localizer: localizer,
		inputs:    []referenceframe.Input{{pt.X}, {pt.Y}},
	}
	var geometry spatialmath.Geometry
	if len(fk.Base.Geometry) != 0 {
		geometry = fk.Base.Geometry[0]
	}

	fk.executionFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits, geometry)
	if err != nil {
		return nil, err
	}

	if options.PositionOnlyMode {
		fk.planningFrame, err = referenceframe.New2DMobileModelFrame(b.Name().ShortName(), limits[:2], geometry)
		if err != nil {
			return nil, err
		}
	} else {
		fk.planningFrame = fk.executionFrame
	}

	fk.options = options
	return fk, nil
}

func (fk *fakeKinematics) Kinematics() referenceframe.Frame {
	return fk.planningFrame
}

func (fk *fakeKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	fk.lock.Lock()
	defer fk.lock.Unlock()
	return fk.inputs, nil
}

func (fk *fakeKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := fk.planningFrame.Transform(inputs)
	if err != nil {
		return err
	}
	fk.lock.Lock()
	fk.inputs = inputs
	fk.lock.Unlock()

	// Sleep for a short amount to time to simulate a base taking some amount of time to reach the inputs
	time.Sleep(150 * time.Millisecond)
	return nil
}

//nolint: dupl
func (fk *fakeKinematics) ErrorState(
	ctx context.Context,
	plan [][]referenceframe.Input,
	currentNode int,
) (spatialmath.Pose, error) {
	if currentNode < 0 || currentNode >= len(plan) {
		return nil, fmt.Errorf("cannot get errorState for node %d, must be >= 0 and less than plan length %d", currentNode, len(plan))
	}

	// Get pose-in-frame of the base via its localizer. The offset between the localizer and its base should already be accounted for.
	actualPIF, err := fk.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	var nominalPose spatialmath.Pose

	// Determine the nominal pose, that is, the pose where the robot ought be if it had followed the plan perfectly up until this point.
	// This is done differently depending on what sort of frame we are working with.
	if len(plan) < 2 {
		return nil, errors.New("diff drive motion plan must have at least two waypoints")
	}
	nominalPose, err = fk.planningFrame.Transform(plan[currentNode])
	if err != nil {
		return nil, err
	}
	if currentNode > 0 {
		pastPose, err := fk.planningFrame.Transform(plan[currentNode-1])
		if err != nil {
			return nil, err
		}
		// diff drive bases don't have a notion of "distance along the trajectory between waypoints", so instead we compare to the
		// nearest point on the straight line path.
		nominalPoint := spatialmath.ClosestPointSegmentPoint(pastPose.Point(), nominalPose.Point(), actualPIF.Pose().Point())
		pointDiff := nominalPose.Point().Sub(pastPose.Point())
		desiredHeading := math.Atan2(pointDiff.Y, pointDiff.X)
		nominalPose = spatialmath.NewPose(nominalPoint, &spatialmath.OrientationVector{OZ: 1, Theta: desiredHeading})
	}

	return spatialmath.PoseBetween(nominalPose, actualPIF.Pose()), nil
}
