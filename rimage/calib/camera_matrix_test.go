package calib

import (
	"math"
	"os"
	"testing"

	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/rimage"
)

func TestPC1(t *testing.T) {
	iwd, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), true)
	if err != nil {
		t.Fatal(err)
	}

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	cameraMatrices, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
	if err != nil {
		t.Fatal(err)
	}

	pc, err := cameraMatrices.ImageWithDepthToPointCloud(iwd)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)

	file, err := os.OpenFile("out/x.pcd", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	pc.ToPCD(file)
}

func TestPC2(t *testing.T) {
	iwd, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), true)
	if err != nil {
		t.Fatal(err)
	}

	// get camera matrix parameters
	jsonFilePath := "../../robots/configs/intel515_parameters.json"
	colorIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "color")
	if err != nil {
		t.Fatal(err)
	}

	pixel2meter := 0.001
	pc, err := DepthMapToPointCloud(iwd.Depth, pixel2meter, colorIntrinsics, rimage.Depth(0), rimage.Depth(math.MaxUint16))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll("out", 0775)

	err = pc.WriteToFile("out/board2.las")
	if err != nil {
		t.Fatal(err)
	}

}
