package transform

import (
	"fmt"
	"image"
	"math"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
)

func TestParallelProjectionOntoXZWithRobotMarker(t *testing.T) {
	t.Run("Project an empty pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err.Error(), test.ShouldContainSubstring, "projection point cloud is empty")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a pointcloud with NaN positional value", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()

		p1 := r3.Vector{X: math.NaN(), Y: 0, Z: 0}
		err := pointcloud.Set(p1, pc.NewBasicData())
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err.Error(), test.ShouldContainSubstring, "NaN detected")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a single point pointcloud with no data", func(t *testing.T) {
		pose := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&pose)

		pointcloud := pc.New()
		p1 := r3.Vector{X: 5, Y: 8, Z: 2}
		err := pointcloud.Set(p1, pc.NewBasicData())
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)

		minX := math.Min(pose.Point().X, p1.X)
		maxX := math.Max(pose.Point().X, p1.X)
		minZ := math.Min(pose.Point().Z, p1.Z)
		maxZ := math.Max(pose.Point().Z, p1.Z)

		scaleFactor := math.Min((imageWidth-1)/(maxX-minX), (imageHeight-1)/(maxZ-minZ))

		robotMarkerExpectedPos := image.Point{
			X: int(math.Round((pose.Point().X - minX) * scaleFactor)),
			Y: int(math.Round((pose.Point().Z - minZ) * scaleFactor)),
		}

		colorAtPos := im.GetXY(robotMarkerExpectedPos.X, robotMarkerExpectedPos.Y)
		expectedRobotMarkerColor := rimage.NewColor(255, 0, 0)
		test.That(t, colorAtPos, test.ShouldResemble, expectedRobotMarkerColor)

		pointExpectedPos := image.Point{
			X: int(math.Round((p1.X - minX) * scaleFactor)),
			Y: int(math.Round((p1.Z - minZ) * scaleFactor)),
		}

		colorAtPoint := im.GetXY(pointExpectedPos.X, pointExpectedPos.Y)
		expectedPointColor := rimage.NewColor(255, 255, 255)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, colorAtPoint, test.ShouldResemble, expectedPointColor)
	})

	t.Run("Project a point with out of range data", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()
		err := pointcloud.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(200))
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			fmt.Sprintf("received a value of %v which is outside the range (0 - 100) representing probabilities", 200))
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a two point pointcloud with data with image pixel checks", func(t *testing.T) {
		pose := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&pose)

		pointcloud := pc.New()
		d := pc.NewBasicData()
		p1 := r3.Vector{X: -2, Y: 10, Z: -3}
		err := pointcloud.Set(p1, d)
		test.That(t, err, test.ShouldBeNil)
		p2 := r3.Vector{X: 10, Y: 10, Z: 10}
		err = pointcloud.Set(p2, d)
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)

		minX := math.Min(math.Min(pose.Point().X, p1.X), p2.X)
		maxX := math.Max(math.Max(pose.Point().X, p1.X), p2.X)
		minZ := math.Min(math.Min(pose.Point().Z, p1.Z), p2.Z)
		maxZ := math.Max(math.Max(pose.Point().Z, p1.Z), p2.Z)

		scaleFactor := math.Min((imageWidth-1)/(maxX-minX), (imageHeight-1)/(maxZ-minZ))

		robotMarkerExpectedPos := image.Point{
			X: int(math.Round((pose.Point().X - minX) * scaleFactor)),
			Y: int(math.Round((pose.Point().Z - minZ) * scaleFactor)),
		}

		colorAtPos := im.GetXY(robotMarkerExpectedPos.X, robotMarkerExpectedPos.Y)
		expectedRobotMarkerColor := rimage.NewColor(255, 0, 0)
		test.That(t, colorAtPos, test.ShouldResemble, expectedRobotMarkerColor)

		point1ExpectedPos := image.Point{
			X: int(math.Round((p1.X - minX) / scaleFactor)),
			Y: int(math.Round((p1.Z - minZ) / scaleFactor)),
		}

		colorAtPoint1 := im.GetXY(point1ExpectedPos.X, point1ExpectedPos.Y)
		expectedPoint1Color, err := getColorFromProbabilityValue(d)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, colorAtPoint1, test.ShouldResemble, expectedPoint1Color)

		point2ExpectedPos := image.Point{
			X: int(math.Round((p2.X - minX) / scaleFactor)),
			Y: int(math.Round((p2.Z - minZ) / scaleFactor)),
		}

		colorAtPoint2 := im.GetXY(point2ExpectedPos.X, point2ExpectedPos.Y)
		expectedPoint2Color, err := getColorFromProbabilityValue(d)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, colorAtPoint2, test.ShouldResemble, expectedPoint2Color)
	})

	t.Run("Project an imported pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pcdFile, err := os.Open(artifact.MustPath("pointcloud/test_short.pcd"))
		test.That(t, err, test.ShouldBeNil)

		PC, err := pc.ReadPCD(pcdFile)
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(PC)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Test that projecting two offset pointclouds will produce same image", func(t *testing.T) {
		// Image 1
		pose := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&pose)

		pointcloud := pc.New()
		d := pc.NewBasicData()
		p1 := r3.Vector{X: -2, Y: 10, Z: -3}
		err := pointcloud.Set(p1, d)
		test.That(t, err, test.ShouldBeNil)
		p2 := r3.Vector{X: 10, Y: 10, Z: 10}
		err = pointcloud.Set(p2, d)
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)

		// Image 2
		offset := r3.Vector{X: 7, Y: -19, Z: 2}

		pose1 := spatialmath.NewPose(
			r3.Vector{X: pose.Point().X + offset.X, Y: pose.Point().Y + offset.Y, Z: pose.Point().Z + offset.Z},
			spatialmath.NewOrientationVector(),
		)
		ppRM1 := NewParallelProjectionOntoXZWithRobotMarker(&pose1)

		pointcloud = pc.New()
		d = pc.NewBasicData()
		p1 = r3.Vector{X: p1.X + offset.X, Y: p1.Y + offset.Y, Z: p1.Z + offset.Z}
		err = pointcloud.Set(p1, d)
		test.That(t, err, test.ShouldBeNil)
		p2 = r3.Vector{X: p2.X + offset.X, Y: p2.Y + offset.Y, Z: p2.Z + offset.Z}
		err = pointcloud.Set(p2, d)
		test.That(t, err, test.ShouldBeNil)

		im1, unusedDepthMap1, err := ppRM1.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im1.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im1.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap1, test.ShouldBeNil)

		test.That(t, im, test.ShouldResemble, im1)
	})
}
