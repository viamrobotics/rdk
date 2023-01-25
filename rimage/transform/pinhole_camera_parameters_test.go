package transform

import (
	"context"
	"image"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIntrinsicsJSON(t *testing.T) {
	colorIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("rimage/transform/data/intel515_color_camera.json"),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, colorIntrinsics.Height, test.ShouldEqual, 720)
	test.That(t, colorIntrinsics.Width, test.ShouldEqual, 1280)
	test.That(t, colorIntrinsics.Fx, test.ShouldEqual, 900.538)
	test.That(t, colorIntrinsics.Fy, test.ShouldEqual, 900.818)
}

func TestDepthColorIntrinsicsExtrinsics(t *testing.T) {
	sensorParams, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)
	// check depth sensor parameters values
	depthIntrinsics := sensorParams.DepthCamera
	test.That(t, depthIntrinsics.Height, test.ShouldEqual, 768)
	test.That(t, depthIntrinsics.Width, test.ShouldEqual, 1024)
	test.That(t, depthIntrinsics.Fx, test.ShouldEqual, 734.938)
	test.That(t, depthIntrinsics.Fy, test.ShouldEqual, 735.516)

	// check color sensor parameters values
	colorIntrinsics := sensorParams.ColorCamera
	test.That(t, colorIntrinsics.Height, test.ShouldEqual, 720)
	test.That(t, colorIntrinsics.Width, test.ShouldEqual, 1280)
	test.That(t, colorIntrinsics.Fx, test.ShouldEqual, 900.538)
	test.That(t, colorIntrinsics.Fy, test.ShouldEqual, 900.818)

	// check sensorParams sensor parameters values
	gtRotation, err := spatialmath.NewRotationMatrix([]float64{
		0.999958, -0.00838489, 0.00378392,
		0.00824708, 0.999351, 0.0350734,
		-0.00407554, -0.0350407, 0.999378,
	})
	test.That(t, err, test.ShouldBeNil)
	gtTranslation := r3.Vector{-0.000828434, 0.0139185, -0.0033418}

	rotationMatrix := sensorParams.ExtrinsicD2C.Orientation().RotationMatrix()
	translationVector := sensorParams.ExtrinsicD2C.Point()
	test.That(t, spatialmath.OrientationAlmostEqual(rotationMatrix, gtRotation), test.ShouldBeTrue)
	test.That(t, translationVector.X, test.ShouldAlmostEqual, gtTranslation.X)
	test.That(t, translationVector.Y, test.ShouldAlmostEqual, gtTranslation.Y)
	test.That(t, translationVector.Z, test.ShouldAlmostEqual, gtTranslation.Z)
}

func TestTransformPointToPoint(t *testing.T) {
	dcie := NewEmptyDepthColorIntrinsicsExtrinsics()
	x1, y1, z1 := 0., 0., 1.
	t1 := r3.Vector{0, 0, 1}
	// Get rigid body transform between Depth and RGB sensor
	dcie.ExtrinsicD2C = spatialmath.NewPoseFromPoint(t1)
	x2, y2, z2 := dcie.TransformPointToPoint(x1, y1, z1)
	test.That(t, x2, test.ShouldEqual, 0.)
	test.That(t, y2, test.ShouldEqual, 0.)
	test.That(t, z2, test.ShouldEqual, 2.)

	t2 := r3.Vector{0, 2, 0}
	dcie.ExtrinsicD2C = spatialmath.NewPoseFromPoint(t2)
	x3, y3, z3 := dcie.TransformPointToPoint(x1, y1, z1)
	test.That(t, x3, test.ShouldEqual, 0.)
	test.That(t, y3, test.ShouldEqual, 2.)
	test.That(t, z3, test.ShouldEqual, 1.)
	// Rotation in the (z,x) plane of 90 degrees
	rot, err := spatialmath.NewRotationMatrix([]float64{0, 0, 1, 0, 1, 0, -1, 0, 0})
	test.That(t, err, test.ShouldBeNil)
	dcie.ExtrinsicD2C = spatialmath.NewPose(t2, rot)
	x4, y4, z4 := dcie.TransformPointToPoint(x1, y1, z1)
	test.That(t, x4, test.ShouldAlmostEqual, 1.)
	test.That(t, y4, test.ShouldAlmostEqual, 2.)
	test.That(t, z4, test.ShouldAlmostEqual, 0.)
}

