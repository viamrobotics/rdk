package vision

import (
	"testing"

	"gocv.io/x/gocv"
)

func Test1(t *testing.T) {
	img := gocv.IMRead("chess_test1.bmp", gocv.IMReadUnchanged)
	process(img)
	//process2(img)

	gocv.IMWrite("test1.out.bmp", img)

}
