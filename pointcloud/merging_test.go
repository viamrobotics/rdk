package pointcloud

import (
	"context"
	"image/color"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestApplyOffset(t *testing.T) {
	pc1 := NewBasicPointCloud(3)
	err := pc1.Set(NewVector(1, 0, 0), NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = pc1.Set(NewVector(1, 1, 0), NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = pc1.Set(NewVector(1, 1, 1), NewColoredData(color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	// apply a simple translation
	transPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 99, 0})
	transPc := NewBasicPointCloud(0)
	err = ApplyOffset(context.Background(), pc1, transPose, transPc)
	test.That(t, err, test.ShouldBeNil)
	correctCount := 0
	transPc.Iterate(0, 0, func(p r3.Vector, d Data) bool { // check if all points transformed as expected
		r, g, b := d.RGB255()
		if r == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{1, 99, 0}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		if g == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{1, 100, 0}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		if b == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{1, 100, 1}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		return true
	})
	test.That(t, correctCount, test.ShouldEqual, 3)
	// apply a translation and rotation
	transrotPose := spatialmath.NewPose(r3.Vector{0, 99, 0}, &spatialmath.R4AA{math.Pi / 2., 0., 0., 1.})
	transrotPc := NewBasicPointCloud(0)
	err = ApplyOffset(context.Background(), pc1, transrotPose, transrotPc)
	test.That(t, err, test.ShouldBeNil)
	correctCount = 0
	transrotPc.Iterate(0, 0, func(p r3.Vector, d Data) bool { // check if all points transformed as expected
		r, g, b := d.RGB255()
		if r == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{0, 100, 0}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		if g == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{-1, 100, 0}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		if b == 255 {
			correctPoint := spatialmath.NewPoint(r3.Vector{-1, 100, 1}, "")
			test.That(t, correctPoint.Pose().Point().X, test.ShouldAlmostEqual, p.X)
			test.That(t, correctPoint.Pose().Point().Y, test.ShouldAlmostEqual, p.Y)
			test.That(t, correctPoint.Pose().Point().Z, test.ShouldAlmostEqual, p.Z)
			correctCount++
		}
		return true
	})
	test.That(t, correctCount, test.ShouldEqual, 3)
}

func TestMergePointsWithColor(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud := NewBasicPointCloud(0)
	err := MergePointCloudsWithColor(clouds, mergedCloud)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.Size(), test.ShouldResemble, 9)

	a, got := mergedCloud.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	b, got := mergedCloud.At(0, 0, 1)
	test.That(t, got, test.ShouldBeTrue)

	c, got := mergedCloud.At(30, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	test.That(t, a.Color(), test.ShouldResemble, b.Color())
	test.That(t, a.Color(), test.ShouldNotResemble, c.Color())
}

func BenchmarkApplyOffset(b *testing.B) {
	in := newBigPC()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := NewBasicPointCloud(0)
		transPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 99, 0})
		err := ApplyOffset(context.Background(), in, transPose, out)
		test.That(b, err, test.ShouldBeNil)
		test.That(b, out.Size(), test.ShouldEqual, in.Size())
	}
}
