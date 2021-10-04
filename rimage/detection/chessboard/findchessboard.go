package chessboard

import (
	"github.com/golang/geo/r2"
)

// DetectionConfiguration stores the parameters necessary for chessboard detection in an image
type DetectionConfiguration struct {
	Saddle   SaddleConfiguration        `json:"saddle"`
	Contours ChessContoursConfiguration `json:"contours"`
	Greedy   ChessGreedyConfiguration   `json:"greedy"`
}

//var DefaultSaddleConf = SaddleConfiguration{
//	GrayThreshold:     128.,
//	ScoreThresholdMin: 10000.,
//	ScoreThresholdMax: 100000.,
//	NMSWindowSize:     10,
//}

//// nonMaxSuppression performs a non maximum suppression in a mat.Dense, with a window of size winSize
//func nonMaxSuppression(img *mat.Dense, winSize int) *mat.Dense {
//	h, w := img.Dims()
//	imgSup := mat.NewDense(h, w, nil)
//	for i := 0; i < h; i++ {
//		for j := 0; j < w; j++ {
//			if img.At(i, j) != 0 {
//				// get neighborhood limits
//				ta := utils.MaxInt(0, i-winSize)
//				tb := utils.MinInt(h, i+winSize+1)
//				tc := utils.MaxInt(0, j-winSize)
//				td := utils.MinInt(w, j+winSize+1)
//				// cell
//				cell := img.Slice(ta, tc, tb, td)
//				if mat.Max(cell) == img.At(i, j) {
//					imgSup.Set(i, j, img.At(i, j))
//				}
//			}
//		}
//	}
//	return imgSup
//}

// getMinSaddleDistance returns the saddle point that minimizes the distance with r2.Point pt, as well as this minimum
// distance
func getMinSaddleDistance(saddlePoints []r2.Point, pt r2.Point) (r2.Point, float64) {
	bestDist := 100000.
	bestPt := pt
	for _, saddlePt := range saddlePoints {
		diff := pt.Sub(saddlePt)
		dist := diff.Norm()
		if dist < bestDist {
			bestDist = dist
			bestPt = saddlePt

		}
	}
	return bestPt, bestDist
}

//// pruneSaddle prunes the saddle points map until the number of non-zero points reaches a value <= 10000
//func pruneSaddle(s *mat.Dense) {
//	thresh := 128.
//	r, c := s.Dims()
//	scores := mat.NewDense(r, c, nil)
//	scores.Apply(SumPositive, s)
//	score := mat.Sum(scores)
//	for score > 10000 {
//		thresh = thresh * 2
//		decFilt := func(r, c int, v float64) float64 {
//			if v < thresh {
//				return 0.
//			}
//			return v
//		}
//		//mask := mat.NewDense(r,c,nil)
//		s.Apply(decFilt, s)
//		scores.Apply(SumPositive, s)
//		score = mat.Sum(scores)
//	}
//}

//def getAngle(a, b, c):
//# Get angle given 3 side lengths, in degrees
//k = (a * a + b * b - c * c) / (2 * a * b)
//# Handle floating point errors
//if k < -1:
//k = -1
//elif k > 1:
//k = 1
//return np.arccos(k) * 180.0 / np.pi
