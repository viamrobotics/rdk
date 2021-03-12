package point_cloud_segmentation

import (
	"fmt"
	"go.viam.com/robotcore/rimage"
	"image"
	"image/png"
	"math"
	"os"
	"testing"
)

func savePNG(fn string, m image.Image) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, m)
}

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
	rgbd, err := rimage.BothReadFromFile("/Users/louisenaud/Dropbox/echolabs_data/intel515alginment/align-test-1615172036.both.gz")
	m := rgbd.Depth
	//rgb := rgbd.Color

	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(m.Rows(), m.Cols())
	//w, h := m.Width(), m.Height()


	// Pixel to Meter number from Intel
	//pixel2meter := 0.000250000011874363
	pixel2meter := 0.001
	depthMin, depthMax := rimage.Depth(200), rimage.Depth(2000)
	pc_ := DepthMapTo3D(m, pixel2meter, 542.078, 398.016, 734.938, 735.516, depthMin, depthMax)

	plane_idx, eq := SegmentPlane(pc_, 1000, 0.0025, pixel2meter)
	fmt.Print(eq[0], eq[1], eq[2], eq[3])
	fmt.Println("")
	fmt.Println(len(plane_idx))

	// assign gt plane equation - obtained from open3d library with the same parameters
	gtPlaneEquation := make([]float64, 4)
	//gtPlaneEquation =  0.02x + 1.00y + 0.09z + -1.12 = 0, obtained from Open3D

	gtPlaneEquation[0] = 0.02
	gtPlaneEquation[1] = 1.0
	gtPlaneEquation[2] = 0.09
	gtPlaneEquation[3] = -1.12

	dot := eq[0] * gtPlaneEquation[0] + eq[1] * gtPlaneEquation[1] + eq[2] * gtPlaneEquation[2]
	fmt.Println(dot)
	if math.Abs(dot) < 0.75{
		t.Error("The estimated plane normal differs from the GT normal vector too much.")
	}


	//if math.Abs(gtPlaneEquation[0] - math.Abs(eq[0]) ) > 0.075 {
	//	t.Error("The plane is too different in the x axis.")
	//}
	//if math.Abs(gtPlaneEquation[1] - sign * eq[1]) > 0.075 {
	//	t.Error("The plane is too different in the y axis.")
	//}
	//if math.Abs(gtPlaneEquation[2] - sign * eq[2]) > 0.075 {
	//	t.Error("The plane is too different in the z axis.")
	//}
	//if math.Abs(gtPlaneEquation[3] - eq[3]) > 0.05 {
	//	t.Error("The plane offsets are too different.")
	//}
}
