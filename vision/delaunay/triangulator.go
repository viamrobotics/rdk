package delaunay

import (
	"errors"
	"math"
	"sort"
)

type triangulator struct {
	points           []Point
	squaredDistances []float64
	ids              []int
	center           Point
	triangles        []int
	halfedges        []int
	trianglesLen     int
	hull             *node
	hash             []*node
}

func newTriangulator(points []Point) *triangulator {
	return &triangulator{points: points}
}

// sorting a triangulator sorts the `ids` such that the referenced points
// are in order by their distance to `center`

// Len computes length of points.
func (tri *triangulator) Len() int {
	return len(tri.points)
}

func (tri *triangulator) Swap(i, j int) {
	tri.ids[i], tri.ids[j] = tri.ids[j], tri.ids[i]
}

func (tri *triangulator) Less(i, j int) bool {
	d1 := tri.squaredDistances[tri.ids[i]]
	d2 := tri.squaredDistances[tri.ids[j]]
	if d1 != d2 {
		return d1 < d2
	}
	p1 := tri.points[tri.ids[i]]
	p2 := tri.points[tri.ids[j]]
	if p1.X != p2.X {
		return p1.X < p2.X
	}
	return p1.Y < p2.Y
}

func (tri *triangulator) triangulate() error {
	points := tri.points

	n := len(points)
	if n == 0 {
		return nil
	}

	tri.ids = make([]int, n)

	// compute bounds
	x0 := points[0].X
	y0 := points[0].Y
	x1 := points[0].X
	y1 := points[0].Y
	for i, p := range points {
		if p.X < x0 {
			x0 = p.X
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.Y > y1 {
			y1 = p.Y
		}
		tri.ids[i] = i
	}

	var i0, i1, i2 int

	// pick a seed point close to midpoint
	m := Point{(x0 + x1) / 2, (y0 + y1) / 2}
	minDist := infinity
	for i, p := range points {
		d := p.squaredDistance(m)
		if d < minDist {
			i0 = i
			minDist = d
		}
	}

	// find point closest to seed point
	minDist = infinity
	for i, p := range points {
		if i == i0 {
			continue
		}
		d := p.squaredDistance(points[i0])
		if d > 0 && d < minDist {
			i1 = i
			minDist = d
		}
	}

	// find the third point which forms the smallest circumcircle
	minRadius := infinity
	for i, p := range points {
		if i == i0 || i == i1 {
			continue
		}
		r := circumRadius(points[i0], points[i1], p)
		if r < minRadius {
			i2 = i
			minRadius = r
		}
	}
	if minRadius == infinity {
		return errors.New("no Delaunay triangulation exists for this input")
	}

	// swap the order of the seed points for counter-clockwise orientation
	if area(points[i0], points[i1], points[i2]) < 0 {
		i1, i2 = i2, i1
	}

	tri.center = circumcenter(points[i0], points[i1], points[i2])

	// sort the points by distance from the seed triangle circumcenter
	tri.squaredDistances = make([]float64, n)
	for i, p := range points {
		tri.squaredDistances[i] = p.squaredDistance(tri.center)
	}
	sort.Sort(tri)

	// initialize a hash table for storing edges of the advancing convex hull
	hashSize := int(math.Ceil(math.Sqrt(float64(n))))
	tri.hash = make([]*node, hashSize)

	// initialize a circular doubly-linked list that will hold an advancing convex hull
	nodes := make([]node, n)

	e := newNode(nodes, i0, nil)
	e.t = 0
	tri.hashEdge(e)

	e = newNode(nodes, i1, e)
	e.t = 1
	tri.hashEdge(e)

	e = newNode(nodes, i2, e)
	e.t = 2
	tri.hashEdge(e)

	tri.hull = e

	maxTriangles := 2*n - 5
	tri.triangles = make([]int, maxTriangles*3)
	tri.halfedges = make([]int, maxTriangles*3)

	tri.addTriangle(i0, i1, i2, -1, -1, -1)

	pp := Point{infinity, infinity}
	for k := 0; k < n; k++ {
		i := tri.ids[k]
		p := points[i]

		// skip nearly-duplicate points
		if p.squaredDistance(pp) < eps {
			continue
		}
		pp = p

		// skip seed triangle points
		if i == i0 || i == i1 || i == i2 {
			continue
		}

		// find a visible edge on the convex hull using edge hash
		var start *node
		key := tri.hashKey(p)
		for j := 0; j < len(tri.hash); j++ {
			start = tri.hash[key]
			if start != nil && start.i >= 0 {
				break
			}
			key++
			if key >= len(tri.hash) {
				key = 0
			}
		}
		start = start.prev

		e := start
		for area(p, points[e.i], points[e.next.i]) >= 0 {
			e = e.next
			if e == start {
				e = nil
				break
			}
		}
		if e == nil {
			// likely a near-duplicate point; skip it
			continue
		}
		walkBack := e == start

		// add the first triangle from the point
		t := tri.addTriangle(e.i, i, e.next.i, -1, -1, e.t)
		e.t = t // keep track of boundary triangles on the hull
		e = newNode(nodes, i, e)

		// recursively flip triangles from the point until they satisfy the Delaunay condition
		e.t = tri.legalize(t + 2)

		// walk forward through the hull, adding more triangles and flipping recursively
		q := e.next
		for area(p, points[q.i], points[q.next.i]) < 0 {
			t = tri.addTriangle(q.i, i, q.next.i, q.prev.t, -1, q.t)
			q.prev.t = tri.legalize(t + 2)
			tri.hull = q.remove()
			q = q.next
		}

		if walkBack {
			// walk backward from the other side, adding more triangles and flipping
			q := e.prev
			for area(p, points[q.prev.i], points[q.i]) < 0 {
				t = tri.addTriangle(q.prev.i, i, q.i, -1, q.t, q.prev.t)
				tri.legalize(t + 2)
				q.prev.t = t
				tri.hull = q.remove()
				q = q.prev
			}
		}

		// save the two new edges in the hash table
		tri.hashEdge(e)
		tri.hashEdge(e.prev)
	}

	tri.triangles = tri.triangles[:tri.trianglesLen]
	tri.halfedges = tri.halfedges[:tri.trianglesLen]

	return nil
}

func (tri *triangulator) hashKey(point Point) int {
	d := point.sub(tri.center)
	return int(pseudoAngle(d.X, d.Y) * float64(len(tri.hash)))
}

func (tri *triangulator) hashEdge(e *node) {
	tri.hash[tri.hashKey(tri.points[e.i])] = e
}

// addTriangle add a triangle to the triangulation.
func (tri *triangulator) addTriangle(i0, i1, i2, a, b, c int) int {
	i := tri.trianglesLen
	tri.triangles[i] = i0
	tri.triangles[i+1] = i1
	tri.triangles[i+2] = i2
	tri.link(i, a)
	tri.link(i+1, b)
	tri.link(i+2, c)
	tri.trianglesLen += 3
	return i
}

func (tri *triangulator) link(a, b int) {
	tri.halfedges[a] = b
	if b >= 0 {
		tri.halfedges[b] = a
	}
}

func (tri *triangulator) legalize(a int) int {
	// if the pair of triangles doesn'tri satisfy the Delaunay condition
	// (p1 is inside the circumcircle of [p0, pl, pr]), flip them,
	// then do the same check/flip recursively for the new pair of triangles
	//
	//           pl                    pl
	//          /||\                  /  \
	//       al/ || \bl            al/    \a
	//        /  ||  \              /      \
	//       /  a||b  \    flip    /___ar___\
	//     p0\   ||   /p1   =>   p0\---bl---/p1
	//        \  ||  /              \      /
	//       ar\ || /br             b\    /br
	//          \||/                  \  /
	//           pr                    pr

	b := tri.halfedges[a]

	a0 := a - a%3
	b0 := b - b%3

	al := a0 + (a+1)%3
	ar := a0 + (a+2)%3
	bl := b0 + (b+2)%3

	if b < 0 {
		return ar
	}

	p0 := tri.triangles[ar]
	pr := tri.triangles[a]
	pl := tri.triangles[al]
	p1 := tri.triangles[bl]

	illegal := inCircle(tri.points[p0], tri.points[pr], tri.points[pl], tri.points[p1])

	if illegal {
		tri.triangles[a] = p1
		tri.triangles[b] = p0

		// edge swapped on the other side of the hull (rare)
		// fix the halfedge reference
		if tri.halfedges[bl] == -1 {
			e := tri.hull
			for {
				if e.t == bl {
					e.t = a
					break
				}
				e = e.next
				if e == tri.hull {
					break
				}
			}
		}

		tri.link(a, tri.halfedges[bl])
		tri.link(b, tri.halfedges[ar])
		tri.link(ar, bl)

		br := b0 + (b+1)%3

		tri.legalize(a)
		return tri.legalize(br)
	}

	return ar
}

func (tri *triangulator) convexHull() []Point {
	var result []Point
	e := tri.hull
	for e != nil {
		result = append(result, tri.points[e.i])
		e = e.prev
		if e == tri.hull {
			break
		}
	}
	return result
}
