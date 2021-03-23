package rimage

import (
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

func TestPC1(t *testing.T) {
	pc, err := NewImageWithDepth("data/board2.png", "data/board2.dat.gz")
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
	pc, err := NewImageWithDepth("data/board1.png", "data/board1.dat.gz")
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
	iwd, err := NewImageWithDepth("data/board2.png", "data/board2.dat.gz")
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
