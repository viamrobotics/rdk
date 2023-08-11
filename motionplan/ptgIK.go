package motionplan


import (
	"context"
	"math"
	"math/rand"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultDiffT = 0.01 // seconds
	nloptSeed = 42
)

// ptgGridSim will take a PrecomputePTG, and simulate out a number of trajectories through some requested time/distance for speed of lookup
// later. It will store the trajectories in a grid data structure allowing relatively fast lookups.
type ptgIK struct {
	refDist  float64
	simPTG   tpspace.PrecomputePTG
	ptgFrame referenceframe.Frame
	fastGradDescent *NloptIK
	randseed *rand.Rand
	
	gridSim tpspace.PTG
}

// NewPTGIK creates a new PTG by simulating a PrecomputePTG for some distance, then cacheing the results in a grid for fast lookup.
func NewPTGIK(simPTG tpspace.PrecomputePTG, logger golog.Logger, refDist float64, randSeed int) (tpspace.PTG, error) {
	ptgFrame, err := tpspace.NewPTGIKFrame(simPTG, refDist)
	if err != nil {
		return nil, err
	}
	
	//nolint: gosec
	rseed := rand.New(rand.NewSource(int64(randSeed)))
	
	nlopt, err := CreateNloptIKSolver(ptgFrame, logger, 1, defaultEpsilon*defaultEpsilon)
	if err != nil {
		return nil, err
	}
	
	// create an ends-only grid sim for quick end-of-trajectory calculations
	gridSim, err := tpspace.NewPTGGridSim(simPTG, 0, refDist, true)
	if err != nil {
		return nil, err
	}

	ptg := &ptgIK{
		refDist:   refDist,
		simPTG: simPTG,
		ptgFrame:   ptgFrame,
		fastGradDescent: nlopt,
		randseed: rseed,
		gridSim: gridSim,
	}

	return ptg, nil
}

func (ptg *ptgIK) CToTP(ctx context.Context, pose spatialmath.Pose) (*tpspace.TrajNode, error) {
	
	
	
	solutionGen := make(chan []referenceframe.Input, 1)
	seedInput := []referenceframe.Input{{math.Pi/3}, {ptg.refDist/3}} // random value to seed the IK solver
	goalMetric := NewSquaredNormMetric(pose)
	// Spawn the IK solver to generate a solution
	err := ptg.fastGradDescent.Solve(ctx, solutionGen, seedInput, goalMetric, nloptSeed)
	// We should have zero or one solutions
	var solved []referenceframe.Input
	select {
	case solved = <-solutionGen:
	default:
	}
	close(solutionGen)
	if err != nil || solved == nil {
		return nil, nil
		return ptg.gridSim.CToTP(ctx, pose)
	}
	// TODO: make this more efficient
	traj, err := ptg.Trajectory(solved[0].Value, solved[1].Value)
	if err != nil {
		return nil, err
	}
	return traj[len(traj) - 1], nil
}

func (ptg *ptgIK) RefDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgIK) Trajectory(alpha, dist float64) ([]*tpspace.TrajNode, error) {
	return tpspace.ComputePTG(alpha, ptg.simPTG, dist, defaultDiffT)
}
