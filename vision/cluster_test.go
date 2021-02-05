package vision

import (
	"os"
	"testing"

	"github.com/viamrobotics/robotcore/utils"
)

func TestCluster1(t *testing.T) {
	img, err := NewImageFromFile("data/warped-board-1605543525.png")
	if err != nil {
		t.Fatal(err)
	}

	clusters, err := img.ClusterHSV(4)
	if err != nil {
		t.Fatal(err)
	}

	os.Mkdir("out", 7555)

	res := ClusterImage(clusters, img)
	err = utils.WriteImageToFile("out/warped-board-1605543525-kemans.png", res)
	if err != nil {
		t.Fatal(err)
	}

}
