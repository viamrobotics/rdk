package transform

import (
	"image"
	"math"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "rimage_calib")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

func TestPC1(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
	test.That(t, err, test.ShouldBeNil)

	pcCrop, err := cameraMatrices.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{30, 30}, image.Point{50, 50}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcCrop.Size(), test.ShouldEqual, 1)

	// error -- too many rectangles
	_, err = cameraMatrices.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{30, 30}, image.Point{50, 50}}, image.Rectangle{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one cropping rectangle")

	pc, err := cameraMatrices.RGBDToPointCloud(img, dm)
	test.That(t, err, test.ShouldBeNil)

	file, err := os.OpenFile(outDir+"/x.pcd", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o755)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()

	pointcloud.ToPCD(pc, file, pointcloud.PCDAscii)
}

func TestPC2(t *testing.T) {
	dm, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	colorIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "color")
	test.That(t, err, test.ShouldBeNil)

	pixel2meter := 0.001
	pc, err := DepthMapToPointCloud(dm, pixel2meter, colorIntrinsics, rimage.Depth(0), rimage.Depth(math.MaxUint16))
	test.That(t, err, test.ShouldBeNil)

	err = pointcloud.WriteToLASFile(pc, outDir+"/board2.las")
	test.That(t, err, test.ShouldBeNil)
}

func TestCameraMatrixTo3D(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	// get and set camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
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
