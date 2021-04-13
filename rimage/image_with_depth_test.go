package rimage

import (
	"os"
	"testing"

	"go.viam.com/robotcore/artifact"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

func TestPC1(t *testing.T) {
	pc, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"))
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

func TestPCRoundTrip(t *testing.T) {
	pc, err := NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"))
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)

	fn := "out/roundtrip1.both.gz"
	err = pc.WriteTo(fn)
	if err != nil {
		t.Fatal(err)
	}

	pc2, err := BothReadFromFile(fn)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pc.Color.Width(), pc2.Color.Width())
	assert.Equal(t, pc.Color.Height(), pc2.Color.Height())
	assert.Equal(t, pc.Depth.Width(), pc2.Depth.Width())
	assert.Equal(t, pc.Depth.Height(), pc2.Depth.Height())
}

func TestPC3(t *testing.T) {
	logger := golog.NewTestLogger(t)
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"))
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)

	pc, err := iwd.ToPointCloud(logger)
	if err != nil {
		t.Fatal(err)
	}

	err = pc.WriteToFile("out/board2.las")
	if err != nil {
		t.Fatal(err)
	}

}

func TestImageWithDepthFromImages(t *testing.T) {
	iwd, err := NewImageWithDepthFromImages(artifact.MustPath("rimage/shelf_color.png"), artifact.MustPath("rimage/shelf_grayscale.png"))
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)

	err = iwd.WriteTo("out/shelf.both.gz")
	if err != nil {
		t.Fatal(err)
	}
}

func TestImageToDepthMap(t *testing.T) {
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"))
	if err != nil {
		t.Fatal(err)
	}
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()
	// convert back
	dmFromImage := imageToDepthMap(depthImage)
	// tests
	assert.Equal(t, dmFromImage.Height(), iwd.Depth.Height())
	assert.Equal(t, dmFromImage.Width(), iwd.Depth.Width())
	assert.Equal(t, dmFromImage, iwd.Depth)
}

func TestConvertToDepthMap(t *testing.T) {
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"))
	if err != nil {
		t.Fatal(err)
	}
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()

	// case 1
	dm1, err := ConvertImageToDepthMap(iwd)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, dm1, iwd.Depth)
	// case 2
	dm2, err := ConvertImageToDepthMap(depthImage)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, dm2, iwd.Depth)
	// default - should return error
	badType := iwd.Color
	dm3, err := ConvertImageToDepthMap(badType)
	if dm3 != nil {
		t.Errorf("expected error for image type %T, got err = %v", badType, err)
	}
}
