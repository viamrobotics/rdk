// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	rdkutils "go.viam.com/rdk/utils"
)

// Define a default speed to target for the base in the case where one is not provided.
const defaultBaseMps = 0.6

const (
	ptgIndex int = iota
	trajectoryIndexWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgBaseKinematics struct {
	base.Base
	frame referenceframe.Frame
	fs    referenceframe.FrameSystem
}

// wrapWithPTGKinematics takes a Base component and adds a PTG kinematic model so that it can be controlled.
func wrapWithPTGKinematics(
	ctx context.Context,
	b base.Base,
) (KinematicBase, error) {
	properties, err := b.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	if properties.TurningRadiusMeters == 0 {
		return nil, errors.New("can only wrap with PTG kinematics if Base property TurningRadiusMeters is nonzero")
	}

	baseMetersPerSecond := defaultBaseMps // Currently no way to get this out of properties

	geometries, err := b.Geometries(ctx)
	if err != nil {
		return nil, err
	}

	frame, err := referenceframe.NewPTGFrameFromTurningRadius(
		b.Name().ShortName(),
		baseMetersPerSecond,
		properties.TurningRadiusMeters,
		0,
		geometries,
	)
	if err != nil {
		return nil, err
	}

	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(frame, fs.World()); err != nil {
		return nil, err
	}

	return &ptgBaseKinematics{
		Base:  b,
		frame: frame,
		fs:    fs,
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
	ptgProv, ok := ptgk.frame.(tpspace.PTGProvider)
	if !ok {
		return errors.New("unable to cast ptgk frame to a PTG Provider")
	}
	ptgs := ptgProv.PTGs()
	selectedPTG := ptgs[int(math.Round(inputs[ptgIndex].Value))]
	selectedTraj := selectedPTG.Trajectory(uint(math.Round(inputs[trajectoryIndexWithinPTG].Value)))

	lastTime := 0.
	for _, trajNode := range selectedTraj {
		if trajNode.Dist > inputs[distanceAlongTrajectoryIndex].Value {
			break
		}
		timestep := time.Duration(1e6*(trajNode.Time-lastTime)) * time.Microsecond
		lastTime = trajNode.Time
		linVel := r3.Vector{0, trajNode.Linvel * 1, 0}
		angVel := r3.Vector{0, 0, rdkutils.RadToDeg(trajNode.Angvel)}
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
