package motionplan

import (
	"context"
	"fmt"

	"math"
	"math/rand"

	"runtime"
	"testing"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var (
	home = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	nCPU = runtime.NumCPU()
)

// This should test a simple linear motion
func TestSimpleMotion(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik, err := kinematics.CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	mp := &linearMotionPlanner{solver: ik, frame: m}

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  250,
		Y:  0,
		Z:  200,
		OZ: -1,
	}
	solution, err := mp.Plan(context.Background(), pos, home)
	test.That(t, err, test.ShouldBeNil)

	fmt.Println(solution)
}

// This should test a simple linear motion
func TestSample(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	// goal orientation of no rotation
	//~ goalOrient := spatial.NewZeroPose()
	ov := &spatial.OrientationVector{math.Pi / 2, 0, 0, 1}
	ov.Normalize()
	goalOrient := spatial.NewPoseFromOrientationVector(r3.Vector{100, 100, 200}, ov)
	
	match := 0
	nSamp := 1000000
	for i := 0; i < nSamp; i++ {
		inputs := randPos(m, rnd)
		rPos, _ := m.Transform(inputs)
		dist := spatial.PoseDelta(goalOrient, rPos)
		//~ fmt.Println(dist)
		if (dist[3]*dist[3]) + (dist[4]*dist[4]) + (dist[5]*dist[5]) < 0.01 {
			match++
		}
		if i % 10000 == 0 {
			fmt.Println(match, " / ", i)
		}
	}
	fmt.Println(match, " / ", nSamp)
}


