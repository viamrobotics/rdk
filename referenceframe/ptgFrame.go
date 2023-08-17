package referenceframe

import (
	"errors"
	"fmt"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultSimDistMM      = 2000.
	defaultAlphaCnt  uint = 121
)

const (
	ptgIndex int = iota
	trajectoryIndexWithinPTG
	distanceAlongTrajectoryIndex
)

type ptgFactory func(float64, float64, float64) tpspace.PrecomputePTG

var defaultPTGs = []ptgFactory{
	tpspace.NewCirclePTG,
	tpspace.NewCCPTG,
	tpspace.NewCCSPTG,
	tpspace.NewCSPTG,
	tpspace.NewAlphaPTG,
}

type ptgGridSimFrame struct {
	name       string
	limits     []Limit
	geometries []spatialmath.Geometry
	ptgs       []tpspace.PTG
}

// NewPTGFrameFromTurningRadius will create a new Frame which is also a tpspace.PTGProvider. It will precompute the default set of
// trajectories out to a given distance, or a default distance if the given distance is <= 0.
func NewPTGFrameFromTurningRadius(name string, velocityMMps, turnRadMeters, simDist float64, geoms []spatialmath.Geometry) (Frame, error) {
	if velocityMMps <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, movement velocity %f must be >0", velocityMMps)
	}
	if turnRadMeters <= 0 {
		return nil, fmt.Errorf("cannot create ptg frame, turning radius %f must be >0", turnRadMeters)
	}

	if simDist <= 0 {
		simDist = defaultSimDistMM
	}

	// Get max angular velocity in radians per second
	maxRPS := velocityMMps / (1000. * turnRadMeters)
	pf := &ptgGridSimFrame{name: name}
	err := pf.initPTGs(velocityMMps, maxRPS, simDist)
	if err != nil {
		return nil, err
	}

	pf.geometries = geoms

	pf.limits = []Limit{
		{Min: 0, Max: float64(len(pf.ptgs) - 1)},
		{Min: 0, Max: float64(defaultAlphaCnt)},
		{Min: 0, Max: simDist},
	}

	return pf, nil
}

func (pf *ptgGridSimFrame) DoF() []Limit {
	return pf.limits
}

func (pf *ptgGridSimFrame) Name() string {
	return pf.name
}

// TODO: Define some sort of config struct for a PTG frame.
func (pf *ptgGridSimFrame) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// Inputs are: [0] index of PTG to use, [1] index of the trajectory within that PTG, and [2] distance to travel along that trajectory.
func (pf *ptgGridSimFrame) Transform(inputs []Input) (spatialmath.Pose, error) {
	ptgIdx := int(math.Round(inputs[ptgIndex].Value))
	trajIdx := uint(math.Round(inputs[trajectoryIndexWithinPTG].Value))
	traj := pf.ptgs[ptgIdx].Trajectory(trajIdx)
	lastPose := spatialmath.NewZeroPose()
	for _, trajNode := range traj {
		// Walk the trajectory until we pass the specified distance
		if trajNode.Dist > inputs[distanceAlongTrajectoryIndex].Value {
			break
		}
		lastPose = trajNode.Pose
	}

	return lastPose, nil
}

func (pf *ptgGridSimFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = Input{d}
	}
	return n
}

func (pf *ptgGridSimFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

func (pf *ptgGridSimFrame) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	if len(pf.geometries) == 0 {
		return NewGeometriesInFrame(pf.Name(), nil), nil
	}

	if inputs == nil {
		return nil, errors.New("please specify non-nil inputs value")
	}
	transformedPose, err := pf.Transform(inputs)
	if err != nil {
		return nil, err
	}
	geoms := make([]spatialmath.Geometry, 0, len(pf.geometries))
	for _, geom := range pf.geometries {
		geoms = append(geoms, geom.Transform(transformedPose))
	}
	return NewGeometriesInFrame(pf.name, geoms), nil
}

func (pf *ptgGridSimFrame) PTGs() []tpspace.PTG {
	return pf.ptgs
}

func (pf *ptgGridSimFrame) initPTGs(maxMps, maxRPS, simDist float64) error {
	ptgs := []tpspace.PTG{}
	for _, ptg := range defaultPTGs {
		for _, k := range []float64{1., -1.} {
			// Positive K calculates trajectories forwards, negative k calculates trajectories backwards
			ptgGen := ptg(maxMps, maxRPS, k)
			if ptgGen != nil {
				// irreversible trajectories, e.g. alpha, will return nil for negative k
				newptg, err := tpspace.NewPTGGridSim(ptgGen, defaultAlphaCnt, simDist)
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
