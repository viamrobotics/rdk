package tpspace

import (
	"testing"
	"fmt"

	"go.viam.com/test"
)


type ptgFactory func(float64, float64) PrecomputePTG

var defaultPTGs = []ptgFactory{
	NewCirclePTG,
	NewCCPTG,
	NewCCSPTG,
	NewCSPTG,
}

var (
	defaultMMps    = 800.
	turnRadMeters = 1.
)

func TestSim(t *testing.T) {
	simDist := 6000.
	alphaCnt := uint(121)
	fmt.Println("type,X,Y")
	//~ for _, ptg := range defaultPTGs {
	ptg := NewCCSPTG
		radPS := defaultMMps / (turnRadMeters * 1000)

		ptgGen := ptg(defaultMMps, radPS)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		grid, err := NewPTGGridSim(ptgGen, alphaCnt, simDist)
		test.That(t, err, test.ShouldBeNil)
		
		for i := uint(0); i < alphaCnt; i++ {
		//~ i := uint(60)
			traj, _ := grid.Trajectory(index2alpha(i, alphaCnt), simDist)
			for _, intPose := range traj{
				fmt.Printf("FINALPATH,%f,%f\n", intPose.Pose.Point().X, intPose.Pose.Point().Y)
			}
		}
}
