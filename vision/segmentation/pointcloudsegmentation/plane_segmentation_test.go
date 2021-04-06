package pointcloudsegmentation

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/calib"
)

func TestSegmentPlane(t *testing.T) {
	// Intel Sensor Extrinsic data from manufacturer
	// Intel sensor depth 1024x768 to  RGB 1280x720
	//Translation Vector : [-0.000828434,0.0139185,-0.0033418]
	//Rotation Matrix    : [0.999958,-0.00838489,0.00378392]
	//                   : [0.00824708,0.999351,0.0350734]
	//                   : [-0.00407554,-0.0350407,0.999378]
	// Intel sensor RGB 1280x720 to depth 1024x768
	// Translation Vector : [0.000699992,-0.0140336,0.00285468]
	//Rotation Matrix    : [0.999958,0.00824708,-0.00407554]
	//                   : [-0.00838489,0.999351,-0.0350407]
	//                   : [0.00378392,0.0350734,0.999378]
	// Intel sensor depth 1024x768 intrinsics
	//Principal Point         : 542.078, 398.016
	//Focal Length            : 734.938, 735.516
	// get depth map
	rgbd, err := rimage.BothReadFromFile("data/align-test-1615172036.both.gz")
	m := rgbd.Depth
	//rgb := rgbd.Color

	if err != nil {
		t.Fatal(err)
	}

	// Pixel to Meter
	pixel2meter := 0.001
	depthMin, depthMax := rimage.Depth(200), rimage.Depth(2000)
	depthIntrinsics, err := calib.NewPinholeCameraIntrinsicsFromJSONFile("../../../robots/configs/intel515_parameters.json", "depth")
	if err != nil {
		t.Fatal(err)
	}
	pts := CreatePoints3DFromDepthMap(m, pixel2meter, *depthIntrinsics, depthMin, depthMax)
	// Segment Plane
	logger := golog.NewTestLogger(t)
	_, eq, _ := pts.SegmentPlane(1000, 0.0025, pixel2meter, logger)

	// assign gt plane equation - obtained from open3d library with the same parameters
	gtPlaneEquation := make([]float64, 4)
	//gtPlaneEquation =  0.02x + 1.00y + 0.09z + -1.12 = 0, obtained from Open3D
	gtPlaneEquation[0] = 0.02
	gtPlaneEquation[1] = 1.0
	gtPlaneEquation[2] = 0.09
	gtPlaneEquation[3] = -1.12

	dot := eq[0]*gtPlaneEquation[0] + eq[1]*gtPlaneEquation[1] + eq[2]*gtPlaneEquation[2]
	if math.Abs(dot) < 0.75 {
		t.Error("The estimated plane normal differs from the GT normal vector too much.")
	}
	// Test conversion function
	pointCloud, _ := pts.convert3DPointsToPointCloud(pixel2meter, logger)
	if pointCloud.Size() == 0 {
		t.Error("pointCloud could not be created.")
	}
}

func TestDepthMapToPointCloud(t *testing.T) {
	logger := golog.NewTestLogger(t)
	rgbd, err := rimage.BothReadFromFile("data/align-test-1615172036.both.gz")
	m := rgbd.Depth
	//rgb := rgbd.Color

	if err != nil {
		t.Fatal(err)
	}
	pixel2meter := 0.001
	depthIntrinsics, err := calib.NewPinholeCameraIntrinsicsFromJSONFile("../../../robots/configs/intel515_parameters.json", "depth")
	if err != nil {
		t.Fatal(err)
	}
	pc, _ := DepthMapToPointCloud(m, pixel2meter, *depthIntrinsics, logger)

	if pc.Size() != 456371 {
		t.Error("Size of Point Cloud does not correspond to the GT point cloud size.")
	}
}

func TestProjectPlane3dPointsToRGBPlane(t *testing.T) {
	rgbd, err := rimage.BothReadFromFile("data/align-test-1615172036.both.gz")
	m := rgbd.Depth
	rgb := rgbd.Color
	h, w := rgb.Height(), rgb.Width()

	if err != nil {
		t.Fatal(err)
	}

	// Pixel to Meter
	pixel2meter := 0.001
	// Select depth range
	depthMin, depthMax := rimage.Depth(200), rimage.Depth(6000)
	// Get 3D Points
	fmt.Println(os.Getwd())
	depthIntrinsics, err := calib.NewPinholeCameraIntrinsicsFromJSONFile("../../../robots/configs/intel515_parameters.json", "depth")
	if err != nil {
		t.Fatal(err)
	}
	pts := CreatePoints3DFromDepthMap(m, pixel2meter, *depthIntrinsics, depthMin, depthMax)
	// Get rigid body transform between Depth and RGB sensor
	sensorParams, err := calib.NewDepthColorIntrinsicsExtrinsicsFromJSONFile("../../../robots/configs/intel515_parameters.json")
	if err != nil {
		t.Fatal(err)
	}
	// Apply RBT
	transformedPoints := pts.ApplyRigidBodyTransform(&sensorParams.ExtrinsicD2C)
	// Re-project 3D Points in RGB Plane
	colorIntrinsics, err := calib.NewPinholeCameraIntrinsicsFromJSONFile("../../../robots/configs/intel515_parameters.json", "color")
	if err != nil {
		t.Fatal(err)
	}
	coordinatesRGB := transformedPoints.ProjectPlane3dPointsToRGBPlane(h, w, *colorIntrinsics, pixel2meter)
	// fill image
	upLeft := image.Point{0, 0}
	lowRight := image.Point{w, h}

	img := image.NewGray16(image.Rectangle{upLeft, lowRight})
	for _, pt := range coordinatesRGB {
		if pt.Z > -1.0 {
			img.Set(int(pt.X), int(pt.Y), color.Gray16{uint16(pt.Z / pixel2meter)})
		}
	}

	maxPt := img.Bounds().Max
	if maxPt.X != rgb.Width() {
		t.Error("Projected Depth map does not have the right width.")
	}
	if maxPt.Y != rgb.Height() {
		t.Error("Projected Depth map does not have the right height.")
	}
}
