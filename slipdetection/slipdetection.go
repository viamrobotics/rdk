package slipdetection

import (
	"errors"
	"sync"

	"go.viam.com/core/sensor/forcematrix"
)

// SlipDetector represents a matrix sensor that supports slip
// detection
type SlipDetector interface {
	GetPreviousMatrices() [][][]int // an accessor for a history of matrix readings
}

// TODO: should be made dynamic according to sensitivity?
const readingThreshold = 40.0

// DetectSlip detects whether a slip has occurred. The version parameter determines
// which algorithm version to use
func DetectSlip(fmsm SlipDetector, mu *sync.Mutex, version int, framesToUse int) (bool, error) {
	var slipDetector func(SlipDetector, *sync.Mutex, int) (bool, error)
	if version == 0 {
		slipDetector = DetectSlipV0
	}
	if slipDetector != nil {
		return slipDetector(fmsm, mu, framesToUse)
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

func getMatrixStateDiff(matrixA [][]float64, matrixB [][]float64) [][]int {
	result := make([][]int, 0)
	for i := 0; i < len(matrixA); i++ {
		row := make([]int, len(matrixA[i]))
		for j := 0; j < len(matrixA[i]); j++ {
			rawVal := matrixB[i][j] - matrixA[i][j]
			var val int
			if (rawVal > readingThreshold) || ((-1 * rawVal) > readingThreshold) {
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
func DetectSlipV0(fmsm SlipDetector, mu *sync.Mutex, framesToUse int) (bool, error) {
	mu.Lock()
	defer mu.Unlock()

	matrices := fmsm.GetPreviousMatrices()
	numRecordedMatrices := len(matrices)

	if numRecordedMatrices < framesToUse {
		return false, nil
	}
	if framesToUse > forcematrix.MatrixStorageSize {
		return false, errors.New("max frames exceeded")
	}

	matrices = matrices[(numRecordedMatrices - framesToUse):]
	numMatrices := len(matrices)

	previousFrame := getAverageValues(matrices[0:(numMatrices / 2)])
	currentFrame := getAverageValues(matrices[numMatrices/2 : numMatrices])
	diff := getMatrixStateDiff(previousFrame, currentFrame)
	return !isEmptyState(diff), nil
}
