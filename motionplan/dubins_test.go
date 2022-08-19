package motionplan

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestFindCenter(t *testing.T) {
	start := []float64{0, 0, 0}
	end := []float64{4, 4, math.Pi}

	// constants
	centerLeftStart := []float64{0.0, 1.0}
	centerRightStart := []float64{0.0, -1.0}

	centerLeftEnd := []float64{4.0, 3.0}
	centerRightEnd := []float64{4.0, 5.0}

	epsilon := 0.00001

	radius := 1.0
	pointSep := 1.0

	d := &Dubins{Radius: radius, PointSeparation: pointSep}

	x := d.findCenter(start, true) // Left Start
	test.That(t, math.Abs(x[0]-centerLeftStart[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(x[1]-centerLeftStart[1]), test.ShouldBeLessThan, epsilon)

	x = d.findCenter(start, false) // Right Start
	test.That(t, math.Abs(x[0]-centerRightStart[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(x[1]-centerRightStart[1]), test.ShouldBeLessThan, epsilon)

	x = d.findCenter(end, true) // Left End
	test.That(t, math.Abs(x[0]-centerLeftEnd[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(x[1]-centerLeftEnd[1]), test.ShouldBeLessThan, epsilon)

	x = d.findCenter(end, false) // Right End
	test.That(t, math.Abs(x[0]-centerRightEnd[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(x[1]-centerRightEnd[1]), test.ShouldBeLessThan, epsilon)
}

func TestAllPaths(t *testing.T) {
	// no movement
	start := []float64{0, 0, 0}
	end := []float64{0, 0, 0}

	radius := 1.0
	pointSep := 1.0

	d := &Dubins{Radius: radius, PointSeparation: pointSep}
	paths := d.AllPaths(start, end, true)[0] // get shortest paths

	test.That(t, paths.TotalLen, test.ShouldEqual, 0.0)
	test.That(t, paths.DubinsPath[0], test.ShouldEqual, 0.0)
	test.That(t, paths.DubinsPath[1], test.ShouldEqual, 0.0)
	test.That(t, paths.DubinsPath[2], test.ShouldEqual, 0.0)

	// test shortest path with movement
	start = []float64{0, 0, 0}
	end = []float64{4, 4, math.Pi}

	epsilon := 0.00001

	paths = d.AllPaths(start, end, true)[0] // get shortest paths
	TotalLen := 7.61372
	dubinsPath := []float64{0.4636476090008061, 2.677945044588987, 4.47213595499958}

	test.That(t, math.Abs(paths.TotalLen-TotalLen), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(paths.DubinsPath[0]-dubinsPath[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(paths.DubinsPath[1]-dubinsPath[1]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(paths.DubinsPath[2]-dubinsPath[2]), test.ShouldBeLessThan, epsilon)
	test.That(t, paths.Straight, test.ShouldBeTrue)

	// test individual dubin's paths function math with no sorting
	allPaths := d.AllPaths(start, end, false)
	allLengths := []float64{7.613728608589373, 16.63588051169736, 13.86821850391708, 10.726625850327286}

	for i, val := range allLengths {
		test.That(t, math.Abs(allPaths[i].TotalLen-val), test.ShouldBeLessThan, epsilon)
	}

	test.That(t, allPaths[4].TotalLen, test.ShouldEqual, math.Inf(1))
	test.That(t, allPaths[5].TotalLen, test.ShouldEqual, math.Inf(1))
}

func TestGeneratePoints(t *testing.T) {
	// straight movement points
	start := []float64{0, 0, 0}
	end := []float64{1, 0, 0}

	radius := 1.0
	pointSep := 20.0

	epsilon := 0.0001

	d := &Dubins{Radius: radius, PointSeparation: pointSep}
	paths := d.AllPaths(start, end, true)[0] // get shortest paths
	points := d.generatePoints(start, end, paths.DubinsPath, true)
	test.That(t, len(points), test.ShouldEqual, 2)
	test.That(t, math.Abs(points[0][0]-start[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[0][1]-start[1]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[1][0]-end[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[1][1]-end[1]), test.ShouldBeLessThan, epsilon)

	// curved movement points
	end = []float64{1, 1, math.Pi / 4.0}
	paths = d.AllPaths(start, end, true)[0] // get shortest paths
	points = d.generatePoints(start, end, paths.DubinsPath, false)

	test.That(t, len(points), test.ShouldEqual, 2)
	test.That(t, math.Abs(points[0][0]-start[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[0][1]-start[1]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[1][0]-end[0]), test.ShouldBeLessThan, epsilon)
	test.That(t, math.Abs(points[1][1]-end[1]), test.ShouldBeLessThan, epsilon)
}
