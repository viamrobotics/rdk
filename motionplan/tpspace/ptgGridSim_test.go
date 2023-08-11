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
	defaultMMps    = 300.
	turnRadMeters = 0.3
)

func TestSim(t *testing.T) {
	simDist := 150.
	alphaCnt := uint(121)
	fmt.Println("type,X,Y")
	//~ for _, ptg := range defaultPTGs {
	ptg := NewCSPTG
		radPS := defaultMMps / (turnRadMeters * 1000)

		ptgGen := ptg(defaultMMps, radPS)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		grid, err := NewPTGGridSim(ptgGen, alphaCnt, simDist, false)
		test.That(t, err, test.ShouldBeNil)
		
		//~ for i := uint(0); i < alphaCnt; i++ {
		//~ i := uint(60)
		alpha := -3.115629077940291
			//~ traj, _ := grid.Trajectory(index2alpha(i, alphaCnt), simDist)
			traj, _ := grid.Trajectory(alpha, simDist)
			for _, intPose := range traj{
				fmt.Printf("FINALPATH,%f,%f\n", intPose.Pose.Point().X, intPose.Pose.Point().Y)
			}
		//~ }
}
