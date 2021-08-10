package rimage

import (
	"gonum.org/v1/gonum/mat"
	"math"
)

var NPixelNeighbors = 8

// Point represents a 2D point on a Cartesian plane.
type Point struct {
	X float64
	Y float64
}

// Line represents a line segment.
type Line struct {
	Start Point
	End   Point
}

// Contour is a structure that stores and represent a contour with the points in it, its ID, ParentID and a flag
// that stores if the contour is a hole contour or not
type Contour struct {
	Points   []Point
	Id       int
	ParentId int
	IsHole   bool
}

// NewContour create a pointer to an empty contour with ID 0
func NewContour() *Contour {
	return &Contour{
		Points:   nil,
		Id:       0,
		ParentId: 0,
		IsHole:   false,
	}
}

// NewContourWithID creates a new contour with a given ID
func NewContourWithID(idx int) *Contour {
	return &Contour{
		Points:   nil,
		Id:       idx,
		ParentId: 0,
		IsHole:   false,
	}
}

// neighborIDToIndex maps a neigh ID to the image coordinates
func neighborIDToIndex(i, j, id int) (int, int) {
	iNb := 0
	jNb := 0
	if id == 0 {
		iNb, jNb = i, j+1
	}
	if id == 1 {
		iNb, jNb = i-1, j+1
	}
	if id == 2 {
		iNb, jNb = i-1, j
	}
	if id == 3 {
		iNb, jNb = i-1, j-1
	}
	if id == 4 {
		iNb, jNb = i, j-1
	}
	if id == 5 {
		iNb, jNb = i+1, j-1
	}
	if id == 6 {
		iNb, jNb = i+1, j
	}
	if id == 7 {
		iNb, jNb = i+1, j+1
	}

	return iNb, jNb
}

// neighborIndexToID return a neighbor ID from its image coordinates
func neighborIndexToID(i0, j0, i, j int) int {
	di := i - i0
	dj := j - j0
	if di == 0 && dj == 1 {
		return 0
	}
	if di == -1 && dj == 1 {
		return 1
	}
	if di == -1 && dj == 0 {
		return 2
	}
	if di == -1 && dj == -1 {
		return 3
	}
	if di == 0 && dj == -1 {
		return 4
	}
	if di == 1 && dj == -1 {
		return 5
	}
	if di == 1 && dj == 0 {
		return 6
	}
	if di == 1 && dj == 1 {
		return 7
	}
	return -1
}

// firstClockwiseNonZeroNeighbor returns the first clockwise non-zero element in 8-neighborhood
func firstClockwiseNonZeroNeighbor(F mat.Dense, i0, j0, i, j, offset int) (int, int) {
	idNb := neighborIndexToID(i0, j0, i, j)
	for k := 1; k < NPixelNeighbors; k++ {
		kk := (idNb - k - offset + NPixelNeighbors*2) % NPixelNeighbors
		i_, j_ := neighborIDToIndex(i0, j0, kk)
		if F.At(i_, j_) != 0 {
			return i_, j_
		}
	}
	return 0, 0
}

// firstCounterClockwiseNonZeroNeighbor returns the first counter-clockwise non-zero element in 8-neighborhood
func firstCounterClockwiseNonZeroNeighbor(F mat.Dense, i0, j0, i, j, offset int) (int, int) {
	idNb := neighborIndexToID(i0, j0, i, j)
	for k := 1; k < NPixelNeighbors; k++ {
		kk := (idNb + k + offset + NPixelNeighbors*2) % NPixelNeighbors
		i_, j_ := neighborIDToIndex(i0, j0, kk)
		if F.At(i_, j_) != 0 {
			return i_, j_
		}
	}
	return 0, 0
}

