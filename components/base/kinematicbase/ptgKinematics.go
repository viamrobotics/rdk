// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	rdkutils "go.viam.com/rdk/utils"
)

// Define a default speed to target for the base in the case where one is not provided.
const defaultBaseMMps = 600.

const (
	ptgIndex int = iota
	trajectoryIndexWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgBaseKinematics struct {
	base.Base
	logger golog.Logger
	frame  referenceframe.Frame
	fs     referenceframe.FrameSystem
	ptgs   []tpspace.PTG
}

// wrapWithPTGKinematics takes a Base component and adds a PTG kinematic model so that it can be controlled.
func wrapWithPTGKinematics(
	ctx context.Context,
	b base.Base,
	logger golog.Logger,
	options Options,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	baseMillimetersPerSecond := defaultBaseMMps
	if options.LinearVelocityMMPerSec > 0 {
		baseMillimetersPerSecond = options.LinearVelocityMMPerSec
	}

	baseTurningRadius := properties.TurningRadiusMeters
	if options.AngularVelocityDegsPerSec > 0 {
		// Compute smallest allowable turning radius permitted by the given speeds. Use the greater of the two.
		calcTurnRadius := (baseMillimetersPerSecond / rdkutils.DegToRad(options.AngularVelocityDegsPerSec)) / 1000.
		baseTurningRadius = math.Max(baseTurningRadius, calcTurnRadius)
	}
	logger.Infof(
		"using baseMillimetersPerSecond %f and baseTurningRadius %f for PTG base kinematics",
		baseMillimetersPerSecond,
		baseTurningRadius,
	)

	if baseTurningRadius <= 0 {
		return nil, errors.New("can only wrap with PTG kinematics if turning radius is greater than zero")
	}

	geometries, err := b.Geometries(ctx, nil)
	if err != nil {
		return nil, err
	}

	frame, err := motionplan.NewPTGFrameFromTurningRadius(
		b.Name().ShortName(),
		logger,
		baseMillimetersPerSecond,
		baseTurningRadius,
		0, // pass 0 to use the default refDist
		geometries,
	)
	if err != nil {
		return nil, err
	}

	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(frame, fs.World()); err != nil {
		return nil, err
	}

	ptgProv, ok := frame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGs()

	return &ptgBaseKinematics{
		Base:   b,
		logger: logger,
		frame:  frame,
		fs:     fs,
		ptgs:   ptgs,
	}, nil
}

func (ptgk *ptgBaseKinematics) Kinematics() referenceframe.Frame {
	return ptgk.frame
}

func (ptgk *ptgBaseKinematics) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// A PTG frame is always at its own origin, so current inputs are always all zero/not meaningful
	return []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}, nil
}

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputs []referenceframe.Input) (err error) {
	if len(inputs) != 3 {
		return errors.New("inputs to ptg kinematic base must be length 3")
	}

	ptgk.logger.Debugf("GoToInputs going to %v", inputs)

	selectedPTG := ptgk.ptgs[int(math.Round(inputs[ptgIndex].Value))]
	selectedTraj, err := selectedPTG.Trajectory(inputs[trajectoryIndexWithinPTG].Value, inputs[distanceAlongTrajectoryIndex].Value)
	if err != nil {
		return multierr.Combine(err, ptgk.Base.Stop(ctx, nil))
	}

	lastTime := 0.
	for _, trajNode := range selectedTraj {
		// TODO: Most trajectories update their velocities infrequently, or sometimes never.
		// This function could be improved by looking ahead through the trajectory and minimizing the amount of SetVelocity calls.
		timestep := time.Duration((trajNode.Time-lastTime)*1000*1000) * time.Microsecond
		lastTime = trajNode.Time
		linVel := r3.Vector{0, trajNode.LinVelMMPS, 0}
		angVel := r3.Vector{0, 0, rdkutils.RadToDeg(trajNode.AngVelRPS)}

		ptgk.logger.Debugf(
			"setting velocity to linear %v angular %v and running velocity step for %s",
			linVel,
			angVel,
			timestep,
		)

		err := ptgk.Base.SetVelocity(
			ctx,
			linVel,
			angVel,
			nil,
		)
		if err != nil {
			return multierr.Combine(err, ptgk.Base.Stop(ctx, nil))
		}
		utils.SelectContextOrWait(ctx, timestep)
	}

	return ptgk.Base.Stop(ctx, nil)
}
