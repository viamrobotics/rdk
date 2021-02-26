package vision

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPC1(t *testing.T) {
	m, err := ParseDepthMap("chess/data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	i, err := NewImageFromFile("chess/data/board2.png")
	if err != nil {
		t.Fatal(err)
	}

	pc := PointCloud{m, i}

	file, err := os.OpenFile("/tmp/x.pcd", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	pc.ToPCD(file)
}

func TestPCRoundTrip(t *testing.T) {
	pc, err := NewPointCloud("chess/data/board1.png", "chess/data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)

	fn := "out/roundtrip1.both.gz"
	err = pc.WriteTo(fn)
	if err != nil {
		t.Fatal(err)
	}

	pc2, err := NewPointCloudFromBoth(fn)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pc.Color.Width(), pc2.Color.Width())
	assert.Equal(t, pc.Color.Height(), pc2.Color.Height())
	assert.Equal(t, pc.Depth.Width(), pc2.Depth.Width())
	assert.Equal(t, pc.Depth.Height(), pc2.Depth.Height())
}
