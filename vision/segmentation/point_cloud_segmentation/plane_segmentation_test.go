package point_cloud_segmentation

import (
	"github.com/golang/geo/r3"
	"go.viam.com/robotcore/rimage"
	"gonum.org/v1/gonum/mat"
	"math"
	"testing"
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
	pc_ := DepthMapTo3D(m, pixel2meter, 542.078, 398.016, 734.938, 735.516, depthMin, depthMax)

	_, eq := SegmentPlane(pc_, 1000, 0.0025, pixel2meter)

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
}

func TestDepthMapToPointCloud(t *testing.T) {
	rgbd, err := rimage.BothReadFromFile("data/align-test-1615172036.both.gz")
	m := rgbd.Depth
	//rgb := rgbd.Color

	if err != nil {
		t.Fatal(err)
	}
	pixel2meter := 0.001
	pc := DepthMapToPointCloud(m, pixel2meter, 542.078, 398.016, 734.938, 735.516)
	if pc.Size() != 456371 {
		t.Error("Size of Point Cloud does not correspond to the GT point cloud size.")
	}
}

func TestProjectPlane3dPointsToRGBPlane(t *testing.T) {

}

func TestTransformPointToPoint(t *testing.T) {
	x1, y1, z1 := 0., 0., 1.
	rot1 := mat.NewDense(3, 3, nil)
	rot1.Set(0, 0, 1.)
	rot1.Set(1, 1, 1.)
	rot1.Set(2, 2, 1.)

	t1 := r3.Vector{0., 0., 1.}
	vt1 := TransformPointToPoint(x1, y1, z1, *rot1, t1)
	if vt1.X != 0. {
		t.Error("x value for I rotation and {0,0,1} translation is not 0.")
	}
	if vt1.Y != 0. {
		t.Error("y value for I rotation and {0,0,1} translation is not 0.")
	}
	if vt1.Z != 2. {
		t.Error("z value for I rotation and {0,0,1} translation is not 2.")
	}

	t2 := r3.Vector{0., 2., 0.}
	vt2 := TransformPointToPoint(x1, y1, z1, *rot1, t2)
	if vt2.X != 0. {
		t.Error("x value for I rotation and {0,2,0} translation is not 0.")
	}
	if vt2.Y != 2. {
		t.Error("y value for I rotation and {0,2,0} translation is not 2.")
	}
	if vt2.Z != 1. {
		t.Error("z value for I rotation and {0,2,0} translation is not 1.")
	}
	// Rotation in the (z,x) plane of 90 degrees
	rot2 := mat.NewDense(3, 3, nil)
	rot2.Set(0, 2, 1.)
	rot2.Set(1, 1, 1.)
	rot2.Set(2, 0, -1.)
	vt3 := TransformPointToPoint(x1, y1, z1, *rot2, t2)
	if vt3.X != 1. {
		t.Error("x value for rotation z->x and {0,2,0} translation is not 1.")
	}
	if vt3.Y != 2. {
		t.Error("y value for rotation z->x and {0,2,0} translation is not 2.")
	}
	if vt3.Z != 0. {
		t.Error("z value for rotation z->x and {0,2,0} translation is not 0.")
	}
}
