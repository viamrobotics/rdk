package transform

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
)

var (
	logger = golog.NewLogger("test_cam_pose_estimation")
)

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

func readJsonGroundTruth() *poseGroundTruth {
	// Open our jsonFile
	jsonFile, err := os.Open(artifact.MustPath("rimage/matched_kps.json"))
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil
	}
	logger.Info("Ground Truth json file successfully loaded")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our Users array
	var gt poseGroundTruth

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	json.Unmarshal(byteValue, &gt)
	return &gt
}

func TestComputeFundamentalMatrix(t *testing.T) {
	gt := readJsonGroundTruth()
	pts1 := convert2DSliceToVectorSlice(gt.Pts1)
	pts2 := convert2DSliceToVectorSlice(gt.Pts2)
	F2, err := ComputeFundamentalMatrixAllPoints(pts1, pts2, true)
	fmt.Println("F1: ", mat.Formatted(F2))
	test.That(t, err, test.ShouldBeNil)

	var res1, res2 mat.Dense
	v1 := mat.NewDense(3, 1, []float64{pts1[0].X, pts1[0].Y, 1})
	v2 := mat.NewDense(1, 3, []float64{pts2[0].X, pts2[0].Y, 1})
	res1.Mul(F2, v1)
	res2.Mul(v2, &res1)
	fmt.Println(mat.Formatted(&res2))
	// essential matrix
	K := convert2DSliceToDense(gt.K)
	E, err := GetEssentialMatrixFromFundamental(K, K, F2)
	fmt.Println("E: ", mat.Formatted(E))
}
