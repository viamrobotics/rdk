package chess

import (
	"image"
	"image/color"

	"go.viam.com/robotcore/utils"
	"go.viam.com/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
	"github.com/lucasb-eyer/go-colorful"
)

func isPink(data color.RGBA) bool {
	temp := utils.ConvertToHSV(data)

	if temp.H < 286 {
		return false
	}
	if temp.S < .2 {
		return false
	}
	if temp.V < .5 {
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

func FindChessCornersPinkCheatInQuadrant(img vision.Image, dc *gg.Context, cnts [][]image.Point, xQ, yQ int) image.Point {
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
		golog.Global.Debugf("xQ: %d yQ: %d xWalk: %d ywalk: %d maxCheckForGreen: %d\n", xQ, yQ, xWalk, yWalk, maxCheckForGreen)
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

	dc.DrawCircle(float64(myCenter.X), float64(myCenter.Y), 5)
	dc.SetColor(utils.Red.C)
	dc.Fill()

	return myCenter
}

func FindChessCornersPinkCheat(img vision.Image) (image.Image, []image.Point, error) {
	dc := gg.NewContext(img.Cols(), img.Rows())
	redLittleCircles := []image.Point{}

	cnts := make([][]image.Point, 4)

	for x := 1; x < img.Cols(); x++ {
		for y := 1; y < img.Rows(); y++ {
			p := image.Point{x, y}
			data := img.Color(p)

			if isPink(data) {
				X := 2 * x / img.Cols()
				Y := 2 * y / img.Rows()
				Q := X + (Y * 2)
				cnts[Q] = append(cnts[Q], p)
				dc.DrawCircle(float64(x), float64(y), 1)
				dc.SetColor(utils.Green.C)
				dc.Fill()
			}

			if false {
				if y == 127 && x > 250 && x < 350 {
					temp, _ := colorful.MakeColor(data)
					h, s, v := temp.Hsv()
					golog.Global.Debugf("  --  %d %d %v  h: %v s: %v v: %v isPink: %v\n", x, y, data, h, s, v, isPink(data))
					redLittleCircles = append(redLittleCircles, p)
				}
			}

		}
	}

	a1Corner := FindChessCornersPinkCheatInQuadrant(img, dc, cnts, 0, 0)
	a8Corner := FindChessCornersPinkCheatInQuadrant(img, dc, cnts, 1, 0)
	h1Corner := FindChessCornersPinkCheatInQuadrant(img, dc, cnts, 0, 1)
	h8Corner := FindChessCornersPinkCheatInQuadrant(img, dc, cnts, 1, 1)

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
			utils.WriteImageToFile("/tmp/x.png", clip)

			if out != nil {
				DrawRectangle(out, clipBox, vision.Purple.C, 1)
			}
		}
	*/

	for _, p := range redLittleCircles {
		dc.DrawCircle(float64(p.X), float64(p.Y), 1)
		dc.SetColor(utils.Red.C)
		dc.Fill()
	}

	raw := []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}
	ret := []image.Point{}
	for _, x := range raw {
		if x.X >= 0 && x.Y >= 0 {
			ret = append(ret, x)
		}
	}
	return dc.Image(), ret, nil
}
