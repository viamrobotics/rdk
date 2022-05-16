// Package delaunay implements 2d Delaunay triangulation
package delaunay

import (
	"math"


	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

// Triangulation stores the points, convex hull, triangles and half edges from a Delaunay triangulation.
type Triangulation struct {
	Points     []Point
	ConvexHull []Point
	Triangles  []int
	Halfedges  []int
}

// Triangulate returns a Delaunay triangulation of the provided points.
func Triangulate(points []Point) (*Triangulation, error) {
	t := newTriangulator(points)
	err := t.triangulate()
	return &Triangulation{points, t.convexHull(), t.triangles, t.halfedges}, err
}

func (t *Triangulation) area() float64 {
	var result float64
	points := t.Points
	ts := t.Triangles
	for i := 0; i < len(ts); i += 3 {
		p0 := points[ts[i+0]]
		p1 := points[ts[i+1]]
		p2 := points[ts[i+2]]
		result += area(p0, p1, p2)
	}
	return result / 2
}

// Validate performs several sanity checks on the Triangulation to check for
// potential errors. Returns nil if no issues were found. You normally
// shouldn't need to call this function but it can be useful for debugging.
func (t *Triangulation) Validate() error {
	// verify halfedges
	for i1, i2 := range t.Halfedges {
		if i1 != -1 && t.Halfedges[i1] != i2 {
			return errors.New("invalid halfedge connection")
		}
		if i2 != -1 && t.Halfedges[i2] != i1 {
			return errors.New("invalid halfedge connection")
		}
	}

	// verify convex hull area vs sum of triangle areas
	hull1 := t.ConvexHull
	hull2 := ConvexHull(t.Points)
	area1 := polygonArea(hull1)
	area2 := polygonArea(hull2)
	area3 := t.area()
	if math.Abs(area1-area2) > 1e-9 || math.Abs(area1-area3) > 1e-9 {
		return errors.New("hull areas disagree")
	}

	// verify convex hull perimeter
	perimeter1 := polygonPerimeter(hull1)
	perimeter2 := polygonPerimeter(hull2)
	if math.Abs(perimeter1-perimeter2) > 1e-9 {
		return errors.New("hull perimeters disagree")
	}

	return nil
}

// GetTrianglesPointsMap returns a map that had triangle ID as key, and points IDs as value.
func (t *Triangulation) GetTrianglesPointsMap() map[int][]int {
	triangles := make(map[int][]int)
	ts := t.Triangles
	currentTriangleID := 0
	for i := 0; i < len(t.Triangles); i += 3 {
		id0 := ts[i]
		id1 := ts[i+1]
		id2 := ts[i+2]
		triangles[currentTriangleID] = []int{id0, id1, id2}
		// update current triangle ID
		currentTriangleID++
	}
	return triangles
}

// GetTriangles returns a slice that has triangle ID as index, and points IDs as value.
func (t *Triangulation) GetTriangles(pts3d []r3.Vector) [][]r3.Vector {
	triangles := make([][]r3.Vector, 0, len(t.Triangles))
	ts := t.Triangles
	currentTriangleID := 0
	for i := 0; i < len(t.Triangles); i += 3 {
		id0 := ts[i]
		id1 := ts[i+1]
		id2 := ts[i+2]
		pt0 := pts3d[id0]
		pt1 := pts3d[id1]
		pt2 := pts3d[id2]
		currentTriangle := []r3.Vector{pt0, pt1, pt2}
		triangles = append(triangles, currentTriangle)
		// update current triangle ID
		currentTriangleID++
	}
	return triangles
}
