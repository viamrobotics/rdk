package spatialmath

import (
	"math/rand"
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

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

func TestTriangleDistsEqual(t *testing.T) {
	
	p0 := r3.Vector{0,0,0}
	p1 := r3.Vector{1,0,0}
	p2 := r3.Vector{0,1,0}
	s1 := r3.Vector{0.2,0.2,1}
	s2 := r3.Vector{0.3,0.3,2}
	
	tri := newTriangle(p0, p1, p2)
	
	segPt, coplanar := closestPointsSegmentPlane(s1, s2, tri.p0, tri.normal)
	pt1 := tri.closestPointToPoint(segPt)
	pt2 := tri.closestPointToCoplanarPoint(coplanar)
	
	fmt.Println("seg, co", segPt, coplanar)
	fmt.Println("p1, p2", pt1, pt2)
	fmt.Println("eq",  R3VectorAlmostEqual(pt1, pt2, 1e-4))
	test.That(t, R3VectorAlmostEqual(pt1, pt2, 1e-4), test.ShouldBeTrue)
	

	//~ r := rand.New(rand.NewSource(1))
	//~ for n := 0; n < 10000; n++ {
		//~ p0 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		//~ p1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		//~ p2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		//~ s1 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		//~ s2 := r3.Vector{r.Float64(), r.Float64(), r.Float64()}
		
		//~ tri := newTriangle(p0, p1, p2)
		
		//~ segPt, coplanar := closestPointsSegmentPlane(s1, s2, tri.p0, tri.normal)
		//~ pt1 := tri.closestPointToPoint(segPt)
		//~ pt2 := tri.closestPointToCoplanarPoint(coplanar)
		//~ fmt.Println(n)
		//~ fmt.Println(segPt, coplanar)
		//~ test.That(t, R3VectorAlmostEqual(pt1, pt2, 1e-4), test.ShouldBeTrue)
	//~ }
}
