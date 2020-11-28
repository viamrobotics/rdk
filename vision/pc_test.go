package vision

import (
	"os"
	"testing"
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
