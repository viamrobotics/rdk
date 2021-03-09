package point_cloud_segmentation

import (
	"fmt"
	"github.com/kshedden/gonpy"
	"go.viam.com/robotcore/rimage"
	"gonum.org/v1/gonum/mat"
	"math"
	"testing"
)

func TestSegmentPlane(t *testing.T) {
	// get gt point cloud
	r, _ := gonpy.NewFileReader("data/pts.npy")
	data, _ := r.GetFloat64()
	n_pts := int(math.Floor(float64(len(data)) / 3.))
	ptsMat := mat.NewDense(n_pts, 3, data)
	fmt.Println(ptsMat.Dims())
	// get depth map
	//m, err := rimage.BothReadFromFile("data/20210120.1650.32.both.gz")
	m, err := rimage.ParseDepthMap("data/20201218.1406.31.dat.gz")
	if err != nil {
		t.Fatal(err)
	}
	pixel2meter := 0.0001
	pc_ := DepthMapToPointCloud(m, pixel2meter, 338.734, 248.449, 459.336, 459.691)
	err = pc_.WriteToFile("data/pc.las")
	if err != nil {
		fmt.Println(err)
	}

	_, eq := SegmentPlane(pc_, 750, 0.0025, pixel2meter)
	fmt.Print(eq[0], eq[1], eq[2], eq[3])
	fmt.Println("")

	// assign gt plane equation - obtained from open3d library with the same parameters
	gtPlaneEquation := make([]float64, 4)
	//gtPlaneEquation = eq
	gtPlaneEquation[0] = -0.340709153223868
	gtPlaneEquation[1] = -0.4825536661012693
	gtPlaneEquation[2] = -0.8068824153751892
	gtPlaneEquation[3] = 0.048177795968557716
	var norm_gt, norm_est float64
	norm_gt = math.Pow(-0.340709153223868, 2) + math.Pow(-0.4825536661012693, 2) + math.Pow(-0.8068824153751892, 2) + math.Pow(0.048177795968557716, 2)
	norm_est = math.Pow(eq[0], 2) + math.Pow(eq[1], 2) + math.Pow(eq[2], 2) + math.Pow(eq[3], 2)
	fmt.Println(norm_gt)
	fmt.Println(norm_est)
	if math.Abs(norm_gt-norm_est) > 0.01 {
		t.Error("The plane is too different from the unit vector.")
	}

}
