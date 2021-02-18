package pc

import (
	"github.com/viamrobotics/robotcore/utils"

	"gonum.org/v1/gonum/mat"
)

// creates a dense, expensive copy
func (pc *PointCloud) ToVec2Matrix() *utils.Vec2Matrix {
	zView := mat.DenseCopyOf(pc.DenseZ(0))
	grownZ := zView.Grow(1, 0).(*mat.Dense)
	_, c := grownZ.Dims()
	for i := 0; i < c; i++ {
		grownZ.Set(2, i, 1)
	}
	return (*utils.Vec2Matrix)(grownZ)
}
