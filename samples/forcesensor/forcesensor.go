package forcesensor

import (
	"github.com/go-errors/errors"
)

type ForceMatrix struct {
	matrix   [][]int
	row_pins []string
	rows     int
	cols     int
}

func (fm *ForceMatrix) Init(rows int, cols int, row_pins []string) error {
	if len(row_pins) != rows {
		return errors.Errorf("number of row_pins is (%s) not equal to number of rows (%s)", len(row_pins), rows)
	}
	fm.rows = rows
	fm.cols = cols
	fm.row_pins = row_pins

	return nil
}

func (fm *ForceMatrix) SetMatrix(matrix [][]int) error {
	if len(matrix) == 0 {
		return errors.Errorf("the matrix is empty")
	}
	if len(matrix) != fm.rows || len(matrix[0]) != fm.cols {
		return errors.Errorf("the number of elements in the matrix don't match the set size")
	}
	fm.matrix = matrix
	return nil
}

func (fm *ForceMatrix) GetMatrix() (matrix [][]int) {
	return fm.matrix
}
