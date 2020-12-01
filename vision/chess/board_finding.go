package chess

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/echolabsinc/robotcore/vision"
)

func isPink(data color.RGBA) bool {
	temp, b := colorful.MakeColor(data)
	if !b {
		panic("wtf")
	}
	h, s, v := temp.Hsv()
	if h < 286 {
		return false
	}
	if s < .2 {
		return false
	}
	if v < 100 {
		return false
	}
	return true
}

func inList(l []image.Point, p image.Point) bool {
	for _, t := range l {
		if p.X == t.X && p.Y == t.Y {
			return true
		}
	}
	return false
}

func FindChessCornersPinkCheat_inQuadrant(img vision.Image, out *gocv.Mat, cnts [][]image.Point, xQ, yQ int) image.Point {
	debug := false && xQ == 0 && yQ == 1

	best := cnts[xQ+yQ*2]
	if len(best) == 0 {
		return image.Point{-1, -1}
	}
	// walk up into the corner ---------
	myCenter := vision.Center(best, img.Rows()/10)

	xWalk := ((xQ * 2) - 1)
	yWalk := ((yQ * 2) - 1)

	maxCheckForGreen := img.Rows() / 25

	if debug {
		fmt.Printf("xQ: %d yQ: %d xWalk: %d ywalk: %d maxCheckForGreen: %d\n", xQ, yQ, xWalk, yWalk, maxCheckForGreen)
	}

	for i := 0; i < 50; i++ {
		if inList(best, myCenter) {
			break
		}

		stop := false
		for j := 0; j < maxCheckForGreen; j++ {
			temp := myCenter
			temp.X += j * -1 * xWalk
			if inList(best, temp) {
				stop = true
				break
			}
		}
		if stop {
			break
		}

		myCenter.X += xWalk
		myCenter.Y += yWalk
	}

	if out != nil {
		gocv.Circle(out, myCenter, 5, vision.Red.C, 2)
	}

	return myCenter
}

func FindChessCornersPinkCheat(imgraw gocv.Mat, out *gocv.Mat) ([]image.Point, error) {
	img, err := vision.NewImage(imgraw)
	if err != nil {
		return nil, err
	}

	if out != nil {
		if img.Rows() != out.Rows() || img.Cols() != out.Cols() {
			return nil, fmt.Errorf("img and out don't match size %d,%d %d,%d", img.Rows(), img.Cols(), out.Rows(), out.Cols())
		}
	}

	redLittleCircles := []image.Point{}

	cnts := make([][]image.Point, 4)

	for x := 1; x < img.Cols(); x++ {
		for y := 1; y < img.Rows(); y++ {
			p := image.Point{x, y}
			data := img.Color(p)

			if isPink(data) {
				X := int(2 * x / img.Cols())
				Y := int(2 * y / img.Rows())
				Q := X + (Y * 2)
				cnts[Q] = append(cnts[Q], p)
				if out != nil {
					gocv.Circle(out, p, 1, vision.Green.C, 1)
				}
			}

			if false {
				if y == 127 && x > 250 && x < 350 {
					temp, _ := colorful.MakeColor(data)
					h, s, v := temp.Hsv()
					fmt.Printf("  --  %d %d %v  h: %v s: %v v: %v isPink: %v\n", x, y, data, h, s, v, isPink(data))
					redLittleCircles = append(redLittleCircles, p)
				}
			}

		}
	}

	a1Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 0, 0)
	a8Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 1, 0)
	h1Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 0, 1)
	h8Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 1, 1)

	/*
		if false {
			// figure out orientation of pictures
			xd := (h8Corner.X - h1Corner.X) / 8
			yd := (h8Corner.Y - h1Corner.Y) / 8

			clipMin := image.Point{h1Corner.X + xd*2, h1Corner.Y + yd*2 + xd/20} // the 20 is to move past the black border
			clipMax := image.Point{h1Corner.X + xd*3, h1Corner.Y + yd*3 + xd/2}

			clipBox := image.Rectangle{clipMin, clipMax}
			fmt.Println(clipBox)
			clip := img.Region(clipBox)
			gocv.IMWrite("/tmp/x.png", clip)

			if out != nil {
				gocv.Rectangle(out, clipBox, vision.Purple.C, 1)
			}
		}
	*/

	if out != nil {

		for _, p := range redLittleCircles {
			gocv.Circle(out, p, 1, vision.Red.C, 1)
		}
	}

	raw := []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}
	ret := []image.Point{}
	for _, x := range raw {
		if x.X >= 0 && x.Y >= 0 {
			ret = append(ret, x)
		}
	}
	return ret, nil
}
