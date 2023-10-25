package transform

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestGetCorrectCameraPose(t *testing.T) {
	logger := logging.NewTestLogger(t)
	gt := readJSONGroundTruth(logger)

	pts1 := convert2DSliceToVectorSlice(gt.Pts1)
	pts2 := convert2DSliceToVectorSlice(gt.Pts2)
	K := convert2DSliceToDense(gt.K)
	rows, cols := K.Dims()
	test.That(t, rows, test.ShouldEqual, 3)
	test.That(t, cols, test.ShouldEqual, 3)
	test.That(t, len(pts1), test.ShouldEqual, len(pts2))
	// test pose does not return error
	pose, err := EstimateNewPose(pts1, pts2, K)
	test.That(t, err, test.ShouldBeNil)
	// test dimensions of pose matrix: 3x4
	nRows, nCols := pose.PoseMat.Dims()
	test.That(t, nRows, test.ShouldEqual, 3)
	test.That(t, nCols, test.ShouldEqual, 4)
	// test dimensions of rotation matrix: 3x3
	nRowsR, nColsR := pose.Rotation.Dims()
	test.That(t, nRowsR, test.ShouldEqual, 3)
	test.That(t, nColsR, test.ShouldEqual, 3)
	// test values for 3d translation vector
	test.That(t, pose.Translation.At(2, 0), test.ShouldAlmostEqual, -0.9946075890134962)
	test.That(t, pose.Translation.At(1, 0), test.ShouldBeLessThan, 0.05)
	test.That(t, pose.Translation.At(0, 0), test.ShouldBeLessThan, 0.1)
	// test diagonal elements of rotation matrix
	test.That(t, math.Abs(pose.Rotation.At(0, 0)), test.ShouldBeBetween, 0.98, 1.0)
	test.That(t, math.Abs(pose.Rotation.At(1, 1)), test.ShouldBeBetween, 0.99, 1.0)
	test.That(t, math.Abs(pose.Rotation.At(2, 2)), test.ShouldBeBetween, 0.97, 1.0)

	// test Pose function
	poseSpatialMath, err := pose.Pose()
	test.That(t, err, test.ShouldBeNil)
	t1 := poseSpatialMath.Point()
	test.That(t, math.Abs(t1.X-pose.Translation.At(0, 0)), test.ShouldBeLessThan, 0.0000001)
	test.That(t, math.Abs(t1.Y-pose.Translation.At(1, 0)), test.ShouldBeLessThan, 0.0000001)
	test.That(t, math.Abs(t1.Z-pose.Translation.At(2, 0)), test.ShouldBeLessThan, 0.0000001)
	rot := poseSpatialMath.Orientation().RotationMatrix()
	test.That(t, math.Abs(rot.At(0, 0)-pose.Rotation.At(0, 0)), test.ShouldBeLessThan, 0.0000001)
}
