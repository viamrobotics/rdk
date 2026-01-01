package pointcloud

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

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

func BenchmarkMergeManyPointClouds(b *testing.B) {
	// File names for the 38 PCD files
	fileNames := []string{
		"December_31_2025_12_14_28_imaging_world_frame_0.pcd",
		"December_31_2025_12_14_35_imaging_world_frame_1.pcd",
		"December_31_2025_12_14_43_imaging_world_frame_2.pcd",
		"December_31_2025_12_14_51_imaging_world_frame_3.pcd",
		"December_31_2025_12_14_59_imaging_world_frame_4.pcd",
		"December_31_2025_12_15_06_imaging_world_frame_5.pcd",
		"December_31_2025_12_15_14_imaging_world_frame_6.pcd",
		"December_31_2025_12_15_22_imaging_world_frame_7.pcd",
		"December_31_2025_12_15_30_imaging_world_frame_8.pcd",
		"December_31_2025_12_15_39_imaging_world_frame_9.pcd",
		"December_31_2025_12_15_47_imaging_world_frame_10.pcd",
		"December_31_2025_12_15_55_imaging_world_frame_11.pcd",
		"December_31_2025_12_16_02_imaging_world_frame_12.pcd",
		"December_31_2025_12_16_10_imaging_world_frame_13.pcd",
		"December_31_2025_12_16_19_imaging_world_frame_14.pcd",
		"December_31_2025_12_16_27_imaging_world_frame_15.pcd",
		"December_31_2025_12_16_34_imaging_world_frame_16.pcd",
		"December_31_2025_12_16_42_imaging_world_frame_17.pcd",
		"December_31_2025_12_16_49_imaging_world_frame_18.pcd",
		"December_31_2025_12_16_58_imaging_world_frame_19.pcd",
		"December_31_2025_12_17_05_imaging_world_frame_20.pcd",
		"December_31_2025_12_17_13_imaging_world_frame_21.pcd",
		"December_31_2025_12_17_21_imaging_world_frame_22.pcd",
		"December_31_2025_12_17_28_imaging_world_frame_23.pcd",
		"December_31_2025_12_17_36_imaging_world_frame_24.pcd",
		"December_31_2025_12_17_44_imaging_world_frame_25.pcd",
		"December_31_2025_12_17_51_imaging_world_frame_26.pcd",
		"December_31_2025_12_17_59_imaging_world_frame_27.pcd",
		"December_31_2025_12_18_07_imaging_world_frame_28.pcd",
		"December_31_2025_12_18_15_imaging_world_frame_29.pcd",
		"December_31_2025_12_18_23_imaging_world_frame_30.pcd",
		"December_31_2025_12_18_34_imaging_world_frame_31.pcd",
		"December_31_2025_12_18_42_imaging_world_frame_32.pcd",
		"December_31_2025_12_18_49_imaging_world_frame_33.pcd",
		"December_31_2025_12_18_58_imaging_world_frame_34.pcd",
		"December_31_2025_12_19_06_imaging_world_frame_35.pcd",
		"December_31_2025_12_19_14_imaging_world_frame_36.pcd",
		"December_31_2025_12_19_22_imaging_world_frame_37.pcd",
	}

	// Load all point clouds
	clouds := make([]PointCloud, 0, len(fileNames))
	totalPoints := 0
	for _, fileName := range fileNames {
		cloud, err := NewFromFile(artifact.MustPath("pointcloud/"+fileName), BasicType)
		test.That(b, err, test.ShouldBeNil)
		clouds = append(clouds, cloud)
		totalPoints += cloud.Size()
		fmt.Printf("  Loaded %s: %d points\n", fileName, cloud.Size())
	}
	fmt.Printf("Total points to merge: %d\n\n", totalPoints)

	camPoses := make([]spatialmath.Pose, len(clouds))
	for i := range camPoses {
		camPoses[i] = spatialmath.NewZeroPose()
	}

	// Create CloudAndOffsetFunc for each cloud with zero offset
	fs := make([]CloudAndOffsetFunc, 0, len(clouds))
	for i, cloud := range clouds {
		cloudCopy := cloud
		fs = append(fs, func(_ context.Context) (PointCloud, spatialmath.Pose, error) {
			return cloudCopy, camPoses[i], nil
		})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := NewBasicPointCloud(0)
		err := MergePointClouds(context.Background(), fs, out)
		test.That(b, err, test.ShouldBeNil)
	}
}
