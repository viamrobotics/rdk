package tpspace

import (
	"testing"
	"fmt"

	"go.viam.com/test"
	"go.viam.com/rdk/spatialmath"
	"github.com/golang/geo/r3"
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
	simDist :=2500.
	alphaCnt := uint(61)
	fmt.Println("type,X,Y")
	//~ for _, ptg := range defaultPTGs {
	ptg := NewSideSOverturnPTG
		radPS := defaultMMps / (turnRadMeters * 1000)

		ptgGen := ptg(defaultMMps, radPS)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		grid, err := NewPTGGridSim(ptgGen, alphaCnt, simDist, false)
		test.That(t, err, test.ShouldBeNil)
		
		for i := uint(0); i < alphaCnt; i++ {
		//~ i := uint(60)
		//~ alpha := -0.41541721039203877
			traj, _ := grid.Trajectory(index2alpha(i, alphaCnt), simDist)
			//~ traj, _ := grid.Trajectory(alpha, simDist)
			for _, intPose := range traj{
				fmt.Printf("FINALPATH,%f,%f\n", intPose.Pose.Point().X, intPose.Pose.Point().Y)
			}
		}
}

func TestPose(t *testing.T) {
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{50,1000,0})
	trajPose := spatialmath.NewPose(r3.Vector{-100,10,0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})
	startPose := spatialmath.Compose(goalPose, spatialmath.PoseInverse(trajPose))
	// resultPose x:39.999999999999886 y:1100 o_z:1 theta:-89.99999999999999

	//~ startPose := spatialmath.Compose(spatialmath.PoseInverse(trajPose), goalPose)
	// resultPose x:990 y:50.000000000000114 o_z:1 theta:-89.99999999999999

	fmt.Println("resultPose", spatialmath.PoseToProtobuf(startPose))
}
