package motionplan

import (
	"fmt"
	"testing"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/pointcloud"
	spatial "go.viam.com/rdk/spatialmath"
)

func makeGridTriangles(n int, spacing float64) []*spatial.Triangle {
	tris := make([]*spatial.Triangle, 0, 2*n*n)
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

			tris = append(tris, spatial.NewTriangle(p00, p10, p11))
			tris = append(tris, spatial.NewTriangle(p00, p11, p01))
		}
	}
	return tris
}

func makeGridMesh(n int, spacing float64, pose spatial.Pose, label string) *spatial.Mesh {
	return spatial.NewMesh(pose, makeGridTriangles(n, spacing), label)
}

func BenchmarkMeshCollisionPerformance(b *testing.B) {
	sizes := []int{10, 30, 60}
	spacing := 1.0
	buffer := 1e-8

	for _, n := range sizes {
		n := n
		b.Run(fmt.Sprintf("mesh-mesh-collide-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, spatial.NewZeroPose(), "m1")
			m2 := makeGridMesh(n, spacing, spatial.NewZeroPose(), "m2")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := m1.CollidesWith(m2, buffer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("octree-mesh-collide-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, spatial.NewZeroPose(), "m1")
			oct, err := pointcloud.NewFromMesh(m1)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := oct.CollidesWith(m1, buffer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("mesh-to-octree-build-%dx%d", n, n), func(b *testing.B) {
			m1 := makeGridMesh(n, spacing, spatial.NewZeroPose(), "m1")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := pointcloud.NewFromMesh(m1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
