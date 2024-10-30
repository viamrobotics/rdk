//go:build !no_cgo

package tpspace

import (
	"errors"
	"fmt"
	"math"
	"strconv"

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
	startDistanceAlongTrajectoryIndex
	endDistanceAlongTrajectoryIndex
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
}

var defaultCorrectionPtg = NewCirclePTG

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
	correctionIdx      int
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

	pf := &ptgGroupFrame{name: name}

	longPtgsToUse := []ptgFactory{}
	shortPtgsToUse := []ptgFactory{}
	if canRotateInPlace {
		longPtgsToUse = append(longPtgsToUse, defaultDiffPTG)
	}
	if !diffDriveOnly {
		longPtgsToUse = append(longPtgsToUse, defaultPTGs...)
		shortPtgsToUse = append(shortPtgsToUse, defaultShortPtgs...)
		// Use Circle PTG for course correction. Ensure it is last.
		shortPtgsToUse = append(shortPtgsToUse, defaultCorrectionPtg)
		pf.correctionIdx = len(longPtgsToUse) + (len(shortPtgsToUse) - 1)
	} else {
		// Use diff drive PTG for course correction
		pf.correctionIdx = 0
	}

	longPtgs := initializePTGs(turnRadMillimeters, longPtgsToUse)
	allSolvers, err := initializeSolvers(logger, refDistLong, refDistShort, trajCount, longPtgs)
	if err != nil {
		return nil, err
	}
	shortPtgs := initializePTGs(turnRadMillimeters, shortPtgsToUse)
	shortSolvers, err := initializeSolvers(logger, refDistShort, refDistShort, trajCount, shortPtgs)
	if err != nil {
		return nil, err
	}
	allSolvers = append(allSolvers, shortSolvers...)

	pf.solvers = allSolvers
	pf.geometries = geoms
	pf.turnRadMillimeters = turnRadMillimeters
	pf.trajCount = trajCount
	pf.logger = logger

	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(pf.solvers) - 1)},
		{Min: -math.Pi, Max: math.Pi},
		{Min: 0, Max: refDistLong},
		{Min: 0, Max: refDistLong},
	}

	return pf, nil
}

func (pf *ptgGroupFrame) CorrectionSolverIdx() int {
	return pf.correctionIdx
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

// Inputs are: [0] index of PTG to use, [1] index of the trajectory within that PTG, [2] starting point on the trajectory, and [3] distance
// to travel along that trajectory.
func (pf *ptgGroupFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if err := pf.validInputs(inputs); err != nil {
		return nil, err
	}

	ptgIdx := int(math.Round(inputs[ptgIndex].Value))

	endPose, err := pf.solvers[ptgIdx].Transform([]referenceframe.Input{
		inputs[trajectoryAlphaWithinPTG],
		inputs[endDistanceAlongTrajectoryIndex],
	})
	if err != nil {
		return nil, err
	}
	if inputs[startDistanceAlongTrajectoryIndex].Value != 0 {
		startPose, err := pf.solvers[ptgIdx].Transform([]referenceframe.Input{
			inputs[trajectoryAlphaWithinPTG],
			inputs[startDistanceAlongTrajectoryIndex],
		})
		if err != nil {
			return nil, err
		}
		if inputs[endDistanceAlongTrajectoryIndex].Value < inputs[startDistanceAlongTrajectoryIndex].Value {
			endPose = spatialmath.PoseBetween(spatialmath.Compose(endPose, flipPose), flipPose)
			startPose = spatialmath.PoseBetween(spatialmath.Compose(startPose, flipPose), flipPose)
			endPose = spatialmath.PoseBetweenInverse(endPose, startPose)
		} else {
			endPose = spatialmath.PoseBetween(startPose, endPose)
		}
	}

	return endPose, nil
}

// Interpolate on a PTG group frame follows the following framework:
// Let us say we are executing a set on inputs, for example [1, pi/2, 20, 2000].
// The starting configuration would be [1, pi/2, 20, 20], and Transform() would yield a zero pose. Interpolating from this to the final set
// of inputs above would yield things like [1, pi/2, 20, 50] and so on, the Transform() of each giving the amount moved from the start.
// Some current intermediate input may be [1, pi/2, 20, 150]. If we were to try to interpolate the remainder of the arc, then that would be
// the `from`, while the `to` remains the same. Thus a point along the interpolated path might be [1, pi/2, 150, 170], which would yield
// the Transform from the current position that would be expected during the next 20 distance to be executed.
// If we are interpolating against a hypothetical arc, there is no true "from", so `nil` should be passed instead if coming from somewhere
// which does not have knowledge of specific inputs.
// The above is inverted with negative distances, requiring some complicated logic which should be radically simplified by RSDK-7515.
func (pf *ptgGroupFrame) Interpolate(from, to []referenceframe.Input, by float64) ([]referenceframe.Input, error) {
	if err := pf.validInputs(from); err != nil {
		return nil, err
	}
	if err := pf.validInputs(to); err != nil {
		return nil, err
	}

	// There are two different valid interpretations of `from`. Either it can be an all-zero input, in which case we interpolate across `to`
	// or it can match `to` in every value except the end distance index, as described above.
	zeroInputFrom := true

	nonMatchIndex := endDistanceAlongTrajectoryIndex
	for i, input := range from {
		if input.Value != 0 {
			zeroInputFrom = false
		}

		if !zeroInputFrom {
			if i == nonMatchIndex {
				continue
			}
			if input.Value != to[i].Value {
				return nil, NewNonMatchingInputError(from[i].Value, to[i].Value)
			}
		}
	}

	startVal := from[endDistanceAlongTrajectoryIndex].Value
	if zeroInputFrom {
		startVal = to[startDistanceAlongTrajectoryIndex].Value
	}
	endVal := to[endDistanceAlongTrajectoryIndex].Value

	changeVal := (endVal - startVal) * by
	return []referenceframe.Input{
		to[ptgIndex],
		to[trajectoryAlphaWithinPTG],
		{startVal},
		{startVal + changeVal},
	}, nil
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
	for i, geom := range pf.geometries {
		tfGeom := geom.Transform(transformedPose)
		if tfGeom.Label() == "" {
			tfGeom.SetLabel(pf.name + "_geometry_" + strconv.Itoa(i))
		}
		geoms = append(geoms, tfGeom)
	}
	return referenceframe.NewGeometriesInFrame(pf.name, geoms), nil
}

func (pf *ptgGroupFrame) PTGSolvers() []PTGSolver {
	return pf.solvers
}

// validInputs checks whether the given array of inputs violates any limits.
func (pf *ptgGroupFrame) validInputs(inputs []referenceframe.Input) error {
	var errAll error
	if len(inputs) != len(pf.limits) {
		return referenceframe.NewIncorrectDoFError(len(inputs), len(pf.limits))
	}
	for i := 0; i < len(pf.limits); i++ {
		if inputs[i].Value < pf.limits[i].Min || inputs[i].Value > pf.limits[i].Max {
			lim := []float64{pf.limits[i].Min, pf.limits[i].Max}
			multierr.AppendInto(&errAll, fmt.Errorf("%s %s %s, %s %.5f %s %.5f", "input", fmt.Sprint(i),
				referenceframe.OOBErrString, "input", inputs[i].Value, "needs to be within range", lim))
		}
	}
	return errAll
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
