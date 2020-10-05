package vision

import (
	"path/filepath"
	"testing"

	"gocv.io/x/gocv"
)

func Test1(t *testing.T) {
	
	files, err := filepath.Glob("data/*.jpg")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		img := gocv.IMRead(f, gocv.IMReadUnchanged)
		process(img)
		//process2(img)
		gocv.IMWrite("out/" + f, img)
		
	}
	
}