//FindContourHierarchy finds contours in a binary image
// Implements Suzuki, S. and Abe, K.
// "Topological Structural Analysis of Digitized Binary Images by Border Following."
// See source code for step-by-step correspondence to the paper's algorithm
// description.
// @param  edges    The bitmap, stored in 1-dimensional row-major form.
//                  0=background, 1=foreground, will be modified by the function
//                  to hold semantic information
// @return          An array of contours found in the image.
func FindContourHierarchy(edges *mat.Dense) []*Contour {
	h, w := edges.Dims()
	nbd := 1
	lnbd := 1
	contours := make([]*Contour, 0)
	// edge cases; fill first and last row and col with 0, without loss of generality
	for i := 1; i < h-1; i++ {
		edges.Set(i, 0, 0)
		edges.Set(i, w-1, 0)
	}
	for j := 1; j < h-1; j++ {
		edges.Set(0, j, 0)
		edges.Set(h-1, j, 0)
	}
	//Scan the picture with a TV raster and perform the following steps
	//for each pixel such that fij # 0. Every time we begin to scan a
	//new row of the picture, reset LNBD to 1.
	for i := 1; i < h-1; i++ {
		lnbd = 1
		for j := 1; j < h-1; j++ {
			i2, j2 := 0, 0
			if edges.At(i, j) == 0 {
				continue
			}
			//(a) If fij = 1 and fi, j-1 = 0, then decide that the pixel
			//(i, j) is the border following starting point of an outer
			//border, increment NBD, and (i2, j2) <- (i, j - 1).
			if edges.At(i, j) == 1 && edges.At(i, j-1) == 0 {
				nbd += 1
				i2 = i
				j2 = j - 1
			} else if edges.At(i, j) >= 1 && edges.At(i, j+1) == 0 {
				//(b) Else if fij >= 1 and fi,j+1 = 0, then decide that the
				//pixel (i, j) is the border following starting point of a
				//hole border, increment NBD, (i2, j2) <- (i, j + 1), and
				//LNBD + fij in case fij > 1.
				nbd += 1
				i2 = i
				j2 = j + 1
				if edges.At(i, j) > 1 {
					lnbd = int(edges.At(i, j))
				}
			} else {
				//(c) Otherwise, go to (4).
				//(4) If fij != 1, then LNBD <- |fij| and resume the raster
				//scan from pixel (i,j+1). The algorithm terminates when the
				//scan reaches the lower right corner of the picture
				if edges.At(i, j) != 1 {
					lnbd = int(math.Abs(edges.At(i, j)))
				}
				continue
			}
			//(2) Depending on the types of the newly found border and the border with the sequential number LNBD
			//  (i.e., the last border met on the current row)
			// decide the parent of the current border as shown in Table 1.
			// TABLE 1
			// Decision Rule for the Parent Border of the Newly Found Border B
			// ----------------------------------------------------------------
			// Type of border B with the sequential number LNBD
			// Type of B \                Outer border         Hole border
			// ---------------------------------------------------------------
			// Outer border               The parent border    The border B'
			//                            of the border B'
			// Hole border                The border B'      The parent border
			//                                               of the border B'
			// ----------------------------------------------------------------
			B := NewContour()
			B.Points = make([]Point, 0)
			B.Points = append(B.Points, Point{float64(j), float64(i)})
			B.IsHole = j2 == j+1
			B.Id = nbd
			contours = append(contours, B)

			B0 := NewContour()
			for _, contour := range contours {
				if contour.Id == lnbd {
					B0 = contour
					break
				}
			}
			if B0.IsHole {
				if B.IsHole {
					B.ParentId = B0.ParentId
				} else {
					B.ParentId = lnbd
				}
			} else {
				if B.IsHole {
					B.ParentId = lnbd
				} else {
					B.ParentId = B0.ParentId
				}
			}
			//(3) From the starting point (i, j), follow the detected border:
			//this is done by the following sub-steps (3.1) through (3.5).

			//(3.1) Starting from (i2, j2), look around clockwise the pixels
			//in the neighborhood of (i, j) and find a nonzero pixel.
			//Let (i1, j1) be the first found nonzero pixel. If no nonzero
			//pixel is found, assign -NBD to fij and go to (4).
			i1, j1 := -1, -1
			i1, j1 = firstClockwiseNonZeroNeighbor(*edges, i, j, i2, j2, 0)

			if i1 == 0 && j1 == 0 {
				edges.Set(i, j, float64(-nbd))
				//go to (4)
				if edges.At(i, j) != 1 {
					lnbd = int(math.Abs(edges.At(i, j)))

				}
				continue
			}
			// (3.2) (i2, j2) <- (i1, j1) ad (i3,j3) <- (i, j).
			i2, j2 = i1, j1
			i3, j3 := i, j

			for true {
				//(3.3) Starting from the next element of the pixel (i2, j2) in the counter-clockwise order,
				// examine the pixels in the neighborhood of the current pixel (i3, j3)
				//to find a nonzero pixel and let the first one be (i4, j4).
				i4, j4 := firstCounterClockwiseNonZeroNeighbor(*edges, i3, j3, i2, j2, 1)
				contours[len(contours)-1].Points = append(contours[len(contours)-1].Points, Point{float64(j4), float64(i4)})

				//(a) If the pixel (i3, j3 + 1) is a O-pixel examined in the
				//substep (3.3) then fi3, j3 <-  -NBD.
				if edges.At(i3, j3+1) == 0 {
					edges.Set(i3, j3, float64(-nbd))

					//(b) If the pixel (i3, j3 + 1) is not a O-pixel examined
					//in the sub-step (3.3) and fi3,j3 = 1, then fi3,j3 <- NBD.
				} else if edges.At(i3, j3) == 1 {
					edges.Set(i3, j3, float64(nbd))
				} else {
					//(c) Otherwise, do not change fi3, j3.
				}

				//(3.5) If (i4,j4) = (i,j) and (i3,j3) = (i1,j1) (coming back to the starting point), then go to (4)
				if i4 == i && j4 == j && i3 == i1 && j3 == j1 {
					if edges.At(i, j) != 1 {
						lnbd = int(math.Abs(edges.At(i, j)))
					}
					break
				} else {
					//otherwise, (i2, j2) + (i3, j3),(i3, j3) + (i4, j4), and go back to (3.3)
					i2, j2 = i3, j3
					i3, j3 = i4, j4
				}
			}
		}
	}
	return contours
}