func TestUndistortImage(t *testing.T) {
	params800 := &PinholeCameraIntrinsics{
		Width:  800,
		Height: 600,
		Fx:     887.07855759,
		Fy:     886.579955,
		Ppx:    382.80075175,
		Ppy:    302.75546742,
	}
	distortion800 := &BrownConrady{
		RadialK1:     -0.42333866,
		RadialK2:     0.25696641,
		TangentialP1: 0.00142052,
		TangentialP2: -0.00116427,
		RadialK3:     -0.06468911,
	}
	pinhole800 := &PinholeCameraModel{PinholeCameraIntrinsics: params800, Distortion: distortion800}
	params1280 := &PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     1067.68786,
		Fy:     1067.64416,
		Ppx:    629.229310,
		Ppy:    387.990797,
	}
	distortion1280 := &BrownConrady{
		RadialK1:     -4.27329870e-01,
		RadialK2:     2.41688942e-01,
		TangentialP1: 9.33797688e-04,
		TangentialP2: -2.65675762e-04,
		RadialK3:     -6.51379008e-02,
	}
	pinhole1280 := &PinholeCameraModel{PinholeCameraIntrinsics: params1280, Distortion: distortion1280}
	// nil input
	_, err := pinhole800.UndistortImage(nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "input image is nil")
	// wrong size error
	img1280, err := rimage.NewImageFromFile(artifact.MustPath("transform/undistort/distorted_1280x720.jpg"))
	test.That(t, err, test.ShouldBeNil)
	_, err = pinhole800.UndistortImage(img1280)
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")

	// correct undistortion
	outDir, err = testutils.TempDir("", "transform")
	test.That(t, err, test.ShouldBeNil)
	// 800x600
	img800, err := rimage.NewImageFromFile(artifact.MustPath("transform/undistort/distorted_800x600.jpg"))
	test.That(t, err, test.ShouldBeNil)
	corrected800, err := pinhole800.UndistortImage(img800)
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/corrected_800x600.jpg", corrected800)
	test.That(t, err, test.ShouldBeNil)
	// 1280x720
	corrected1280, err := pinhole1280.UndistortImage(img1280)
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/corrected_1280x720.jpg", corrected1280)
	test.That(t, err, test.ShouldBeNil)
}

func TestUndistortDepthMap(t *testing.T) {
	params := &PinholeCameraIntrinsics{ // not the real intrinsic parameters of the depth map
		Width:  1280,
		Height: 720,
		Fx:     1067.68786,
		Fy:     1067.64416,
		Ppx:    629.229310,
		Ppy:    387.990797,
	}
	distortion := &BrownConrady{
		RadialK1:     0.,
		RadialK2:     0.,
		TangentialP1: 0.,
		TangentialP2: 0.,
		RadialK3:     0.,
	}
	var cameraModel PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = params
	cameraModel.Distortion = distortion
	pinhole := &cameraModel
	// nil input
	_, err := pinhole.UndistortDepthMap(nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "input DepthMap is nil")

	// wrong size error
	dmWrong, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("transform/align-test-1615761793.png"))
	test.That(t, err, test.ShouldBeNil)
	_, err = pinhole.UndistortDepthMap(dmWrong)
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")

	// correct undistortion
	outDir, err = testutils.TempDir("", "transform")
	test.That(t, err, test.ShouldBeNil)
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	corrected, err := pinhole.UndistortDepthMap(img)
	test.That(t, err, test.ShouldBeNil)
	// should not have changed the values at all, as distortion parameters are all 0
	test.That(t, corrected.GetDepth(200, 300), test.ShouldEqual, img.GetDepth(200, 300))
	test.That(t, corrected.GetDepth(0, 0), test.ShouldEqual, img.GetDepth(0, 0))
	test.That(t, corrected.GetDepth(1279, 719), test.ShouldEqual, img.GetDepth(1279, 719))
}

func TestGetCameraMatrix(t *testing.T) {
	intrinsics := &PinholeCameraIntrinsics{
		Width:  0,
		Height: 0,
		Fx:     50,
		Fy:     55,
		Ppx:    320,
		Ppy:    160,
	}
	intrinsicsK := intrinsics.GetCameraMatrix()
	test.That(t, intrinsicsK, test.ShouldNotBeNil)
	test.That(t, intrinsicsK.At(0, 0), test.ShouldEqual, intrinsics.Fx)
	test.That(t, intrinsicsK.At(1, 1), test.ShouldEqual, intrinsics.Fy)
	test.That(t, intrinsicsK.At(0, 2), test.ShouldEqual, intrinsics.Ppx)
	test.That(t, intrinsicsK.At(1, 2), test.ShouldEqual, intrinsics.Ppy)
	test.That(t, intrinsicsK.At(2, 2), test.ShouldEqual, 1)
}

func TestNilIntrinsics(t *testing.T) {
	var nilIntrinsics *PinholeCameraIntrinsics
	test.That(t, func() { nilIntrinsics.CheckValid() }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.GetCameraMatrix() }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.PixelToPoint(0.0, 0.0, 0.0) }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.PointToPixel(0.0, 0.0, 0.0) }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.ImagePointTo3DPoint(image.Point{}, rimage.Depth(0)) }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.RGBDToPointCloud(&rimage.Image{}, &rimage.DepthMap{}) }, test.ShouldNotPanic)
	test.That(t, func() { nilIntrinsics.PointCloudToRGBD(pointcloud.PointCloud(nil)) }, test.ShouldNotPanic)
}
