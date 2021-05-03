package rimage

import (
	"io/ioutil"
	"testing"

	"go.viam.com/robotcore/artifact"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
	golog.Global.Debugf("out dir: %q", outDir)
}

func TestPCRoundTrip(t *testing.T) {
	pc, err := NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	if err != nil {
		t.Fatal(err)
	}

	fn := outDir + "/roundtrip1.both.gz"
	err = pc.WriteTo(fn)
	if err != nil {
		t.Fatal(err)
	}

	pc2, err := BothReadFromFile(fn, true)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pc.Color.Width(), pc2.Color.Width())
	assert.Equal(t, pc.Color.Height(), pc2.Color.Height())
	assert.Equal(t, pc.Depth.Width(), pc2.Depth.Width())
	assert.Equal(t, pc.Depth.Height(), pc2.Depth.Height())
}

func TestImageWithDepthFromImages(t *testing.T) {
	iwd, err := NewImageWithDepthFromImages(artifact.MustPath("rimage/shelf_color.png"), artifact.MustPath("rimage/shelf_grayscale.png"), false)
	if err != nil {
		t.Fatal(err)
	}

	err = iwd.WriteTo(outDir + "/shelf.both.gz")
	if err != nil {
		t.Fatal(err)
	}
}

func TestImageToDepthMap(t *testing.T) {
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
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
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
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
