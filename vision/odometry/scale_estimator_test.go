package odometry

import (
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestGetTriangleNormalVector(t *testing.T) {
	points := []r3.Vector{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}}
	normal := getTriangleNormalVector(points)
	test.That(t, normal.X, test.ShouldEqual, 0)
	test.That(t, normal.Y, test.ShouldEqual, 0)
	test.That(t, normal.Z, test.ShouldEqual, -1)
	fmt.Println(normal)
}
