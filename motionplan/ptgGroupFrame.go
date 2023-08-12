package motionplan

import (
	"fmt"
	"math"

	pb "go.viam.com/api/component/arm/v1"
	"github.com/edaniels/golog"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultSimDistMM      = 600.
)

const (
	ptgIndex int = iota
	trajectoryAlphaWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgFactory func(float64, float64) tpspace.PrecomputePTG

var defaultPTGs = []ptgFactory{
	tpspace.NewCirclePTG,
	tpspace.NewCCPTG,
	tpspace.NewCCSPTG,
	tpspace.NewCSPTG,
	//~ tpspace.NewAlphaPTG,
}

type ptgGroupFrame struct {
	name       string
	limits     []referenceframe.Limit
	geometries []spatialmath.Geometry
	ptgs       []tpspace.PTG
}

// NewPTGFrameFromTurningRadius will create a new Frame which is also a PTGProvider. It will precompute the default set of
// trajectories out to a given distance, or a default distance if the given distance is <= 0.
func NewPTGFrameFromTurningRadius(
	name string,
	logger golog.Logger,
	velocityMMps, turnRadMeters, refDist float64,
	geoms []spatialmath.Geometry,
) (referenceframe.Frame, error) {
	if velocityMMps <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, movement velocity %f must be >0", velocityMMps)
	}
	if turnRadMeters <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, turning radius %f must be >0", turnRadMeters)
	}

	if refDist <= 0 {
		refDist = defaultSimDistMM
	}

	// Get max angular velocity in radians per second
	maxRPS := velocityMMps / (1000. * turnRadMeters)
	pf := &ptgGroupFrame{name: name}
	err := pf.initPTGs(logger, velocityMMps, maxRPS, refDist, false)
	//~ err := pf.initPTGs(logger, velocityMMps, maxRPS, refDist, true)
	if err != nil {
		return nil, err
	}

	pf.geometries = geoms

	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(pf.ptgs) - 1)},
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
	alpha := inputs[trajectoryAlphaWithinPTG].Value
	dist := inputs[distanceAlongTrajectoryIndex].Value
	
	ptgIdx := int(math.Round(inputs[ptgIndex].Value))
	
	traj, err := pf.ptgs[ptgIdx].Trajectory(alpha, dist)
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

func (pf *ptgGroupFrame) PTGs() []tpspace.PTG {
	return pf.ptgs
}

func (pf *ptgGroupFrame) initPTGs(logger golog.Logger, maxMps, maxRPS, simDist float64, simulate bool) error {
	ptgs := []tpspace.PTG{}
	for _, ptg := range defaultPTGs {
		// Positive K calculates trajectories forwards, negative k calculates trajectories backwards
		ptgGen := ptg(maxMps, maxRPS)
		if ptgGen != nil {
			if simulate {
				//~ for _, k := range []float64{1., -1.} {
				for _, k := range []float64{1.} {
					// irreversible trajectories, e.g. alpha, will return nil for negative k
					newptg, err := tpspace.NewPTGGridSim(ptgGen, 0, k*simDist, false) // 0 uses default alpha count
					if err != nil {
						return err
					}
					ptgs = append(ptgs, newptg)
				}
			} else {
				newptg, err := NewPTGIK(ptgGen, logger, simDist, 2)
				if err != nil {
					return err
				}
				ptgs = append(ptgs, newptg)
			}
		}
	}
	pf.ptgs = ptgs
	return nil
}
