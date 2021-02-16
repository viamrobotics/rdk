package segmentation

import (
	"image"

	"github.com/viamrobotics/robotcore/utils"
	"github.com/viamrobotics/robotcore/vision"

	"github.com/edaniels/golog"
)

const (
	ColorThreshold = 1.0
)

type ShapeWalkOptions struct {
	Debug     bool
	MaxRadius int // 0 means no max
}

type walkState struct {
	img     vision.Image
	dots    *SegmentedImage
	options ShapeWalkOptions

	originalColor utils.HSV
	originalPoint image.Point
}

func (ws *walkState) valid(p image.Point) bool {
	return p.X >= 0 && p.X < ws.img.Width() && p.Y >= 0 && p.Y < ws.img.Height()
}

/*
func (ws *walkState) towardsCenter(p image.Point, amount int) image.Point {
	xd := p.X - ws.originalPoint.X
	yd := p.Y - ws.originalPoint.Y

	ret := p

	if xd < 0 {
		ret.X += utils.MinInt(amount, utils.AbsInt(xd))
	} else if xd > 0 {
		ret.X -= utils.MinInt(amount, utils.AbsInt(xd))
	}

	if yd < 0 {
		ret.Y += utils.MinInt(amount, utils.AbsInt(yd))
	} else if yd > 0 {
		ret.Y -= utils.MinInt(amount, utils.AbsInt(yd))
	}

	return ret
}
*/
func (ws *walkState) isPixelIsCluster(p image.Point, colorNumber int, path []image.Point) bool {
	v := ws.dots.get(p)
	if v == 0 {
		good := ws.computeIfPixelIsCluster(p, colorNumber, path)
		if good {
			v = colorNumber
		} else {
			v = -1
		}
		ws.dots.set(p, v)
	}

	return v == colorNumber
}

func (ws *walkState) computeIfPixelIsCluster(p image.Point, colorNumber int, path []image.Point) bool {
	if !ws.valid(p) {
		return false
	}

	if len(path) == 0 {
		if p.X == ws.originalPoint.X && p.Y == ws.originalPoint.Y {
			return true
		}
		panic("wtf")
	}

	if ws.options.MaxRadius > 0 {
		d1 := utils.AbsInt(p.X - ws.originalPoint.X)
		d2 := utils.AbsInt(p.Y - ws.originalPoint.Y)
		if d1 > ws.options.MaxRadius || d2 > ws.options.MaxRadius {
			return false
		}
	}

	myColor := ws.img.ColorHSV(p)

	if ws.options.Debug {
		golog.Global.Debugf("\t %v %v", p, myColor.Hex())
	}

	lookback := 20
	for idx, prev := range path[utils.MaxInt(0, len(path)-lookback):] {
		prevColor := ws.img.ColorHSV(prev)
		d := prevColor.Distance(myColor)

		thresold := ColorThreshold + (float64(lookback-idx) / (float64(lookback) * 12.0))

		good := d < thresold

		if ws.options.Debug {
			golog.Global.Debugf("\t\t %v %v %v %0.3f %0.3f", prev, good, prevColor.Hex(), d, thresold)
			if !good && d-thresold < .05 {
				golog.Global.Debugf("\t\t\t http://www.viam.com/color.html?#1=%s&2=%s", myColor.Hex()[1:], prevColor.Hex()[1:])
			}

		}

		if !good {
			return false
		}

	}

	return true
}

// return the number of pieces added
func (ws *walkState) pieceWalk(start image.Point, colorNumber int, path []image.Point, quadrant image.Point) int {
	if !ws.valid(start) {
		return 0
	}

	if len(path) > 0 && ws.dots.get(start) != 0 {
		// don't recompute a spot
		return 0
	}

	if !ws.isPixelIsCluster(start, colorNumber, path) {
		return 0
	}

	total := 0
	if len(path) > 0 {
		// the original pixel is special and is counted in the main piece function
		total++
	}

	total += ws.pieceWalk(image.Point{start.X + quadrant.X, start.Y + quadrant.Y}, colorNumber, append(path, start), quadrant)
	total += ws.pieceWalk(image.Point{start.X, start.Y + quadrant.Y}, colorNumber, append(path, start), quadrant)
	total += ws.pieceWalk(image.Point{start.X + quadrant.X, start.Y}, colorNumber, append(path, start), quadrant)

	return total
}

// return the number of pieces in the cell
func (ws *walkState) piece(start image.Point, colorNumber int) int {
	if ws.options.Debug {
		golog.Global.Debugf("segmentation.piece start: %v", start)
	}

	ws.originalColor = ws.img.ColorHSV(start)
	ws.originalPoint = start

	total := 1

	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{1, 1})
	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{1, 0})
	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{1, -1})

	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{-1, 1})
	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{-1, 0})
	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{-1, -1})

	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{0, 1})
	total += ws.pieceWalk(start, colorNumber, []image.Point{}, image.Point{0, -1})

	ws.dots.clearTransients()
	return total
}

func ShapeWalk(img vision.Image, start image.Point, options ShapeWalkOptions) (*SegmentedImage, error) {

	return ShapeWalkMultiple(img, []image.Point{start}, options)
}

func ShapeWalkMultiple(img vision.Image, starts []image.Point, options ShapeWalkOptions) (*SegmentedImage, error) {

	ws := walkState{
		img:     img,
		dots:    newSegmentedImage(img),
		options: options,
	}

	for idx, start := range starts {
		ws.piece(start, idx+1)
	}

	ws.dots.createPalette()

	return ws.dots, nil
}

type MyWalkError struct {
	pos image.Point
}

func (e MyWalkError) Error() string {
	return "MyWalkError"
}

func ShapeWalkEntireDebug(img vision.Image, options ShapeWalkOptions) (*SegmentedImage, error) {
	ws := walkState{
		img:     img,
		dots:    newSegmentedImage(img),
		options: options,
	}

	for color := 0; color < 1000; color++ {

		found := vision.Walk(img.Width()/2, img.Height()/2, img.Width(),
			func(x, y int) error {
				if x < 0 || x >= img.Width() || y < 0 || y >= img.Height() {
					return nil
				}

				if ws.dots.get(image.Point{x, y}) != 0 {
					return nil
				}
				return MyWalkError{image.Point{x, y}}
			})

		if found == nil {
			break
		}

		start := found.(MyWalkError).pos
		numPixels := ws.piece(start, color+1)
		if options.Debug && numPixels < 10 {
			golog.Global.Debugf("only found %d pixels in the cluster @ %v", numPixels, start)
		}
	}

	ws.dots.createPalette()

	return ws.dots, nil

}
