package spatialmath

import (
	"math/rand"
	"testing"
	"fmt"

	"github.com/golang/geo/r3"
	//~ "go.viam.com/test"
)

func makeTestCapsule(o Orientation, pt r3.Vector, radius, length float64, label string) Geometry {
	c, _ := NewCapsule(NewPose(pt, o), radius, length, label)
	return c
}

var dist = 0.
var pt = r3.Vector{}

func BenchmarkLineDist1(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < b.N; n++ {
		p1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p3 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		dist = DistToLineSegment(p1, p2, p3)
	}
}

func BenchmarkLineDist2(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < b.N; n++ {
		p1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p3 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		pt = ClosestPointSegmentPoint(p1, p2, p3)
		
	}
}

func BenchmarkTriangleDist1(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < b.N; n++ {
		p0 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		s1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		s2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		
		tri := newTriangle(p0, p1, p2)
		
		segPt4, _ := closestPointsSegmentPlane(s1, s2, tri.p0, tri.normal)
		pt = tri.closestPointToPoint(segPt4)
	}
}

func BenchmarkTriangleDist2(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < b.N; n++ {
		p0 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		p2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		s1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		s2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		
		tri := newTriangle(p0, p1, p2)
		
		_, coplanar := closestPointsSegmentPlane(s1, s2, tri.p0, tri.normal)
		pt = tri.closestPointToCoplanarPoint(coplanar)
	}
}

func TestTriangleDists(t *testing.T) {
	
	p0 := r3.Vector{0,0,0}
	p1 := r3.Vector{3,0,0}
	p2 := r3.Vector{0,3,0}
	test1 := r3.Vector{-1,-1,0}
	
	tri := newTriangle(p0, p1, p2)
	//~ close1 := tri.closestPointToPoint(test1)
	close2, inside := tri.closestInsidePoint(test1)
	//~ test.That(t, R3VectorAlmostEqual(close1, p0, 1e-4), test.ShouldBeTrue)
	fmt.Println(close2, inside)
}
