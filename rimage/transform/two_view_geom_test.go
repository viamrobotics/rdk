package transform

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/logging"
)

type poseGroundTruth struct {
	Pts1 [][]float64 `json:"pts1"`
	Pts2 [][]float64 `json:"pts2"`
	R    [][]float64 `json:"rot"`         // TODO(RSDK-568): unit?
	T    [][]float64 `json:"translation"` // TODO(RSDK-568): unit?
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

func readJSONGroundTruth(logger logging.Logger) *poseGroundTruth {
	// Open jsonFile
	jsonFile, err := os.Open(artifact.MustPath("rimage/matched_kps.json"))
	if err != nil {
		return nil
	}
	logger.Info("Ground Truth json file successfully loaded")
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := io.ReadAll(jsonFile)

	// initialize poseGroundTruth
	var gt poseGroundTruth

	// unmarshal byteArray
	json.Unmarshal(byteValue, &gt)
	return &gt
}

func TestComputeFundamentalMatrix(t *testing.T) {
	logger := logging.NewTestLogger(t)
	gt := readJSONGroundTruth(logger)
	pts1 := convert2DSliceToVectorSlice(gt.Pts1)
	pts2 := convert2DSliceToVectorSlice(gt.Pts2)
	F2, err := ComputeFundamentalMatrixAllPoints(pts1, pts2, true)
	test.That(t, err, test.ShouldBeNil)
	// test that x2^T @ F @ x1 approx 0
	var res1, res2 mat.Dense
	v1 := mat.NewDense(3, 1, []float64{pts1[0].X, pts1[0].Y, 1})
	v2 := mat.NewDense(1, 3, []float64{pts2[0].X, pts2[0].Y, 1})
	res1.Mul(F2, v1)
	res2.Mul(v2, &res1)
	test.That(t, res2.At(0, 0), test.ShouldBeLessThan, 0.01)
	// essential matrix
	K := convert2DSliceToDense(gt.K)
	E, err := GetEssentialMatrixFromFundamental(K, K, F2)
	test.That(t, err, test.ShouldBeNil)
	// test that xHat2^T @ E @ xHat1 approx 0, with xHat = K^-1 @ x
	eNorm := mat.Norm(E, 2)
	E.Scale(1./eNorm, E)
	var res3, res4 mat.Dense
	var Kinv, x1Hat, x2Hat mat.Dense
	err = Kinv.Inverse(K)
	test.That(t, err, test.ShouldBeNil)
	x1Hat.Mul(&Kinv, v1)
	x2Hat.Mul(&Kinv, transposeDense(v2))
	x2HatT := transposeDense(&x2Hat)
	res3.Mul(E, &x1Hat)
	res4.Mul(x2HatT, &res3)
	test.That(t, res4.At(0, 0), test.ShouldBeLessThan, 0.0001)
}
