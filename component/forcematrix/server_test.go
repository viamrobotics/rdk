package forcematrix_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/forcematrix"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testForceMatrixName    = "forcematrix1"
	fakeForceMatrixName    = "forcematrix2"
	missingForceMatrixName = "forcematrix3"
)

func newServer() (pb.ForceMatrixServiceServer, *inject.ForceMatrix, error) {
	injectForceMatrix := &inject.ForceMatrix{}
	forceMatrices := map[resource.Name]interface{}{
		forcematrix.Named(testForceMatrixName): injectForceMatrix,
		forcematrix.Named(fakeForceMatrixName): "notForceMatrix",
	}
	forceMatrixSvc, err := subtype.New(forceMatrices)
	if err != nil {
		return nil, nil, err
	}
	return forcematrix.NewServer(forceMatrixSvc), injectForceMatrix, nil
}

func TestServer(t *testing.T) {
	forceMatrixServer, injectForceMatrix, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	expectedMatrix := make([][]int, 4)
	for i := 0; i < len(expectedMatrix); i++ {
		expectedMatrix[i] = []int{1, 2, 3, 4}
	}
	var capMatrix [][]int
	injectForceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		capMatrix = expectedMatrix
		return expectedMatrix, nil
	}
	injectForceMatrix.IsSlippingFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	t.Run("not registered", func(t *testing.T) {
		_, err := forceMatrixServer.Matrix(
			context.Background(),
			&pb.ForceMatrixServiceMatrixRequest{Name: missingForceMatrixName})
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"no ForceMatrix with name ("+missingForceMatrixName+")")

		_, err = forceMatrixServer.SlipDetection(
			context.Background(),
			&pb.ForceMatrixServiceSlipDetectionRequest{Name: missingForceMatrixName})
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"no ForceMatrix with name ("+missingForceMatrixName+")")
	})

	t.Run("not a ForceMatrix", func(t *testing.T) {
		_, err := forceMatrixServer.Matrix(
			context.Background(),
			&pb.ForceMatrixServiceMatrixRequest{Name: fakeForceMatrixName})
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"resource with name ("+fakeForceMatrixName+") is not a ForceMatrix")

		_, err = forceMatrixServer.SlipDetection(
			context.Background(),
			&pb.ForceMatrixServiceSlipDetectionRequest{Name: fakeForceMatrixName})
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"resource with name ("+fakeForceMatrixName+") is not a ForceMatrix")
	})

	t.Run("working", func(t *testing.T) {
		matrixResponse, err := forceMatrixServer.Matrix(
			context.Background(),
			&pb.ForceMatrixServiceMatrixRequest{Name: testForceMatrixName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capMatrix, test.ShouldResemble, expectedMatrix)
		test.That(t, matrixResponse.Matrix, test.ShouldResemble,
			&pb.Matrix{
				Rows: 4,
				Cols: 4,
				Data: []uint32{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4},
			})

		slipResponse, err := forceMatrixServer.SlipDetection(
			context.Background(),
			&pb.ForceMatrixServiceSlipDetectionRequest{Name: testForceMatrixName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, slipResponse.IsSlipping, test.ShouldBeTrue)
	})
}
