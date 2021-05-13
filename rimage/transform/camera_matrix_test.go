package transform

import (
	"io/ioutil"
	"math"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "rimage_calib")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

func TestPC1(t *testing.T) {
	iwd, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
	test.That(t, err, test.ShouldBeNil)

	pc, err := cameraMatrices.ImageWithDepthToPointCloud(iwd)
	test.That(t, err, test.ShouldBeNil)

	file, err := os.OpenFile(outDir+"/x.pcd", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()

	pc.ToPCD(file)
}

func TestPC2(t *testing.T) {
	iwd, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	colorIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "color")
	test.That(t, err, test.ShouldBeNil)

	pixel2meter := 0.001
	pc, err := DepthMapToPointCloud(iwd.Depth, pixel2meter, colorIntrinsics, rimage.Depth(0), rimage.Depth(math.MaxUint16))
	test.That(t, err, test.ShouldBeNil)

	err = pc.WriteToFile(outDir + "/board2.las")
	test.That(t, err, test.ShouldBeNil)

}
