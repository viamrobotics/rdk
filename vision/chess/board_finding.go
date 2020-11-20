package chess

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/vision"
)

func center(contour []image.Point, maxDiff int) image.Point {
	if len(contour) == 0 {
		return image.Point{0, 0}
	}

	x := 0
	y := 0

	for _, p := range contour {
		x += p.X
		y += p.Y
	}

	weightedMiddle := image.Point{x / len(contour), y / len(contour)}

	// TODO: this should be about coniguous, not distance

	box := image.Rectangle{image.Point{1000000, 100000}, image.Point{0, 0}}
	for _, p := range contour {
		if rcutil.AbsInt(p.X-weightedMiddle.X) > maxDiff || rcutil.AbsInt(p.Y-weightedMiddle.Y) > maxDiff {
			continue
		}

		if p.X < box.Min.X {
			box.Min.X = p.X
		}
		if p.Y < box.Min.Y {
			box.Min.Y = p.Y
		}

		if p.X > box.Max.X {
			box.Max.X = p.X
		}
		if p.Y > box.Max.Y {
			box.Max.Y = p.Y
		}

	}

	avgMiddle := image.Point{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2}
	//fmt.Printf("%v -> %v  box: %v\n", weightedMiddle, avgMiddle, box)
	return avgMiddle
}

var (
	myPinks = []vision.Color{
		vision.Color{color.RGBA{208, 73, 99, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{223, 79, 101, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{195, 78, 109, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{198, 65, 106, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{192, 57, 83, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{183, 68, 107, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{171, 61, 100, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{156, 65, 102, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{221, 68, 93, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{205, 63, 87, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{220, 108, 119, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{205, 101, 103, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{172, 90, 112, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{164, 48, 81, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{149, 47, 85, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{142, 45, 120, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{139, 37, 75, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{203, 108, 142, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{196, 97, 139, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{173, 96, 140, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{161, 112, 144, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{140, 82, 108, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{126, 71, 107, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{221, 105, 164, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{223, 117, 159, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{232, 127, 154, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{234, 109, 153, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{201, 148, 184, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{237, 158, 174, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{191, 121, 171, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{179, 145, 183, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{180, 128, 179, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{167, 125, 164, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{217, 144, 163, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{181, 124, 133, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{177, 75, 134, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{163, 69, 132, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{201, 132, 147, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{163, 69, 132, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{154, 80, 136, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{125, 81, 211, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{210, 85, 127, 0}, "myPink", "pink"},
	}
)

func MyPinkDistance(data color.RGBA) float64 {
	d := 10000000000.0
	for _, c := range myPinks {
		temp := c.Distance(data)
		if temp < d {
			d = temp
		}
	}
	return d
}

func inList(l []image.Point, p image.Point) bool {
	for _, t := range l {
		if p.X == t.X && p.Y == t.Y {
			return true
		}
	}
	return false
}

func FindChessCornersPinkCheat_inQuadrant(img gocv.Mat, out *gocv.Mat, cnts [][]image.Point, xQ, yQ int) image.Point {
	debug := false && xQ == 0 && yQ == 1

	best := cnts[xQ+yQ*2]

	// walk up into the corner ---------
	myCenter := center(best, img.Rows()/10)

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

func _avgColor(img gocv.Mat, x, y int) color.RGBA {
	b := 0
	g := 0
	r := 0

	num := 0

	for X := x - 1; X < x+1; X++ {
		for Y := y - 1; Y < y+1; Y++ {
			data := vision.GetColor(img, Y, X)
			b += int(data.B)
			g += int(data.G)
			r += int(data.R)
			num++
		}
	}

	done := color.RGBA{uint8(r / num), uint8(g / num), uint8(b / num), 0}
	return done
}

func FindChessCornersPinkCheat(img gocv.Mat, out *gocv.Mat) ([]image.Point, error) {
	if out != nil {
		if img.Rows() != out.Rows() || img.Cols() != out.Cols() {
			return nil, fmt.Errorf("img and out don't match size %d,%d %d,%d", img.Rows(), img.Cols(), out.Rows(), out.Cols())
		}
	}

	switch img.Type() {
	case gocv.MatTypeCV8UC3:
		//good
	case gocv.MatTypeCV8UC4:
		gocv.CvtColor(img, &img, gocv.ColorBGRAToBGR)
	default:
		panic(fmt.Errorf("what type is this: %v", img.Type()))
	}

	redLittleCircles := []image.Point{}

	cnts := make([][]image.Point, 4)

	for x := 1; x < img.Cols(); x++ {
		for y := 1; y < img.Rows(); y++ {
			data := vision.GetColor(img, y, x)
			p := image.Point{x, y}

			d := MyPinkDistance(data)

			if d < 40 {
				X := int(2 * x / img.Cols())
				Y := int(2 * y / img.Rows())
				Q := X + (Y * 2)
				cnts[Q] = append(cnts[Q], p)
				if out != nil {
					gocv.Circle(out, p, 1, vision.Green.C, 1)
				}
			}

			if false {
				if y == 10 && x > 150 && x < 200 {
					fmt.Printf("  --  %d %d %v %f\n", x, y, data, d)
					redLittleCircles = append(redLittleCircles, p)
				}
			}

		}
	}

	a1Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 0, 0)
	a8Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 1, 0)
	h1Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 0, 1)
	h8Corner := FindChessCornersPinkCheat_inQuadrant(img, out, cnts, 1, 1)

	if out != nil {
		for _, p := range redLittleCircles {
			gocv.Circle(out, p, 1, vision.Red.C, 1)
		}
	}

	return []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}, nil
}
