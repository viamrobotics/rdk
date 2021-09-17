package chessboard

import (
	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/mat"
	"image"
)

type ChessContoursConfiguration struct {
	CannyLow  float64 `json:"canny-low"`// initial threshold for pruning saddle points in saddle map
	CannyHigh float64 `json:"canny-high"`// minimum saddle score value for pruning

}


// BinarizeMat take a mat.Dense and returns a binary mat.Dense according to threshold value
func BinarizeMat(m *mat.Dense, thresh float64) *mat.Dense {
	out := mat.DenseCopyOf(m)
	nRows, nCols := m.Dims()
	originalSize := image.Point{nCols, nRows}
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		if m.At(y,x) >= thresh {
			out.Set(y,x,1)
		}else{
			out.Set(y,x,0)
		}
	})
	return out
}


func GetChessboardContours(img *rimage.Image) ([][]image.Point, []int) {
	// canny edges
	// binarize image
	// find contours
	// simplify contours
	// prune contours
	return nil, nil
}
