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
	Debug        bool
	MaxRadius    int // 0 means no max
	SkipCleaning bool
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
func (ws *walkState) isPixelIsCluster(p image.Point, clusterNumber int, path []image.Point) bool {
	v := ws.dots.get(p)
	if v == 0 {
		good := ws.computeIfPixelIsCluster(p, clusterNumber, path)
		if good {
			v = clusterNumber
			//} else {
			//v = -1 // TODO(erh): remove clearTransients if i don't put this back
		}
		ws.dots.set(p, v)
	}

	return v == clusterNumber
}

func (ws *walkState) computeIfPixelIsCluster(p image.Point, clusterNumber int, path []image.Point) bool {
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

	lookback := 15
	for idx, prev := range path[utils.MaxInt(0, len(path)-lookback):] {
		prevColor := ws.img.ColorHSV(prev)
		d := prevColor.Distance(myColor)

		thresold := ColorThreshold + (float64(lookback-idx) / (float64(lookback) * 10.0))

		good := d < thresold

		if ws.options.Debug {
			golog.Global.Debugf("\t\t %v %v %v %0.3f %0.3f", prev, good, prevColor.Hex(), d, thresold)
			if !good && d-thresold < .2 {
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
func (ws *walkState) pieceWalk(start image.Point, clusterNumber int, path []image.Point, quadrant image.Point) int {
	if !ws.valid(start) {
		return 0
	}

	if len(path) > 0 && ws.dots.get(start) != 0 {
		// don't recompute a spot
		return 0
	}

	if !ws.isPixelIsCluster(start, clusterNumber, path) {
		return 0
	}

	total := 0
	if len(path) > 0 {
		// the original pixel is special and is counted in the main piece function
		total++
	}

	total += ws.pieceWalk(image.Point{start.X + quadrant.X, start.Y + quadrant.Y}, clusterNumber, append(path, start), quadrant)
	if quadrant.Y != 0 && quadrant.X != 0 {
		total += ws.pieceWalk(image.Point{start.X, start.Y + quadrant.Y}, clusterNumber, append(path, start), quadrant)
		total += ws.pieceWalk(image.Point{start.X + quadrant.X, start.Y}, clusterNumber, append(path, start), quadrant)
	}

	return total
}

var (
	allDirections = []image.Point{
		{1, 1},
		{1, 0},
		{1, -1},

		{-1, 1},
		{-1, 0},
		{-1, -1},

		{0, 1},
		{1, 0},
	}
)

// return the number of pieces in the cell
func (ws *walkState) piece(start image.Point, clusterNumber int) int {
	if ws.options.Debug {
		golog.Global.Debugf("segmentation.piece start: %v", start)
	}

	ws.originalColor = ws.img.ColorHSV(start)
	ws.originalPoint = start

	origTotal := 1

	for _, dir := range allDirections {
		origTotal += ws.pieceWalk(start, clusterNumber, []image.Point{}, dir)
	}

	total := origTotal
	if !ws.options.SkipCleaning {
		total = ws.lookForWeirdShapes(clusterNumber)
	}

	if ws.options.Debug && total != origTotal {
		golog.Global.Debugf("shape walk did %v -> %v", origTotal, total)
	}

	ws.dots.clearTransients()

	return total
}

func (ws *walkState) countOut(start image.Point, clusterNumber int, dir image.Point) int {
	total := 0
	for {
		start = image.Point{start.X + dir.X, start.Y + dir.Y}
		if ws.dots.get(start) != clusterNumber {
			break
		}
		total++
	}

	return total
}

func (ws *walkState) lookForWeirdShapesReset(start image.Point, clusterNumber int, dir image.Point, depth int) int {
	old := ws.dots.get(start)

	good := (old == clusterNumber*-1) || (old == clusterNumber && depth == 0)
	if !good {
		return 0
	}

	ws.dots.set(start, clusterNumber)

	total := 0

	if depth > 0 {
		total++
	}

	total += ws.lookForWeirdShapesReset(image.Point{start.X + dir.X, start.Y + dir.Y}, clusterNumber, dir, depth+1)
	if dir.X != 0 && dir.Y != 0 {
		total += ws.lookForWeirdShapesReset(image.Point{start.X + dir.X, start.Y}, clusterNumber, dir, depth+1)
		total += ws.lookForWeirdShapesReset(image.Point{start.X, start.Y + dir.Y}, clusterNumber, dir, depth+1)
	}

	return total
}

func (ws *walkState) lookForWeirdShapes(clusterNumber int) int {
	for k, v := range ws.dots.dots {
		if v != clusterNumber {
			continue
		}

		start := ws.dots.fromK(k)

		for _, dir := range []image.Point{{1, 0}, {1, 1}, {0, 1}, {-1, 1}} {
			r := ws.countOut(start, clusterNumber, dir)
			r += ws.countOut(start, clusterNumber, image.Point{dir.X * -1, dir.Y * -1})
			if r < 3 {
				if ws.options.Debug {
					golog.Global.Debugf("removing %v b/c radius: %d for direction: %v", start, r, dir)
				}
				ws.dots.dots[k] = 0
				break
			}
		}
	}

	for k, v := range ws.dots.dots {
		if v != clusterNumber {
			continue
		}

		ws.dots.dots[k] = -1 * clusterNumber
	}

	total := 1

	ws.dots.set(ws.originalPoint, clusterNumber)
	for _, dir := range allDirections {
		total += ws.lookForWeirdShapesReset(ws.originalPoint, clusterNumber, dir, 0)
	}

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

	xSegments := 20
	ySegments := 20

	nextColor := 1

	for x := 0; x < xSegments; x++ {
		for y := 0; y < ySegments; y++ {

			startX := (x + 1) * (img.Width() / (xSegments + 2))
			startY := (y + 1) * (img.Height() / (ySegments + 2))

			found := vision.Walk(startX, startY, img.Width(),
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
			numPixels := ws.piece(start, nextColor)
			if options.Debug && numPixels < 10 {
				golog.Global.Debugf("only found %d pixels in the cluster @ %v", numPixels, start)
			}

			nextColor++
		}
	}

	ws.dots.createPalette()

	return ws.dots, nil

}
