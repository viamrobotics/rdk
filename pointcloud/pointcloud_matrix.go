package pointcloud

import (
	"go.viam.com/rdk/utils"

	"gonum.org/v1/gonum/mat"
)

// ToVec2Matrix here creates a dense, expensive copy.
func (pc *basicPointCloud) ToVec2Matrix() (*utils.Vec2Matrix, error) {
	denseZ, err := pc.DenseZ(0)
	if err != nil {
		return nil, err
	}
	zView := mat.DenseCopyOf(denseZ)
	grownZ := zView.Grow(1, 0).(*mat.Dense)
	_, c := grownZ.Dims()
	for i := 0; i < c; i++ {
		grownZ.Set(2, i, 1)
	}
	return (*utils.Vec2Matrix)(grownZ), nil
}
