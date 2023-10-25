package pointcloud

import (
	"context"
	"image/color"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/spatialmath"
)

func makeThreeCloudsWithOffsets(t *testing.T) []CloudAndOffsetFunc {
	t.Helper()
	pc1 := NewWithPrealloc(1)
	err := pc1.Set(NewVector(1, 0, 0), NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc2 := NewWithPrealloc(1)
	err = pc2.Set(NewVector(0, 1, 0), NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc3 := NewWithPrealloc(1)
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

func TestApplyOffset(t *testing.T) {
	// TODO(RSDK-1200): remove skip when complete
	t.Skip("remove skip once RSDK-1200 improvement is complete")
	logger := logging.NewTestLogger(t)
	pc1 := NewWithPrealloc(3)
	err := pc1.Set(NewVector(1, 0, 0), NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = pc1.Set(NewVector(1, 1, 0), NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = pc1.Set(NewVector(1, 1, 1), NewColoredData(color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	// apply a simple translation
	transPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 99, 0})
	transPc, err := ApplyOffset(context.Background(), pc1, transPose, logger)
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
	transrotPc, err := ApplyOffset(context.Background(), pc1, transrotPose, logger)
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

func TestMergePoints1(t *testing.T) {
	// TODO(RSDK-1200): remove skip when complete
	t.Skip("remove skip once RSDK-1200 improvement is complete")
	logger := logging.NewTestLogger(t)
	clouds := makeClouds(t)
	cloudsWithOffset := make([]CloudAndOffsetFunc, 0, len(clouds))
	for _, cloud := range clouds {
		cloudCopy := cloud
		cloudFunc := func(ctx context.Context) (PointCloud, spatialmath.Pose, error) {
			return cloudCopy, nil, nil
		}
		cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	}
	mergedCloud, err := MergePointClouds(context.Background(), cloudsWithOffset, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud, test.ShouldNotBeNil)
	test.That(t, mergedCloud.Size(), test.ShouldEqual, 9)
}

func TestMergePoints2(t *testing.T) {
	// TODO(RSDK-1200): remove skip when complete
	t.Skip("remove skip once RSDK-1200 improvement is complete")
	logger := logging.NewTestLogger(t)
	clouds := makeThreeCloudsWithOffsets(t)
	pc, err := MergePointClouds(context.Background(), clouds, logger)
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

func TestMergePointsWithColor(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud, err := MergePointCloudsWithColor(clouds)
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
