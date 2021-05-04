package rimage

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/robotcore/artifact"
)

func doTest(t *testing.T, fn string, numClusters int) {
	img, err := NewImageFromFile(artifact.MustPath("rimage/" + fn))
	test.That(t, err, test.ShouldBeNil)

	clusters, err := ClusterFromImage(img, numClusters)
	test.That(t, err, test.ShouldBeNil)

	res := ClusterImage(clusters, img)
	err = WriteImageToFile(outDir+"/"+fn, res)
	test.That(t, err, test.ShouldBeNil)
}

func TestCluster1(t *testing.T) {
	doTest(t, "warped-board-1605543525.png", 4)
}

func TestCluster2(t *testing.T) {
	doTest(t, "chess-segment2.png", 3)
}
