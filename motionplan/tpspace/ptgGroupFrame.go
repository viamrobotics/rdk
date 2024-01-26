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
)

const (
	ptgIndex int = iota
	trajectoryAlphaWithinPTG
	distanceAlongTrajectoryIndex
)

// If refDist is not explicitly set, default to pi radians times this adjustment value.
const (
	defaultRefDistLong        = 100000. // 100 meters
	defaultRefDistShortMin    = 500.    // 500 mm
	defaultRefDistHalfCircles = 0.9
	defaultTrajCount          = 2
)

type ptgFactory func(float64) PTG

// These PTGs do not end in a straight line, and thus are restricted to a shorter maximum length.
var defaultShortPtgs = []ptgFactory{
	NewCCPTG,
	NewCCSPTG,
	NewCirclePTG,
}

// These PTGs curve at the beginning and then have a straight line of arbitrary length, which is allowed to extend to defaultRefDistLong.
var defaultPTGs = []ptgFactory{
	NewCSPTG,
	NewSideSOverturnPTG,
}

var defaultDiffPTG ptgFactory = NewDiffDrivePTG

type ptgGroupFrame struct {
	name               string
	limits             []referenceframe.Limit
	geometries         []spatialmath.Geometry
	solvers            []PTGSolver
	turnRadMillimeters float64
	trajCount          int
	logger             logging.Logger
}

// NewPTGFrameFromKinematicOptions will create a new Frame which is also a PTGProvider. It will precompute the default set of
// trajectories out to a given distance, or a default distance if the given distance is <= 0.
func NewPTGFrameFromKinematicOptions(
	name string,
	logger logging.Logger,
	turnRadMeters float64,
	trajCount int,
	geoms []spatialmath.Geometry,
	diffDriveOnly bool,
	canRotateInPlace bool,
) (referenceframe.Frame, error) {
	if turnRadMeters <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, turning radius %f must be >0", turnRadMeters)
	}
	if diffDriveOnly && !canRotateInPlace {
		return nil, errors.New("if diffDriveOnly is used, canRotateInPlace must be true")
	}

	if trajCount <= 0 {
		trajCount = defaultTrajCount
	}

	turnRadMillimeters := turnRadMeters * 1000

	refDistLong := defaultRefDistLong
	refDistShort := math.Max(
		math.Min(turnRadMillimeters*math.Pi*defaultRefDistHalfCircles, refDistLong*0.1),
		defaultRefDistShortMin,
	)

	longPtgsToUse := []ptgFactory{}
	shortPtgsToUse := []ptgFactory{}
	if canRotateInPlace {
		longPtgsToUse = append(longPtgsToUse, defaultDiffPTG)
	}
	if !diffDriveOnly {
		longPtgsToUse = append(longPtgsToUse, defaultPTGs...)
		shortPtgsToUse = append(shortPtgsToUse, defaultShortPtgs...)
	}

	pf := &ptgGroupFrame{name: name}

	longPtgs := initializePTGs(turnRadMillimeters, longPtgsToUse)
	longSolvers, err := initializeSolvers(logger, refDistLong, refDistShort, trajCount, longPtgs)
	if err != nil {
		return nil, err
	}
	shortPtgs := initializePTGs(turnRadMillimeters, shortPtgsToUse)
	shortSolvers, err := initializeSolvers(logger, refDistShort, refDistShort, trajCount, shortPtgs)
	if err != nil {
		return nil, err
	}

	longSolvers = append(longSolvers, shortSolvers...)
	pf.solvers = longSolvers
	pf.geometries = geoms
	pf.turnRadMillimeters = turnRadMillimeters
	pf.trajCount = trajCount
	pf.logger = logger

	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(pf.solvers) - 1)},
		{Min: -math.Pi, Max: math.Pi},
		{Min: -refDistLong, Max: refDistLong},
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

	pose, err := pf.solvers[ptgIdx].Transform([]referenceframe.Input{inputs[trajectoryAlphaWithinPTG], inputs[distanceAlongTrajectoryIndex]})
	if err != nil {
		return nil, err
	}

	return pose, nil
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

func initializePTGs(turnRadius float64, constructors []ptgFactory) []PTG {
	ptgs := []PTG{}
	for _, ptg := range constructors {
		ptgs = append(ptgs, ptg(turnRadius))
	}
	return ptgs
}

type solverAndError struct {
	idx    int
	solver PTGSolver
	err    error
}

func initializeSolvers(logger logging.Logger, simDistFar, simDistRestricted float64, trajCount int, ptgs []PTG) ([]PTGSolver, error) {
	solvers := make([]PTGSolver, len(ptgs))
	solverChan := make(chan *solverAndError, len(ptgs))
	for i := range ptgs {
		j := i
		utils.PanicCapturingGo(func() {
			solver, err := NewPTGIK(ptgs[j], logger, simDistFar, simDistRestricted, j, trajCount)
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
