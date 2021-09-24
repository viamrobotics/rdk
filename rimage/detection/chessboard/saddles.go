package chessboard

import (
	"image"
	"math"

	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/mat"
)

type SaddleConfiguration struct {
	GrayThreshold     float64 `json:"gray"`      // initial threshold for pruning saddle points in saddle map
	ScoreThresholdMin float64 `json:"score-min"` // minimum saddle score value for pruning
	ScoreThresholdMax float64 `json:"score-max"` // saddle score above which non pruned or suppressed points are saddle points
	NMSWindowSize     int     `json:"win-size"`  // window size for non-maximum suppression
}

var DefaultSaddleConf = SaddleConfiguration{
	GrayThreshold:     128.,
	ScoreThresholdMin: 10000.,
	ScoreThresholdMax: 100000.,
	NMSWindowSize:     10,
}

// computePixelWiseHessianDeterminant computes hessian components for each pixel and returns a *mat.Dense containing
// the value of the determinant of the Hessian for each pixel
// The sign and value of the determinant of the Hessian gives location of saddle points
func computePixelWiseHessianDeterminant(img *mat.Dense) (*mat.Dense, error) {
	nRows, nCols := img.Dims()
	sobelX := rimage.GetSobelX()
	sobelY := rimage.GetSobelY()
	gX, err := rimage.ConvolveGrayFloat64(img, &sobelX)
	if err != nil {
		return nil, err
	}
	gY, err := rimage.ConvolveGrayFloat64(img, &sobelY)
	if err != nil {
		return nil, err
	}
	gXX, err := rimage.ConvolveGrayFloat64(gX, &sobelX)
	if err != nil {
		return nil, err
	}
	gYY, err := rimage.ConvolveGrayFloat64(gY, &sobelY)
	if err != nil {
		return nil, err
	}
	gXY, err := rimage.ConvolveGrayFloat64(gX, &sobelY)
	if err != nil {
		return nil, err
	}
	m1 := mat.NewDense(nRows, nCols, nil)
	m2 := mat.NewDense(nRows, nCols, nil)
	out := mat.NewDense(nRows, nCols, nil)
	m1.MulElem(gXX, gYY)
	m2.MulElem(gXY, gXY)
	out.Sub(m1, m2)
	return out, nil
}

func SumPositive(i, j int, val float64) float64 {
	if val > 0 {
		return 1.
	}
	return 0.
}

// PruneSaddle prunes the saddle points map until the number of non-zero points reaches a value <= 10000
func PruneSaddle(s *mat.Dense, cfg *SaddleConfiguration) *mat.Dense {
	thresh := cfg.GrayThreshold
	r, c := s.Dims()
	scores := mat.NewDense(r, c, nil)
	pruned := mat.DenseCopyOf(s)
	scores.Apply(SumPositive, s)
	score := mat.Sum(scores)
	for score > cfg.ScoreThresholdMin {
		thresh = thresh * 2
		decFilt := func(r, c int, v float64) float64 {
			if v < thresh {
				return 0.
			}
			return v
		}
		//mask := mat.NewDense(r,c,nil)
		pruned.Apply(decFilt, pruned)
		scores.Apply(SumPositive, pruned)
		score = mat.Sum(scores)
	}
	return pruned
}

// NonMaxSuppression performs a non-maximum suppression in a mat.Dense, with a window of size winSize
func NonMaxSuppression(img *mat.Dense, winSize int) *mat.Dense {
	h, w := img.Dims()
	imgSup := mat.NewDense(h, w, nil)
	for i := 0; i < h; i++ {
		for j := 0; j < w; j++ {
			if img.At(i, j) != 0 {
				// get neighborhood limits
				ta := int(math.Max(0, float64(i-winSize)))
				tb := int(math.Min(float64(h), float64(i+winSize+1)))
				tc := int(math.Max(0, float64(j-winSize)))
				td := int(math.Min(float64(w), float64(j+winSize+1)))
				// cell
				cell := img.Slice(ta, tb, tc, td)

				if mat.Max(cell) == img.At(i, j) {
					imgSup.Set(i, j, img.At(i, j))
				}
			}
		}
	}
	return imgSup
}

// GetSaddleMapPoints gets a saddle point presence map and a slice of relevant saddle points
func GetSaddleMapPoints(img *mat.Dense, conf *SaddleConfiguration) (*mat.Dense, []image.Point, error) {
	nRows, nCols := img.Dims()
	originalSize := image.Point{nCols, nRows}
	hessian, err := computePixelWiseHessianDeterminant(img)
	if err != nil {
		return nil, nil, err
	}
	// saddle points are points where determinant of hessian is <0
	// for better readability, using negative determinant of Hessian
	hessian.Scale(-1.0, hessian)
	// Set all points < 0 to 0
	thresh := 0.
	decFilt := func(r, c int, v float64) float64 {
		if v < thresh {
			return 0.
		}
		return v
	}
	saddleMap := mat.NewDense(nRows, nCols, nil)
	saddleMap.Apply(decFilt, hessian)
	// pruning saddle point map
	saddleMap = PruneSaddle(saddleMap, conf)
	// non maximum suppression
	nms := NonMaxSuppression(saddleMap, conf.NMSWindowSize)
	// threshold nms saddle map to get saddle points
	saddlePoints := make([]image.Point, 0)
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		if nms.At(y, x) >= conf.ScoreThresholdMax {
			saddlePoints = append(saddlePoints, image.Point{x, y})
		}
	})

	return saddleMap, saddlePoints, nil
}
