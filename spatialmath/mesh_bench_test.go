package spatialmath

import (
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
)

func makeGridTriangles(n int, spacing float64) []*Triangle {
	tris := make([]*Triangle, 0, 2*n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x0 := float64(i) * spacing
			y0 := float64(j) * spacing
			x1 := float64(i+1) * spacing
			y1 := float64(j+1) * spacing

			p00 := r3.Vector{X: x0, Y: y0, Z: 0}
			p10 := r3.Vector{X: x1, Y: y0, Z: 0}
			p01 := r3.Vector{X: x0, Y: y1, Z: 0}
			p11 := r3.Vector{X: x1, Y: y1, Z: 0}

			tris = append(tris, NewTriangle(p00, p10, p11))
			tris = append(tris, NewTriangle(p00, p11, p01))
		}
	}
	return tris
}

func makeGridMesh(n int, spacing float64, pose Pose, label string) *Mesh {
	return NewMesh(pose, makeGridTriangles(n, spacing), label)
}

func BenchmarkMeshCollisions(b *testing.B) {
	sizes := []int{10, 30, 60}
	spacing := 1.0
	buffer := defaultCollisionBufferMM

	for _, n := range sizes {
		n := n
		b.Run(fmt.Sprintf("mesh-mesh-collide-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, NewZeroPose(), "m1")
			m2 := makeGridMesh(n, spacing, NewZeroPose(), "m2")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := m1.CollidesWith(m2, buffer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("mesh-mesh-separated-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, NewZeroPose(), "m1")
			pose2 := NewPose(r3.Vector{X: 0, Y: 0, Z: float64(n+10) * spacing}, NewZeroOrientation())
			m2 := makeGridMesh(n, spacing, pose2, "m2")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := m1.CollidesWith(m2, buffer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("mesh-sphere-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, NewZeroPose(), "m1")
			s, err := NewSphere(NewPoseFromPoint(r3.Vector{X: float64(n) * spacing * 0.5, Y: float64(n) * spacing * 0.5, Z: spacing}), spacing, "s")
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := m1.CollidesWith(s, buffer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
