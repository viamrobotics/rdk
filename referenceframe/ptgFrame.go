package referenceframe

import (
	"fmt"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultSimDistMM      = 1100.
	defaultAlphaCnt  uint = 121
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
	maxRadps := velocityMMps / (1000. * turnRadMeters)
	pf := &ptgGridSimFrame{name: name}
	err := pf.initPTGs(velocityMMps, maxRadps, simDist)
	if err != nil {
		return nil, err
	}

	pf.geometries = geoms

	// This is meaningless but needs to be len > 0
	pf.limits = []Limit{
		{Min: 0, Max: float64(len(pf.ptgs) - 1)},
		{Min: 0, Max: float64(defaultAlphaCnt)},
		{Min: 0},
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

func (pf *ptgGridSimFrame) Transform(inputs []Input) (spatialmath.Pose, error) {
	ptgIdx := int(math.Round(inputs[0].Value))
	trajIdx := uint(math.Round(inputs[1].Value))
	traj := pf.ptgs[ptgIdx].Trajectory(trajIdx)
	lastPose := spatialmath.NewZeroPose()
	for _, trajNode := range traj {
		// Walk the trajectory until we pass the specified distance
		if trajNode.Dist > inputs[2].Value {
			lastPose = trajNode.Pose
		} else {
			break
		}
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
	return NewGeometriesInFrame(pf.name, pf.geometries), nil
}

// TODO: make this work.
func (pf *ptgGridSimFrame) AlmostEquals(otherFrame Frame) bool {
	return false
}

func (pf *ptgGridSimFrame) PTGs() []tpspace.PTG {
	return pf.ptgs
}

func (pf *ptgGridSimFrame) initPTGs(maxMps, maxRadps, simDist float64) error {
	ptgs := []tpspace.PTG{}
	for _, ptg := range defaultPTGs {
		// Forwards version of grid sim
		ptgGen := ptg(maxMps, maxRadps, 1.)
		newptg, err := tpspace.NewPTGGridSim(ptgGen, defaultAlphaCnt, simDist)
		if err != nil {
			return err
		}
		ptgs = append(ptgs, newptg)
		ptgGen = ptg(maxMps, maxRadps, -1.)
		if ptgGen != nil {
			// irreversible trajectories, e.g. alpha, will return nil
			newptg, err = tpspace.NewPTGGridSim(ptgGen, defaultAlphaCnt, simDist)
			if err != nil {
				return err
			}
			ptgs = append(ptgs, newptg)
		}
	}
	pf.ptgs = ptgs
	return nil
}
