//go:build !no_cgo

package tpspace

import (
	"errors"
	"fmt"
	"math"

	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	ptgIndex int = iota
	trajectoryAlphaWithinPTG
	distanceAlongTrajectoryIndex
)

// If refDist is not explicitly set, default to pi radians times this adjustment value.
const (
	refDistHalfCircles = 0.9
	defaultTrajCount   = 3
)

type ptgFactory func(float64, float64) PTG

var defaultPTGs = []ptgFactory{
	NewCirclePTG,
	NewCCPTG,
	NewCCSPTG,
	NewCSPTG,
	NewSideSPTG,
	NewSideSOverturnPTG,
}

var defaultDiffPTG ptgFactory = NewDiffDrivePTG

type ptgGroupFrame struct {
	name               string
	limits             []referenceframe.Limit
	geometries         []spatialmath.Geometry
	solvers            []PTGSolver
	velocityMMps       float64
	angVelocityRadps   float64
	turnRadMillimeters float64
	trajCount          int
	logger             logging.Logger
}

// NewPTGFrameFromKinematicOptions will create a new Frame which is also a PTGProvider. It will precompute the default set of
// trajectories out to a given distance, or a default distance if the given distance is <= 0.
func NewPTGFrameFromKinematicOptions(
	name string,
	logger logging.Logger,
	velocityMMps, angVelocityDegps, turnRadMeters, refDist float64,
	trajCount int,
	geoms []spatialmath.Geometry,
	diffDriveOnly bool,
) (referenceframe.Frame, error) {
	if velocityMMps <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, movement velocity %f must be >0", velocityMMps)
	}
	if turnRadMeters < 0 {
		return nil, fmt.Errorf("cannot create ptg frame, turning radius %f must be >0", turnRadMeters)
	}
	if refDist < 0 {
		return nil, fmt.Errorf("cannot create ptg frame, refDist %f must be >=0", refDist)
	}
	if diffDriveOnly && turnRadMeters != 0 {
		return nil, errors.New("if diffDriveOnly is used, turning radius must be zero")
	}

	if trajCount <= 0 {
		trajCount = defaultTrajCount
	}

	turnRadMillimeters := turnRadMeters * 1000

	angVelocityRadps := rdkutils.DegToRad(angVelocityDegps)
	if angVelocityRadps == 0 {
		if turnRadMeters == 0 {
			return nil, errors.New("cannot create ptg frame, turning radius and angular velocity cannot both be zero")
		}
		angVelocityRadps = velocityMMps / turnRadMillimeters
	} else if turnRadMeters > 0 {
		// Compute smallest allowable turning radius permitted by the given speeds. Use the greater of the two.
		calcTurnRadius := (velocityMMps / angVelocityRadps)
		if calcTurnRadius > turnRadMillimeters {
			// This is a debug message because the user will never notice the difference; the trajectories executed by the base will be a
			// subset of the ones that would have been had this conditional not been hit.
			logger.Debugf(
				"given turning radius was %f but a linear velocity of %f "+
					"meters per sec and angular velocity of %f degs per sec only allow a turning radius of %f, using that instead",
				turnRadMeters, velocityMMps/1000., angVelocityDegps, calcTurnRadius,
			)
		} else if calcTurnRadius < turnRadMillimeters {
			// If max allowed angular velocity would turn tighter than given turn radius, shrink the max used angular velocity
			// to match the requested tightest turn radius.
			angVelocityRadps = velocityMMps / turnRadMillimeters
			// This is a warning message because the user will observe the base turning at a different speed than the one requested.
			logger.Warnf(
				"given turning radius was %f but a linear velocity of %f "+
					"meters per sec and angular velocity of %f degs per sec would turn at a radius of %f. Decreasing angular velocity to %f.",
				turnRadMeters, velocityMMps/1000., angVelocityDegps, calcTurnRadius, rdkutils.RadToDeg(angVelocityRadps),
			)
		}
	}

	if refDist == 0 {
		// Default to a distance of just over one half of a circle turning at max radius
		refDist = turnRadMillimeters * math.Pi * refDistHalfCircles
		logger.Debugf("refDist was zero, calculating default %f", refDist)
	}

	ptgsToUse := []ptgFactory{}
	if turnRadMeters == 0 {
		ptgsToUse = append(ptgsToUse, defaultDiffPTG)
	}
	if !diffDriveOnly {
		ptgsToUse = append(ptgsToUse, defaultPTGs...)
	}

	pf := &ptgGroupFrame{name: name}

	ptgs := initializePTGs(velocityMMps, angVelocityRadps, ptgsToUse)
	solvers, err := initializeSolvers(logger, refDist, trajCount, ptgs)
	if err != nil {
		return nil, err
	}

	pf.solvers = solvers

	pf.geometries = geoms
	pf.velocityMMps = velocityMMps
	pf.angVelocityRadps = angVelocityRadps
	pf.turnRadMillimeters = turnRadMillimeters
	pf.trajCount = trajCount

	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(pf.solvers) - 1)},
		{Min: -math.Pi, Max: math.Pi},
		{Min: 0, Max: refDist},
	}

	return pf, nil
}

