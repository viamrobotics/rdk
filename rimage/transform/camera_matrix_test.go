package transform

import (
	"context"
	"image"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

func TestPC1(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	// get camera matrix parameters
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)

	pcCrop, err := cameraMatrices.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{30, 30}, image.Point{50, 50}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcCrop.Size(), test.ShouldEqual, 1)

	// error -- too many rectangles
	_, err = cameraMatrices.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{30, 30}, image.Point{50, 50}}, image.Rectangle{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one cropping rectangle")

	pc, err := cameraMatrices.RGBDToPointCloud(img, dm)
	test.That(t, err, test.ShouldBeNil)

	file, err := os.OpenFile(t.TempDir()+"/x.pcd", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o755)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()

	pointcloud.ToPCD(pc, file, pointcloud.PCDAscii)
}

func TestCameraMatrixTo3D(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	// get and set camera matrix parameters
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)

	// test To3D
	testPoint := image.Point{0, 0}
	vec, err := cameraMatrices.ImagePointTo3DPoint(testPoint, dm.Get(testPoint))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vec.Z, test.ShouldEqual, float64(dm.Get(testPoint)))
	// out of bounds - panic
	testPoint = image.Point{img.Width(), img.Height()}
	test.That(t, func() { cameraMatrices.ImagePointTo3DPoint(testPoint, dm.Get(testPoint)) }, test.ShouldPanic)
}
