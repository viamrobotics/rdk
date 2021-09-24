package chessboard

import (
	"fmt"
	"image"
	"math"

	"github.com/golang/geo/r2"
	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/mat"
)

type ChessContoursConfiguration struct {
	CannyLow  float64 `json:"canny-low"`  // initial threshold for pruning saddle points in saddle map
	CannyHigh float64 `json:"canny-high"` // minimum saddle score value for pruning
	WinSize   int     `json:"win-size"`   // half window size for looking for saddle points in a chess corner neighborhood
}

// BinarizeMat take a mat.Dense and returns a binary mat.Dense according to threshold value
func BinarizeMat(m *mat.Dense, thresh float64) *mat.Dense {
	out := mat.DenseCopyOf(m)
	nRows, nCols := m.Dims()
	originalSize := image.Point{nCols, nRows}
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		if m.At(y, x) >= thresh {
			out.Set(y, x, 1)
		} else {
			out.Set(y, x, 0)
		}
	})
	return out
}

// GetAngle computes the angle given a 3 square side lengths, in degrees
func GetAngle(a, b, c float64) float64 {
	k := (a*a + b*b - c*c) / (2 * a * b)
	// handle floating point issues
	k = utils.ClampF64(k, -1, 1)
	return math.Acos(k) * 180 / math.Pi
}

// IsContourSquare takes a 4 points contour and checks if it corresponds to a square transformed by a homography
func IsContourSquare(contour []r2.Point) bool {
	isSquare := false
	if len(contour) != 4 {
		return isSquare
	}
	p0 := contour[0]
	p1 := contour[1]
	p2 := contour[2]
	p3 := contour[3]
	// side lengths
	dd0 := p0.Sub(p1).Norm()
	dd1 := p1.Sub(p2).Norm()
	dd2 := p2.Sub(p3).Norm()
	dd3 := p3.Sub(p0).Norm()
	// diagonal lengths
	xa := p0.Sub(p2).Norm()
	xb := p1.Sub(p3).Norm()
	// check that points in contour are part of a convex hull
	ta := GetAngle(dd3, dd0, xb)
	tb := GetAngle(dd0, dd1, xa)
	tc := GetAngle(dd1, dd2, xb)
	td := GetAngle(dd2, dd3, xa)
	fmt.Println(ta, tb, tc, td)
	angles := []float64{ta, tb, tc, td}
	nGoodAngles := uint8(0)
	for _, angle := range angles {
		if angle > 40. && angle < 150. {
			nGoodAngles += 1
		}
	}
	isSquare = nGoodAngles == 4
	return isSquare
}

// UpdateCorners take a contour, checks the maximum saddle score in a window of size winSize around each point in the contour
// if the saddle score is > 0, replace the contour point with the point with the maximum saddle score
// if one of the contour points is not close to a saddle point, returns nil
func UpdateCorners(contour []r2.Point, saddleScoreMap *mat.Dense, winSize int) []r2.Point {
	nRows, nCols := saddleScoreMap.Dims()
	// copy contour
	newContour := make([]r2.Point, len(contour))
	for i, pt := range contour {
		newContour[i] = r2.Point{
			X: float64(pt.X),
			Y: float64(pt.Y),
		}
	}

	// initialize score slice
	scores := make([]float64, 0, len(contour))
	// go through contour
	for i, pt := range contour {
		cc, rr := pt.X, pt.Y
		rowLow := int(math.Max(0, rr-float64(winSize)))
		colLow := int(math.Max(0, cc-float64(winSize)))
		rowHigh := int(math.Min(float64(nRows), rr+float64(winSize)))
		colHigh := int(math.Min(float64(nCols), cc+float64(winSize)))
		bestRow, bestCol := rowLow, colLow
		saddleScore := saddleScoreMap.At(rowLow, colLow)
		// find maximum saddle score in window
		for r := rowLow; r < rowHigh; r++ {
			for c := colLow; c < colHigh; c++ {
				if saddleScoreMap.At(r, c) > saddleScore {
					bestRow = r
					bestCol = c
					saddleScore = saddleScoreMap.At(r, c)
				}
			}
		}
		scores = append(scores, saddleScore)
		bestRow -= int(math.Min(float64(winSize), float64(rowLow)))
		bestCol -= int(math.Min(float64(winSize), float64(colLow)))
		if saddleScore > 0. {
			newContour[i] = r2.Point{
				X: cc + float64(bestCol),
				Y: rr + float64(bestRow),
			}
		} else {
			return nil
		}
	}
	return newContour
}

// getContourBoundingBoxArea gets the bounding box around a contours and computes the area of that box
func getContourBoundingBoxArea(contour []r2.Point) float64 {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := 0.0, 0.0
	for _, pt := range contour {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	w := maxX - minX
	h := maxY - minY
	return w * h
}

// PruneContours keeps contours that correspond to a chessboard square
func PruneContours(contours [][]r2.Point, hierarchy [][]int, saddleScoreMap *mat.Dense, winSize int) [][]r2.Point {
	newContours := make([][]r2.Point, 0)
	for i, c := range contours {
		cSorted := rimage.SortPointCounterClockwise(c)
		fmt.Println(cSorted)
		h := hierarchy[i]
		// we only want child contours
		if h[2] != -1 {
			continue
		}
		// select only quadrilaterals
		if len(c) != 4 {
			continue
		}
		// select contours that caver an area bigger than 64 pixels (8x8 pixel square)
		if getContourBoundingBoxArea(c) < 64 {
			continue
		}
		if !IsContourSquare(cSorted) {
			continue
		}
		cnt := UpdateCorners(cSorted, saddleScoreMap, winSize)
		if cnt == nil {
			continue
		}
		newContours = append(newContours, cnt)
	}
	return newContours
}
