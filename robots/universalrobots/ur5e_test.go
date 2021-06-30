package universalrobots

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/core/arm"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/test"
	pb "go.viam.com/core/proto/api/v1"
)

func testUR5eForwardKinements(t *testing.T, jointRadians []float64, correct *pb.ArmPosition) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	
	dummy := inject.Arm{}

	a, err := kinematics.NewArm(&dummy, ur5modeljson, 4, logger)
	test.That(t, err, test.ShouldBeNil)
	
	dummy.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return arm.JointPositionsFromRadians(jointRadians), nil
	}
	
	pos, err := a.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.X, test.ShouldAlmostEqual, correct.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, correct.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, correct.Z, .01)

	// TODO(erh): check orientation
}

func TestKin1(t *testing.T) {
	// data came from excel file found here
	// https://www.universal-robots.com/articles/ur/application-installation/dh-parameters-for-calculations-of-kinematics-and-dynamics/
	// https://s3-eu-west-1.amazonaws.com/ur-support-site/45257/DH-Transformation.xlsx
	// Note: we use millimeters, they use meters

	// Section 1 - first we test each joint independantly

	//    Home
	testUR5eForwardKinements(t, []float64{0,0,0,0,0,0}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})

	//    Joint 0
	testUR5eForwardKinements(t, []float64{math.Pi/2,0,0,0,0,0}, &pb.ArmPosition{X: 232.90, Y: -817.2, Z: 62.80})
	testUR5eForwardKinements(t, []float64{math.Pi,0,0,0,0,0}, &pb.ArmPosition{X: 817.2, Y: 232.90, Z: 62.80})

	//    Joint 1
	testUR5eForwardKinements(t, []float64{0,math.Pi/2,0,0,0,0}, &pb.ArmPosition{X: 99.7, Y: -232.90, Z: -654.70})
	testUR5eForwardKinements(t, []float64{0,math.Pi,0,0,0,0}, &pb.ArmPosition{X: 817.2, Y: -232.90, Z: 262.2})

	//    Joint 2
	testUR5eForwardKinements(t, []float64{0,0,math.Pi/2,0,0,0}, &pb.ArmPosition{X: -325.3, Y: -232.90, Z: -229.7})
	testUR5eForwardKinements(t, []float64{0,0,math.Pi,0,0,0}, &pb.ArmPosition{X: -32.8, Y: -232.90, Z: 262.2})
	
	//    Joint 3
	testUR5eForwardKinements(t, []float64{0,0,0,math.Pi/2,0,0}, &pb.ArmPosition{X: -717.5, Y: -232.90, Z: 162.5})
	testUR5eForwardKinements(t, []float64{0,0,0,math.Pi,0,0}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 262.2})

	//    Joint 4
	testUR5eForwardKinements(t, []float64{0,0,0,0,math.Pi/2,0}, &pb.ArmPosition{X: -916.80, Y: -133.3, Z: 62.8})
	testUR5eForwardKinements(t, []float64{0,0,0,0,math.Pi,0}, &pb.ArmPosition{X: -817.2, Y: -33.7, Z: 62.8})

	//    Joint 5
	testUR5eForwardKinements(t, []float64{0,0,0,0,0,math.Pi/2}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})
	testUR5eForwardKinements(t, []float64{0,0,0,0,0,math.Pi}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})

	// Section 2 - try some consistent angle
	rad := math.Pi / 4
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: 16.62, Y: -271.49, Z: -509.52})
	
	rad = math.Pi / 2
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: 133.3, Y: 292.5, Z: -162.9})

	rad = math.Pi
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: -32.8, Y: 33.7, Z: 262.2})	

	// Section 3 - try some random angles
	testUR5eForwardKinements(t, []float64{math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2, 0}, &pb.ArmPosition{X: 193.91, Y: 5.39, Z: -654.63})
	testUR5eForwardKinements(t, []float64{0, math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2}, &pb.ArmPosition{X: 97.11, Y: -203.73, Z: -394.65})

}
