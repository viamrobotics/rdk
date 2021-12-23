package slipdetection

import (
	"errors"
	"math"
	"sync"

	"go.viam.com/rdk/sensor/forcematrix"
)

// ReadingsHistoryProvider represents a matrix sensor that supports slip detection
type ReadingsHistoryProvider interface {
	GetPreviousMatrices() [][][]int // an accessor for a history of matrix readings
}

// DetectSlip detects whether a slip has occurred. The version parameter determines
// which algorithm version to use
func DetectSlip(rhp ReadingsHistoryProvider, mu *sync.Mutex, version int, slipThreshold float64, framesToUse int) (bool, error) {
	var slipDetector func(ReadingsHistoryProvider, *sync.Mutex, float64, int) (bool, error)
	switch version {
	case 0:
		slipDetector = DetectSlipV0
	}
	if slipDetector != nil {
		return slipDetector(rhp, mu, slipThreshold, framesToUse)
	}
	return false, errors.New("version unsupported")
}

func makeEmptyMatrix(iDim int, jDim int) [][]float64 {
	resultMatrix := make([][]float64, 0)
	for i := 0; i < iDim; i++ {
		zeroRow := make([]float64, jDim)
		resultMatrix = append(resultMatrix, zeroRow)
	}
	return resultMatrix
}

func getMatrixStateDiff(matrixA [][]float64, matrixB [][]float64, slipThreshold float64) [][]int {
	result := make([][]int, 0)
	for i := 0; i < len(matrixA); i++ {
		row := make([]int, len(matrixA[i]))
		for j := 0; j < len(matrixA[i]); j++ {
			rawVal := matrixB[i][j] - matrixA[i][j]
			var val int
			if math.Abs(rawVal) > slipThreshold {
				val = 1
			} else {
				val = 0
			}
			row = append(row, val)
		}
		result = append(result, row)
	}
	return result
}

func getAverageValues(matrices [][][]int) [][]float64 {
	numMatrices := float64(len(matrices))
	sumMatrix := makeEmptyMatrix(len(matrices[0]), len(matrices[0][0]))

	for _, matrix := range matrices {
		for j, row := range sumMatrix {
			for k := 0; k < len(row); k++ {
				row[k] += float64(matrix[j][k])
			}
		}
	}

	for _, summedRow := range sumMatrix {
		for m := 0; m < len(summedRow); m++ {
			summedRow[m] = summedRow[m] / numMatrices
		}
	}

	return sumMatrix
}

func isEmptyState(matrix [][]int) bool {
	for _, row := range matrix {
		for _, val := range row {
			if val != 0 {
				return false
			}
		}
	}
	return true
}

// DetectSlipV0 implements version 0 of a slip detection algorithm
func DetectSlipV0(rhp ReadingsHistoryProvider, mu *sync.Mutex, slipThreshold float64, framesToUse int) (bool, error) {
	mu.Lock()
	defer mu.Unlock()

	matrices := rhp.GetPreviousMatrices()
	numRecordedMatrices := len(matrices)

	if numRecordedMatrices < framesToUse {
		return false, nil
	}
	if framesToUse > forcematrix.MatrixStorageSize {
		return false, errors.New("max frames exceeded")
	}

	matrices = matrices[(numRecordedMatrices - framesToUse):]
	numMatrices := len(matrices)

	previousFrame := getAverageValues(matrices[:(numMatrices / 2)])
	currentFrame := getAverageValues(matrices[numMatrices/2:])
	diff := getMatrixStateDiff(previousFrame, currentFrame, slipThreshold)
	return !isEmptyState(diff), nil
}
