package tpspace

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"

)

func TestAlphaIdx(t *testing.T) {
	for i := uint(0); i < defaultAlphaCnt; i++ {
		alpha := index2alpha(i, defaultAlphaCnt)
		i2 := alpha2index(alpha, defaultAlphaCnt)
		test.That(t, i, test.ShouldEqual, i2)
	}
}

func alpha2index(alpha float64, numPaths uint) uint {
	alpha = wrapTo2Pi(alpha+math.Pi) - math.Pi
	idx := uint(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0)))
	return idx
}

func TestDiffDrivePTG(t *testing.T) {
	dd := NewDiffDrivePTG(300, 2.35)
	ddArc, err := ComputePTG(dd, -1.65, 2, 0.01)
	test.That(t, err, test.ShouldBeNil)
	for _, tNode := range ddArc {
		fmt.Println(spatialmath.PoseToProtobuf(tNode.Pose))
		fmt.Println(tNode)
	}
}

func verifyTrajectory(t *testing.T, traj []*TrajNode) {
	
	trajPose := spatialmath.NewZeroPose()
	elapsed := 0.
	for i := 1; i < len(traj); i++ {
		fmt.Println("")
		tNode := traj[i]
		fmt.Println("tnode", tNode)
		timeStep := tNode.Time - elapsed
		linStep := spatialmath.NewPoseFromPoint(r3.Vector{0, tNode.LinVelMMPS * timeStep, 0})
		trajPose = spatialmath.Compose(trajPose, linStep)
		
		angStep := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVector{OZ: 1, Theta: tNode.AngVelRPS * timeStep})
		trajPose = spatialmath.Compose(trajPose, angStep)
		
		fmt.Println("node pose", spatialmath.PoseToProtobuf(tNode.Pose))
		fmt.Println("traj pose", spatialmath.PoseToProtobuf(trajPose))
		elapsed = tNode.Time
	}
}

func TestPTGVelocities(t *testing.T) {
	dd := NewDiffDrivePTG(300, 2.35)
	//~ ddArc, err := ComputePTG(dd, -1.65, 2, 0.01)
	ddArc, err := ComputePTG(dd, 0, 10, 0.01)
	test.That(t, err, test.ShouldBeNil)
	verifyTrajectory(t, ddArc)
}