// DistanceToPoint returns the perpendicular distance of a point to the line.
func (l Line) DistanceToPoint(pt Point) float64 {
	a, b, c := l.Coefficients()
	return math.Abs(a*pt.X+b*pt.Y+c) / math.Sqrt(a*a+b*b)
}

// Coefficients returns the three coefficients that define a line.
// A line can represent by the following equation.
//
// ax + by + c = 0
//
func (l Line) Coefficients() (a, b, c float64) {
	a = l.Start.Y - l.End.Y
	b = l.End.X - l.Start.X
	c = l.Start.X*l.End.Y - l.End.X*l.Start.Y

	return a, b, c
}

// SimplifyPath accepts a list of points and epsilon as threshold, simplifies a path by dropping
// points that do not pass threshold values.
func SimplifyPath(points []Point, ep float64) []Point {
	if len(points) <= 2 {
		return points
	}

	l := Line{Start: points[0], End: points[len(points)-1]}

	idx, maxDist := seekMostDistantPoint(l, points)
	if maxDist >= ep {
		left := SimplifyPath(points[:idx+1], ep)
		right := SimplifyPath(points[idx:], ep)
		return append(left[:len(left)-1], right...)
	}

	// If the most distant point fails to pass the threshold test, then just return the two points
	return []Point{points[0], points[len(points)-1]}
}

// seekMostDistantPoint finds the point that is the most distant from the line defined by 2 points
func seekMostDistantPoint(l Line, points []Point) (idx int, maxDist float64) {
	for i := 0; i < len(points); i++ {
		d := l.DistanceToPoint(points[i])
		if d > maxDist {
			maxDist = d
			idx = i
		}
	}

	return idx, maxDist
}
