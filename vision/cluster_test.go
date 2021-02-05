package vision

import (
	"os"
	"testing"

	"github.com/viamrobotics/robotcore/utils"
)

func doTest(t *testing.T, fn string, numClusters int) {
	img, err := NewImageFromFile("data/" + fn)
	if err != nil {
		t.Fatal(err)
	}

	clusters, err := img.ClusterHSV(numClusters)
	if err != nil {
		t.Fatal(err)
	}

	os.Mkdir("out", 0755)

	res := ClusterImage(clusters, img)
	err = utils.WriteImageToFile("out/"+fn, res)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCluster1(t *testing.T) {
	doTest(t, "warped-board-1605543525.png", 4)
}

func TestCluster2(t *testing.T) {
	doTest(t, "chess-segment2.png", 3)
}
