package chessboard

import (
	"image"
	"math"

	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"
)

// ChessContoursConfiguration stores the parameters needed for contour precessing in chessboard detection
type ChessContoursConfiguration struct {
	CannyLow  float64 `json:"canny-low"`  // low threshold for Canny contours detection
	CannyHigh float64 `json:"canny-high"` // high threshold for Canny contours detection
	WinSize   int     `json:"win-size"`   // half window size for looking for saddle points in a chess corner neighborhood
}

// BinarizeMat take a mat.Dense and returns a binary mat.Dense according to threshold value
func BinarizeMat(m mat.Matrix, thresh float64) *mat.Dense {

	out := mat.DenseCopyOf(m)
	nRows, nCols := (m).Dims()
	originalSize := image.Point{nCols, nRows}
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		if (m).At(y, x) >= thresh {
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
	angles := []float64{ta, tb, tc, td}
	nGoodAngles := uint8(0)
	for _, angle := range angles {
		if angle > 40. && angle < 150. {
			nGoodAngles++
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
			X: pt.X,
			Y: pt.Y,
		}
	}
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
		bestRow -= int(math.Min(float64(winSize), float64(rowLow)))
		bestCol -= int(math.Min(float64(winSize), float64(colLow)))
		if saddleScore > 0. {
			newContour[i] = r2.Point{
				X: (cc + float64(bestCol)) / 2,
				Y: (rr + float64(bestRow)) / 2,
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
func PruneContours(contours [][]r2.Point, hierarchy []rimage.Node, saddleScoreMap *mat.Dense, winSize int) [][]r2.Point {
	newContours := make([][]r2.Point, 0)
	for i, c := range contours {
		cSorted := rimage.SortPointCounterClockwise(c)
		h := hierarchy[i+1]
		// we only want child contours
		if h.FirstChild != -1 {
			continue
		}
		// if a polygon contour has a very small side, remove it
		cnt := RemoveSmallSidePolygon(cSorted, 2.0)
		// select only quadrilaterals
		if len(cnt) != 4 {
			continue
		}
		// select contours that caver an area bigger than 64 pixels (8x8 pixel square).
		//TODO(louise): add this area in configuration file?
		if getContourBoundingBoxArea(cnt) < 64 {
			continue
		}
		if !IsContourSquare(cnt) {
			continue
		}
		cntUpdated := UpdateCorners(cnt, saddleScoreMap, winSize)

		if cntUpdated == nil {
			continue
		}
		newContours = append(newContours, cntUpdated)
	}
	return newContours
}

// RemoveSmallSidePolygon takes a polygonal contour as an input (CCW sorted); if a side of the polygon has a length < eps
// the end point of that side will be removed
func RemoveSmallSidePolygon(points []r2.Point, eps float64) []r2.Point {
	outPoints := make([]r2.Point, 0, len(points))
	for i := range points {
		p1 := points[i]
		p2 := points[(i+1)%len(points)]
		d := p1.Sub(p2).Norm()
		if d > eps {
			outPoints = append(outPoints, p1)
		}
	}
	return outPoints
}
