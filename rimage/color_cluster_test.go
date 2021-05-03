package rimage

import (
	"testing"

	"go.viam.com/robotcore/artifact"
)

func doTest(t *testing.T, fn string, numClusters int) {
	img, err := NewImageFromFile(artifact.MustPath("rimage/" + fn))
	if err != nil {
		t.Fatal(err)
	}

	clusters, err := ClusterFromImage(img, numClusters)
	if err != nil {
		t.Fatal(err)
	}

	res := ClusterImage(clusters, img)
	err = WriteImageToFile(outDir+"/"+fn, res)
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
