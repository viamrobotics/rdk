package pointcloud

import (
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

// ToVec2Matrix here creates a dense, expensive copy.
func (cloud *basicPointCloud) ToVec2Matrix() (*utils.Vec2Matrix, error) {
	denseZ, err := cloud.DenseZ(0)
	if err != nil {
		return nil, err
	}
	zView := mat.DenseCopyOf(denseZ)
	grownZ := zView.Grow(1, 0)
	grownZMat, ok := grownZ.(*mat.Dense)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(grownZMat, grownZ)
	}
	_, c := grownZ.Dims()
	for i := 0; i < c; i++ {
		grownZMat.Set(2, i, 1)
	}
	return (*utils.Vec2Matrix)(grownZMat), nil
}
