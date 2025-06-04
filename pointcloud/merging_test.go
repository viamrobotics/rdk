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
	err = ApplyOffset(pc1, transPose, transPc)
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
	err = ApplyOffset(pc1, transrotPose, transrotPc)
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
		err := ApplyOffset(in, transPose, out)
		test.That(b, err, test.ShouldBeNil)
		test.That(b, out.Size(), test.ShouldEqual, in.Size())
	}
}

func makeThreeCloudsWithOffsets(t *testing.T) []CloudAndOffsetFunc {
	t.Helper()
	pc1 := NewBasicPointCloud(1)
	err := pc1.Set(NewVector(1, 0, 0), NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc2 := NewBasicPointCloud(1)
	err = pc2.Set(NewVector(0, 1, 0), NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc3 := NewBasicPointCloud(1)
	err = pc3.Set(NewVector(0, 0, 1), NewColoredData(color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	pose1 := spatialmath.NewPoseFromPoint(r3.Vector{100, 0, 0})
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{100, 0, 100})
	pose3 := spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 100})
	func1 := func(context context.Context) (PointCloud, spatialmath.Pose, error) {
		return pc1, pose1, nil
	}
	func2 := func(context context.Context) (PointCloud, spatialmath.Pose, error) {
		return pc2, pose2, nil
	}
	func3 := func(context context.Context) (PointCloud, spatialmath.Pose, error) {
		return pc3, pose3, nil
	}
	return []CloudAndOffsetFunc{func1, func2, func3}
}

func TestMergePoints1(t *testing.T) {
	clouds := makeClouds(t)
	cloudsWithOffset := make([]CloudAndOffsetFunc, 0, len(clouds))
	for _, cloud := range clouds {
		cloudCopy := cloud
		cloudFunc := func(ctx context.Context) (PointCloud, spatialmath.Pose, error) {
			return cloudCopy, nil, nil
		}
		cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	}
	mergedCloud := NewBasicEmpty()
	err := MergePointClouds(context.Background(), cloudsWithOffset, mergedCloud)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.Size(), test.ShouldEqual, 9)
}

func TestMergePoints2(t *testing.T) {
	clouds := makeThreeCloudsWithOffsets(t)
	pc := NewBasicEmpty()
	err := MergePointClouds(context.Background(), clouds, pc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 3)

	data, got := pc.At(101, 0, 0)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{255, 0, 0, 255})

	data, got = pc.At(100, 1, 100)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 255, 0, 255})

	data, got = pc.At(100, 100, 101)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 0, 255, 255})
}

func BenchmarkMerge(b *testing.B) {
	inA := newBigPC()
	inB := NewBasicEmpty()
	err := ApplyOffset(inA, spatialmath.NewPoseFromPoint(r3.Vector{1000, 1000, 1000}), inB)
	test.That(b, err, test.ShouldBeNil)

	fs := []CloudAndOffsetFunc{
		func(_ context.Context) (PointCloud, spatialmath.Pose, error) {
			return inA, spatialmath.NewPoseFromPoint(r3.Vector{1, 1, 1}), nil
		},
		func(_ context.Context) (PointCloud, spatialmath.Pose, error) {
			return inB, spatialmath.NewPoseFromPoint(r3.Vector{1, 1, 1}), nil
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := NewBasicPointCloud(0)
		err := MergePointClouds(context.Background(), fs, out)
		test.That(b, err, test.ShouldBeNil)
	}
}
