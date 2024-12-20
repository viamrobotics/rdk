package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestTriangleIntersectsPlane(t *testing.T) {
	tests := []struct {
		name        string
		triangle    *Triangle
		planePt     r3.Vector
		planeNormal r3.Vector
		want        bool
	}{
		{
			name: "triangle intersects plane",
			triangle: NewTriangle(
				r3.Vector{X: -1, Y: 0, Z: -1},
				r3.Vector{X: 1, Y: 0, Z: -1},
				r3.Vector{X: 0, Y: 0, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			want:        true,
		},
		{
			name: "triangle above plane",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 1},
				r3.Vector{X: 1, Y: 0, Z: 1},
				r3.Vector{X: 0, Y: 1, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			want:        false,
		},
		{
			name: "triangle below plane",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: -1},
				r3.Vector{X: 1, Y: 0, Z: -1},
				r3.Vector{X: 0, Y: 1, Z: -1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			want:        false,
		},
		{
			name: "triangle lies in plane",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			want:        true,
		},
		{
			name: "triangle touches plane at vertex",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 1},
				r3.Vector{X: 0, Y: 1, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.triangle.IntersectsPlane(tt.planePt, tt.planeNormal)
			test.That(t, tt.want, test.ShouldEqual, got)
		})
	}
}

func TestTrianglePlaneIntersectingSegment(t *testing.T) {
	tests := []struct {
		name        string
		triangle    *Triangle
		planePt     r3.Vector
		planeNormal r3.Vector
		wantP1      r3.Vector
		wantP2      r3.Vector
		wantExists  bool
	}{
		{
			name: "triangle intersects plane - simple case",
			triangle: NewTriangle(
				r3.Vector{X: -1, Y: 0, Z: -1},
				r3.Vector{X: 1, Y: 0, Z: -1},
				r3.Vector{X: 0, Y: 0, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			wantP1:      r3.Vector{X: 0.5, Y: 0, Z: 0},
			wantP2:      r3.Vector{X: -0.5, Y: 0, Z: 0},
			wantExists:  true,
		},
		{
			name: "triangle lies in plane",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			wantP1:      r3.Vector{X: 1, Y: 0, Z: 0},
			wantP2:      r3.Vector{X: 0, Y: 1, Z: 0},
			wantExists:  true,
		},
		{
			name: "no intersection",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 1},
				r3.Vector{X: 1, Y: 0, Z: 1},
				r3.Vector{X: 0, Y: 1, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			wantP1:      r3.Vector{},
			wantP2:      r3.Vector{},
			wantExists:  false,
		},
		{
			name: "triangle touches plane at vertex",
			triangle: NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 1},
				r3.Vector{X: 0, Y: 1, Z: 1},
			),
			planePt:     r3.Vector{X: 0, Y: 0, Z: 0},
			planeNormal: r3.Vector{X: 0, Y: 0, Z: 1},
			wantP1:      r3.Vector{X: 0, Y: 0, Z: 0},
			wantP2:      r3.Vector{X: 0, Y: 0, Z: 0},
			wantExists:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotP1, gotP2, gotExists := tt.triangle.TrianglePlaneIntersectingSegment(tt.planePt, tt.planeNormal)
			test.That(t, tt.wantExists, test.ShouldEqual, gotExists)
			if tt.wantExists {
				test.That(t, tt.wantP1.ApproxEqual(gotP1), test.ShouldBeTrue)
				test.That(t, tt.wantP2.ApproxEqual(gotP2), test.ShouldBeTrue)
			}
		})
	}
}