// NewPTGFrameFromPTGFrame will create a new Frame from a preexisting ptgGroupFrame, allowing the adjustment of `refDist` while keeping
// other params the same. This may be expanded to allow altering turning radius, geometries, etc.
func NewPTGFrameFromPTGFrame(frame referenceframe.Frame, refDist float64, trajCount int) (referenceframe.Frame, error) {
	ptgFrame, ok := frame.(*ptgGroupFrame)
	if !ok {
		return nil, errors.New("cannot create ptg framem given frame is not a ptgGroupFrame")
	}
	if refDist < 0 {
		return nil, fmt.Errorf("cannot create ptg frame, refDist %f must be >=0", refDist)
	}

	if refDist <= 0 {
		refDist = ptgFrame.turnRadMillimeters * math.Pi * refDistHalfCircles
		ptgFrame.logger.Debugf("refDist was zero, calculating default %f", refDist)
	}

	if trajCount <= 0 {
		trajCount = ptgFrame.trajCount
	}

	// Get max angular velocity in radians per second
	pf := &ptgGroupFrame{name: ptgFrame.name}
	ptgs := []PTG{}
	// Go doesn't let us do this all at once
	for _, solver := range ptgFrame.solvers {
		ptgs = append(ptgs, solver)
	}
	solvers, err := initializeSolvers(ptgFrame.logger, refDist, trajCount, ptgs)
	if err != nil {
		return nil, err
	}

	pf.solvers = solvers
	pf.geometries = ptgFrame.geometries
	pf.angVelocityRadps = ptgFrame.angVelocityRadps
	pf.turnRadMillimeters = ptgFrame.turnRadMillimeters
	pf.velocityMMps = ptgFrame.velocityMMps
	pf.trajCount = trajCount

	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(pf.solvers) - 1)},
		{Min: -math.Pi, Max: math.Pi},
		{Min: 0, Max: refDist},
	}

	return pf, nil
}

func (pf *ptgGroupFrame) DoF() []referenceframe.Limit {
	return pf.limits
}

func (pf *ptgGroupFrame) Name() string {
	return pf.name
}

// TODO: Define some sort of config struct for a PTG frame.
func (pf *ptgGroupFrame) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// Inputs are: [0] index of PTG to use, [1] index of the trajectory within that PTG, and [2] distance to travel along that trajectory.
func (pf *ptgGroupFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != len(pf.DoF()) {
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), len(pf.DoF()))
	}

	ptgIdx := int(math.Round(inputs[ptgIndex].Value))
	alpha := inputs[trajectoryAlphaWithinPTG].Value
	dist := inputs[distanceAlongTrajectoryIndex].Value

	traj, err := pf.solvers[ptgIdx].Trajectory(alpha, dist)
	if err != nil {
		return nil, err
	}

	return traj[len(traj)-1].Pose, nil
}

func (pf *ptgGroupFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	n := make([]referenceframe.Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = referenceframe.Input{d}
	}
	return n
}

func (pf *ptgGroupFrame) ProtobufFromInput(input []referenceframe.Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

func (pf *ptgGroupFrame) Geometries(inputs []referenceframe.Input) (*referenceframe.GeometriesInFrame, error) {
	if len(pf.geometries) == 0 {
		return referenceframe.NewGeometriesInFrame(pf.Name(), nil), nil
	}

	transformedPose, err := pf.Transform(inputs)
	if err != nil {
		return nil, err
	}
	geoms := make([]spatialmath.Geometry, 0, len(pf.geometries))
	for _, geom := range pf.geometries {
		geoms = append(geoms, geom.Transform(transformedPose))
	}
	return referenceframe.NewGeometriesInFrame(pf.name, geoms), nil
}

func (pf *ptgGroupFrame) PTGSolvers() []PTGSolver {
	return pf.solvers
}

func initializePTGs(maxMps, maxRPS float64, constructors []ptgFactory) []PTG {
	ptgs := []PTG{}
	for _, ptg := range constructors {
		ptgs = append(ptgs, ptg(maxMps, maxRPS))
	}
	return ptgs
}

type solverAndError struct {
	idx    int
	solver PTGSolver
	err    error
}

func initializeSolvers(logger logging.Logger, simDist float64, trajCount int, ptgs []PTG) ([]PTGSolver, error) {
	solvers := make([]PTGSolver, len(ptgs))
	solverChan := make(chan *solverAndError, len(ptgs))
	for i := range ptgs {
		j := i
		utils.PanicCapturingGo(func() {
			solver, err := NewPTGIK(ptgs[j], logger, simDist, j, trajCount)
			solverChan <- &solverAndError{j, solver, err}
		})
	}
	var allErr error
	for range ptgs {
		solverReturn := <-solverChan
		if solverReturn.solver != nil {
			// Consistent ordering, so that if we create a child frame with NewPTGFrameFromPTGFrame, then the same inputs still work
			solvers[solverReturn.idx] = solverReturn.solver
		}
		allErr = multierr.Combine(allErr, solverReturn.err)
	}
	if allErr != nil {
		return nil, allErr
	}
	return solvers, nil
}
