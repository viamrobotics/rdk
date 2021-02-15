package segmentation

import (
	"image"
	"image/color"

	"github.com/viamrobotics/robotcore/utils"
	"github.com/viamrobotics/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	ColorThreshold = 1.0
)

type SegmentedImage struct {
	palette []color.Color
	dots    []int //  a value of 0 means no segment, < 0 is transient, > 0 is the segment number
	width   int
	height  int
}

func newSegmentedImage(img vision.Image) *SegmentedImage {
	si := &SegmentedImage{
		width:  img.Width(),
		height: img.Height(),
	}
	si.dots = make([]int, si.width*si.height)
	return si
}

func (si *SegmentedImage) toK(p image.Point) int {
	return (p.Y * si.width) + p.X
}

func (si *SegmentedImage) get(p image.Point) int {
	k := si.toK(p)
	if k < 0 || k >= len(si.dots) {
		return 0
	}
	return si.dots[k]
}

func (si *SegmentedImage) set(p image.Point, val int) {
	k := si.toK(p)
	if k < 0 || k >= len(si.dots) {
		return
	}
	si.dots[k] = val
}

func (si *SegmentedImage) PixelsInSegmemnt(segment int) int {
	num := 0
	for _, v := range si.dots {
		if v == segment {
			num++
		}
	}
	return num
}

func (si *SegmentedImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (si *SegmentedImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, si.width, si.height)
}

func (si *SegmentedImage) At(x, y int) color.Color {
	v := si.get(image.Point{x, y})
	if v <= 0 {
		return color.RGBA{0, 0, 0, 0}
	}
	return si.palette[v-1]
}

func (si *SegmentedImage) createPalette() {
	max := 0
	for _, v := range si.dots {
		if v > max {
			max = v
		}
	}

	if max == 0 {
		// no segments
		return
	}

	palette := colorful.FastWarmPalette(max)

	for _, p := range palette {
		si.palette = append(si.palette, p)
	}

}

func (si *SegmentedImage) clearTransients() {
	for k, v := range si.dots {
		if v == -1 {
			si.dots[k] = 0
		}
	}
}

// -----

type walkState struct {
	img   vision.Image
	dots  *SegmentedImage
	debug bool

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

	myColor := ws.img.ColorHSV(p)

	if ws.debug {
		golog.Global.Debugf("\t %v %v", p, myColor.Hex())
	}

	lookback := 10
	for idx, prev := range path[utils.MaxInt(0, len(path)-lookback):] {
		prevColor := ws.img.ColorHSV(prev)
		d := prevColor.Distance(myColor)

		thresold := ColorThreshold + (float64(lookback-idx) / (float64(lookback) * 5.0))

		good := d < thresold

		if ws.debug {
			golog.Global.Debugf("\t\t %v %v %v %0.3f %0.3f", prev, good, prevColor.Hex(), d, thresold)
			if !good && d-thresold < .05 {
				golog.Global.Debugf("\t\t\t http://www.viam.com/color.html?#1=%s&2=%s", myColor.Hex(), prevColor.Hex())
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
	if ws.debug {
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

func ShapeWalk(img vision.Image, start image.Point, debug bool) (*SegmentedImage, error) {

	return ShapeWalkMultiple(img, []image.Point{start}, debug)
}

func ShapeWalkMultiple(img vision.Image, starts []image.Point, debug bool) (*SegmentedImage, error) {

	ws := walkState{
		img:   img,
		dots:  newSegmentedImage(img),
		debug: debug,
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

func ShapeWalkEntireDebug(img vision.Image, debug bool) (*SegmentedImage, error) {
	ws := walkState{
		img:   img,
		dots:  newSegmentedImage(img),
		debug: debug,
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
		if numPixels < 10 {
			golog.Global.Debugf("only found %d pixels in the cluster @ %v", numPixels, start)
			if numPixels == 1 {
				ws.debug = true
				ws.piece(start, color+1)
				ws.debug = debug
			}
		}
	}

	ws.dots.createPalette()

	return ws.dots, nil

}
