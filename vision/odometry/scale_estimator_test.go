package odometry

import (
	"encoding/json"
	"fmt"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func generatePointZEqualsZeroPlane(n int) []r3.Vector {
	points := make([]r3.Vector, n)
	for i := 0; i < n; i++ {
		x := rand.Float64()
		y := rand.Float64()
		points[i] = r3.Vector{x, y, 0}
	}
	return points
}

var logger = golog.NewLogger("test_cam_height_estimation")

type poseGroundTruth struct {
	Pts1 [][]float64 `json:"pts1"`
	Pts2 [][]float64 `json:"pts2"`
	R    [][]float64 `json:"rot"`
	T    [][]float64 `json:"translation"`
	K    [][]float64 `json:"cam_mat"`
	F    [][]float64 `json:"fundamental_matrix"`
}

func convert2DSliceToVectorSlice(points [][]float64) []r2.Point {
	vecs := make([]r2.Point, len(points))
	for i, pt := range points {
		vecs[i] = r2.Point{
			X: pt[0],
			Y: pt[1],
		}
	}
	return vecs
}

func convert2DSliceToDense(data [][]float64) *mat.Dense {
	m := len(data)
	n := len(data[0])
	out := mat.NewDense(m, n, nil)
	for i, row := range data {
		out.SetRow(i, row)
	}
	return out
}

func readJSONGroundTruth() *poseGroundTruth {
	// Open jsonFile
	jsonFile, err := os.Open(artifact.MustPath("rimage/matched_kps.json"))
	if err != nil {
		return nil
	}
	logger.Info("Ground Truth json file successfully loaded")
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// initialize poseGroundTruth
	var gt poseGroundTruth

	// unmarshal byteArray
	json.Unmarshal(byteValue, &gt)
	return &gt
}

func TestGetTriangleNormalVector(t *testing.T) {
	points := []r3.Vector{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}}
	normal := getTriangleNormalVector(points)
	test.That(t, normal.X, test.ShouldEqual, 0)
	test.That(t, normal.Y, test.ShouldEqual, 0)
	test.That(t, normal.Z, test.ShouldEqual, -1)
}

func TestPlaneFrom3PointsDistance(t *testing.T) {
	points := []r3.Vector{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}}
	normal, offset := estimatePlaneFrom3Points(points[0], points[1], points[2])
	test.That(t, normal.X, test.ShouldEqual, 0)
	test.That(t, normal.Y, test.ShouldEqual, 0)
	test.That(t, normal.Z, test.ShouldEqual, -1)
	test.That(t, offset, test.ShouldAlmostEqual, 0)
	pt := r3.Vector{1, 1, 1}
	dist := distToPlane(pt, normal, offset)
	test.That(t, dist, test.ShouldEqual, 1)
}

func TestGetPlaneInliers(t *testing.T) {
	points := generatePointZEqualsZeroPlane(1000)
	normal := r3.Vector{0, 0, 1}
	offset := 0.0
	inliers := getPlaneInliers(points, normal, offset, 0.0001)
	test.That(t, len(inliers), test.ShouldEqual, 1000)
	// test parallel plane with distance > threshold
	offset = 3.0
	inliersOffset3 := getPlaneInliers(points, normal, offset, 0.0001)
	test.That(t, len(inliersOffset3), test.ShouldEqual, 0)
	// test parallel plane with distance < threshold
	offset = 0.1
	inliersSmallOffset := getPlaneInliers(points, normal, offset, 0.25)
	test.That(t, len(inliersSmallOffset), test.ShouldEqual, 1000)
}

func TestEstimatePitch(t *testing.T) {
	// get pose from kitti odometry dataset
	poseData := []float64{9.999996e-01, -9.035185e-04, -2.101169e-04, 1.289128e-03,
		9.037964e-04, 9.999987e-01, 1.325646e-03, -1.821616e-02,
		2.089193e-04, -1.325834e-03, 9.999991e-01, 1.310643e+00,
	}
	// get our camera pose structure
	poseMat := mat.NewDense(3, 4, poseData)
	pose := transform.NewCamPoseFromMat(poseMat)
	// estimate pitch
	pitch := estimatePitchFromCameraPose(pose)
	pitchDegrees := pitch * 180 / math.Pi
	// test pitch value is similar to the KITTI GT
	test.That(t, pitchDegrees, test.ShouldAlmostEqual, -1.0437668176234958)

	// test camera height - with small pitch, height should be close to Y coordinate (<3-5 cm)
	pt := r3.Vector{10, 1.73, 1.5}
	height := getCameraHeightFromGroundPoint(pt, pitch)
	test.That(t, math.Abs(height-pt.Y), test.ShouldBeLessThan, 0.03)

}

func TestEstimateCameraHeight(t *testing.T) {
	gt := readJSONGroundTruth()
	pts1 := convert2DSliceToVectorSlice(gt.Pts1)
	pts2 := convert2DSliceToVectorSlice(gt.Pts2)
	//pts1H := transform.Convert2DPointsToHomogeneousPoints(pts1)
	//pts2H := transform.Convert2DPointsToHomogeneousPoints(pts2)
	T := convert2DSliceToDense(gt.T)
	R := convert2DSliceToDense(gt.R)
	poseMat := mat.NewDense(3, 4, nil)
	poseMat.Augment(R, T)
	fmt.Println(mat.Formatted(poseMat))
	pose := transform.NewCamPoseFromMat(poseMat)
	height, err := EstimateCameraHeight(pts1, pts2, pose, 0.97, 0.005)
	test.That(t, err, test.ShouldBeNil)
	fmt.Println(height)

}
