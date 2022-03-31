package delaunay

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/test"
)

// uniform samples points from a uniform 2D distribution.
func uniform(n int, rnd *rand.Rand) []Point {
	points := make([]Point, n)
	for i := range points {
		x := rnd.Float64()
		y := rnd.Float64()
		points[i] = Point{x, y}
	}
	return points
}

// normal samples 2d points from a 2d normal distribution.
func normal(n int, rnd *rand.Rand) []Point {
	points := make([]Point, n)
	for i := range points {
		x := rnd.NormFloat64()
		y := rnd.NormFloat64()
		points[i] = Point{x, y}
	}
	return points
}

// grid samples 2d points on a 2d grid.
func grid(n int) []Point {
	side := int(math.Floor(math.Sqrt(float64(n))))
	n = side * side
	points := make([]Point, 0, n)
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			p := Point{float64(x), float64(y)}
			points = append(points, p)
		}
	}
	return points
}

// circle samples 2d point on a circle.
func circle(n int) []Point {
	points := make([]Point, n)
	for i := range points {
		t := float64(i) / float64(n)
		x := math.Cos(t)
		y := math.Sin(t)
		points[i] = Point{x, y}
	}
	return points
}

func TestConvexHull(t *testing.T) {
	// test square + point at the centroid
	points := []Point{{0, 0}, {1, 1}, {0, 1}, {1, 0}, {0.5, 0.5}}
	hull := ConvexHull(points)
	test.That(t, hull, test.ShouldNotBeNil)
	test.That(t, len(hull), test.ShouldEqual, 4)
	test.That(t, hull[0], test.ShouldResemble, Point{0, 0})
	test.That(t, hull[1], test.ShouldResemble, Point{1, 0})
	test.That(t, hull[2], test.ShouldResemble, Point{1, 1})
	test.That(t, hull[3], test.ShouldResemble, Point{0, 1})
	// test segment
	points2 := []Point{{0, 0}, {1, 1}}
	hull2 := ConvexHull(points2)
	test.That(t, len(hull2), test.ShouldEqual, len(points2))
	test.That(t, hull2[0], test.ShouldResemble, points2[0])
	test.That(t, hull2[1], test.ShouldResemble, points2[1])
	// test one point: empty convex hull
	points1 := []Point{{1, 1}}
	hull1 := ConvexHull(points1)
	test.That(t, len(hull1), test.ShouldEqual, 0)
}

func TestTricky(t *testing.T) {
	var points []Point
	rnd := rand.New(rand.NewSource(99))
	for len(points) < 100000 {
		x := rnd.NormFloat64() * 0.5
		y := rnd.NormFloat64() * 0.5
		points = append(points, Point{x, y})
		nx := rnd.Intn(4)
		for i := 0; i < nx; i++ {
			x = math.Nextafter(x, x+1)
		}
		ny := rnd.Intn(4)
		for i := 0; i < ny; i++ {
			y = math.Nextafter(y, y+1)
		}
		points = append(points, Point{x, y})
	}
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri.Points), test.ShouldEqual, 100000)
	test.That(t, len(tri.Triangles), test.ShouldEqual, 299958)
}

func TestCases(t *testing.T) {
	// test one point
	tri1, err := Triangulate([]Point{{0, 0}})

	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(tri1.Points), test.ShouldEqual, 1)
	test.That(t, len(tri1.Triangles), test.ShouldEqual, 0)

	// test 3 times same point
	tri2, err := Triangulate([]Point{{0, 0}, {0, 0}, {0, 0}})

	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(tri2.Points), test.ShouldEqual, 3)
	test.That(t, len(tri1.Triangles), test.ShouldEqual, 0)

	// test 2 points
	tri3, err := Triangulate([]Point{{0, 0}, {1, 0}})

	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(tri3.Points), test.ShouldEqual, 2)
	test.That(t, len(tri1.Triangles), test.ShouldEqual, 0)

	// test collinear points
	tri4, err := Triangulate([]Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}})

	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(tri4.Points), test.ShouldEqual, 4)
	test.That(t, len(tri1.Triangles), test.ShouldEqual, 0)

	// test should work
	points := []Point{
		{516, 661},
		{369, 793},
		{426, 539},
		{273, 525},
		{204, 694},
		{747, 750},
		{454, 390},
	}
	tri5, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri5.Points), test.ShouldEqual, 7)
	test.That(t, len(tri5.Triangles), test.ShouldEqual, 21)
}

func TestUniform(t *testing.T) {
	rnd := rand.New(rand.NewSource(99))
	points := uniform(100000, rnd)
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri.Points), test.ShouldEqual, 100000)
	test.That(t, len(tri.Triangles), test.ShouldEqual, 599910)
	test.That(t, len(tri.ConvexHull), test.ShouldEqual, 28)
}

func TestNormal(t *testing.T) {
	rnd := rand.New(rand.NewSource(99))
	points := normal(100000, rnd)
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri.Points), test.ShouldEqual, 100000)
	test.That(t, len(tri.Triangles), test.ShouldEqual, 599943)
	test.That(t, len(tri.ConvexHull), test.ShouldEqual, 17)
}

func TestGrid(t *testing.T) {
	points := grid(100000)
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri.Points), test.ShouldEqual, 99856)
	test.That(t, len(tri.Triangles), test.ShouldEqual, 595350)
	test.That(t, len(tri.ConvexHull), test.ShouldEqual, 1260)

	// test that parallelograms on a grid have an area of 1
	ts := tri.Triangles
	for i := 0; i < len(ts); i += 3 {
		p0 := points[ts[i+0]]
		p1 := points[ts[i+1]]
		p2 := points[ts[i+2]]
		a := area(p0, p1, p2)
		test.That(t, a, test.ShouldEqual, 1)
	}
}

func TestCircle(t *testing.T) {
	points := circle(10000)
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tri.Points), test.ShouldEqual, 10000)
	test.That(t, len(tri.ConvexHull), test.ShouldEqual, 10000)
}

func TestGetTriangleIdsMap(t *testing.T) {
	points := circle(10000)
	tri, err := Triangulate(points)
	test.That(t, err, test.ShouldBeNil)
	triangles := tri.Triangles
	test.That(t, len(triangles)%3, test.ShouldEqual, 0)
	idMap := tri.GetTranglesPointsMap()
	// test number of triangles
	test.That(t, len(idMap), test.ShouldEqual, len(triangles)/3)
	// test that every triangle has 3 and only 3 points
	for _, v := range idMap {
		test.That(t, len(v), test.ShouldEqual, 3)
	}
}

func BenchmarkUniform(b *testing.B) {
	rnd := rand.New(rand.NewSource(99))
	points := uniform(b.N, rnd)
	Triangulate(points)
}

func BenchmarkNormal(b *testing.B) {
	rnd := rand.New(rand.NewSource(99))
	points := normal(b.N, rnd)
	Triangulate(points)
}

func BenchmarkGrid(b *testing.B) {
	points := grid(b.N)
	Triangulate(points)
}
