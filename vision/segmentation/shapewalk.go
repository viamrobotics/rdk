package segmentation

import (
	"image"
	"math"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// TODO.
const (
	DefaultColorThreshold             = 1.0
	DefaultLookback                   = 15  // how many pixels do we ensure are similar
	DefaultLookbackScaling            = 6.0 // the bigger the number, the tighter the threshold
	DefaultInterestingThreshold       = .45
	DefaultInterestingRange           = 5
	DefaultAverageColorDistanceWeight = .5
)

// ShapeWalkOptions TODO.
type ShapeWalkOptions struct {
	Debug        bool
	MaxRadius    int // 0 means no max
	SkipCleaning bool
	ThresholdMod float64 // 0 means no modification > 0 means more things will match
	Diffs        *rimage.ColorDiffs
}

type walkState struct {
	img       *rimage.Image
	dots      *SegmentedImage
	options   ShapeWalkOptions
	threshold float64

	originalColor                   rimage.Color
	originalPoint                   image.Point
	originalInterestingPixelDensity float64

	interestingPixels *image.Gray

	depth              *rimage.DepthMap
	depthMin, depthMax rimage.Depth
	depthRange         float64

	logger logging.Logger
}

func (ws *walkState) initIfNot() {
	if ws.interestingPixels == nil {
		ws.interestingPixels = ws.img.InterestingPixels(.2)
		if ws.depth != nil {
			ws.depthMin, ws.depthMax = ws.depth.MinMax()
			ws.depthRange = float64(ws.depthMax) - float64(ws.depthMin)
			if ws.options.Debug {
				ws.logger.Debugf("depthRange %v", ws.depthRange)
			}
		}
	}
}

func (ws *walkState) interestingPixelDensity(p image.Point) float64 {
	total := 0.0
	interesting := 0.0

	err := utils.Walk(p.X, p.Y, DefaultInterestingRange,
		func(x, y int) error {
			if x < 0 || x >= ws.img.Width() || y < 0 || y >= ws.img.Height() {
				return nil
			}

			total++

			k := ws.interestingPixels.PixOffset(x, y)
			if ws.interestingPixels.Pix[k] > 0 {
				interesting++
			}

			return nil
		},
	)
	if err != nil {
		panic(err) // impossible
	}

	return interesting / total
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
	}.
*/
func (ws *walkState) isPixelIsCluster(p image.Point, clusterNumber int, path []image.Point) bool {
	v := ws.dots.get(p)
	if v == 0 {
		good := ws.computeIfPixelIsCluster(p, path)
		if good {
			v = clusterNumber
			// } else {
			// v = -1 // TODO(erh): remove clearTransients if i don't put this back
		}
		ws.dots.set(p, v)
	}

	return v == clusterNumber
}

func (ws *walkState) computeIfPixelIsCluster(p image.Point, path []image.Point) bool {
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

	myColor := ws.img.Get(p)

	myInterestingPixelDensity := ws.interestingPixelDensity(p)
	myInterestingPixelDensity = math.Abs(myInterestingPixelDensity - ws.originalInterestingPixelDensity)

	if ws.options.Debug {
		ws.logger.Debugf("\t %v %v myInterestingPixelDensity: %v", p, myColor.Hex(), myInterestingPixelDensity)
	}

	if myInterestingPixelDensity > DefaultInterestingThreshold {
		if ws.options.Debug {
			ws.logger.Debug("\t\t blocked b/c density")
		}
		return false
	}

	lookback := DefaultLookback
	toLookAt := path[utils.MaxInt(0, len(path)-lookback):]
	for idx, prev := range toLookAt {
		prevColor := ws.img.Get(prev)
		d := prevColor.Distance(myColor)
		if ws.options.Diffs != nil && d > .1 {
			ws.options.Diffs.AddD(prevColor, myColor, d)
		}

		threshold := ws.threshold + (float64(lookback-idx) / (float64(lookback) * DefaultLookbackScaling))
		depthThreshold := 0.0

		if idx == len(toLookAt)-1 && ws.depth != nil {
			// only look at the last point
			myZ := ws.depth.Get(p)
			prevZ := ws.depth.Get(prev)
			if myZ > 0 && prevZ > 0 {
				depthThreshold = math.Abs(float64(myZ) - float64(prevZ))
				// in mm right now

				// first we scale to 0 -> 1 based on the data in the image
				depthThreshold /= ws.depthRange

				depthThreshold = ((depthThreshold - .01) * 50)
				depthThreshold *= -1

				if ws.options.Debug {
					ws.logger.Debugf("\t\t\t XXX %v %v %v", myZ, prevZ, depthThreshold)
				}
			} else if myZ > 0 || prevZ > 0 {
				// this means one of the points had good data and one didn't
				// this usually means it's an edge or something
				// so make the threshold a bit smaller
				depthThreshold = -1.1
			}
		}

		threshold += depthThreshold

		good := d < threshold

		if ws.options.Debug {
			ws.logger.Debugf("\t\t %v %v %v %0.3f threshold: %0.3f depthThreshold: %0.3f", prev, good, prevColor.Hex(), d, threshold, depthThreshold)
			if !good && d-threshold < .2 {
				ws.logger.Debugf("\t\t\t http://www.viam.com/color.html?#1=%s&2=%s", myColor.Hex()[1:], prevColor.Hex()[1:])
			}
		}

		if !good {
			return false
		}
	}

	return true
}

// return the number of pieces added.
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

var allDirections = []image.Point{
	{1, 1},
	{1, 0},
	{1, -1},

	{-1, 1},
	{-1, 0},
	{-1, -1},

	{0, 1},
	{1, 0},
}

// return the number of pieces in the cell.
func (ws *walkState) piece(start image.Point, clusterNumber int) int {
	if ws.options.Debug {
		ws.logger.Debugf("segmentation.piece start: %v", start)
	}

	ws.initIfNot()

	// ws.originalColor = ws.img.ColorHSV(start)
	ws.originalPoint = start
	ws.originalInterestingPixelDensity = ws.interestingPixelDensity(start)

	temp, averageColorDistance := ws.img.AverageColorAndStats(start, 1)
	ws.originalColor = temp

	//nolint:ifshort,nolintlint
	oldThreshold := ws.threshold
	defer func() {
		ws.threshold = oldThreshold
	}()

	ws.threshold += averageColorDistance * DefaultAverageColorDistanceWeight
	ws.threshold += ws.originalInterestingPixelDensity

	if ws.options.Debug {
		ws.logger.Debugf("\t\t averageColorDistance: %v originalInterestingPixelDensity: %v threshold: %v -> %v",
			averageColorDistance,
			ws.originalInterestingPixelDensity,
			oldThreshold, ws.threshold,
		)
	}

	origTotal := 1 // we count the original pixel here

	for _, dir := range allDirections {
		origTotal += ws.pieceWalk(start, clusterNumber, []image.Point{}, dir)
	}

	total := origTotal
	if !ws.options.SkipCleaning {
		total = ws.lookForWeirdShapes(clusterNumber)
	}

	if ws.options.Debug && total != origTotal {
		ws.logger.Debugf("shape walk did %v -> %v", origTotal, total)
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
					ws.logger.Debugf("removing %v b/c radius: %d for direction: %v", start, r, dir)
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

// ShapeWalk TODO.
func ShapeWalk(img *rimage.Image, dm *rimage.DepthMap, start image.Point, options ShapeWalkOptions, logger logging.Logger,
) (*SegmentedImage, error) {
	return ShapeWalkMultiple(img, dm, []image.Point{start}, options, logger)
}

// ShapeWalkMultiple TODO.
func ShapeWalkMultiple(
	img *rimage.Image, dm *rimage.DepthMap,
	starts []image.Point,
	options ShapeWalkOptions,
	logger logging.Logger,
) (*SegmentedImage, error) {
	ws := walkState{
		img:       img,
		depth:     dm,
		dots:      newSegmentedImage(img),
		options:   options,
		threshold: DefaultColorThreshold + options.ThresholdMod,
		logger:    logger,
	}

	for idx, start := range starts {
		ws.piece(start, idx+1)
	}

	ws.dots.createPalette()

	return ws.dots, nil
}

// MyWalkError TODO.
type MyWalkError struct {
	pos image.Point
}

// Error TODO.
func (e MyWalkError) Error() string {
	return "MyWalkError"
}

// ShapeWalkEntireDebug TODO.
func ShapeWalkEntireDebug(img *rimage.Image, dm *rimage.DepthMap, options ShapeWalkOptions, logger logging.Logger,
) (*SegmentedImage, error) {
	var si *SegmentedImage
	var err error

	for extra := 0.0; extra < .7; extra += .2 {
		si, err = shapeWalkEntireDebugOnePass(img, dm, options, extra, logger)
		if err != nil {
			return nil, err
		}

		if true {
			// TODO(erh): is this idea worth exploring
			break
		}

		if float64(si.NumInAnyCluster()) > float64(img.Width()*img.Height())*.9 {
			break
		}
	}

	return si, err
}

func shapeWalkEntireDebugOnePass(
	img *rimage.Image, dm *rimage.DepthMap,
	options ShapeWalkOptions,
	extraThreshold float64,
	logger logging.Logger,
) (*SegmentedImage, error) {
	ws := walkState{
		img:       img,
		depth:     dm,
		dots:      newSegmentedImage(img),
		options:   options,
		threshold: DefaultColorThreshold + options.ThresholdMod + extraThreshold,
		logger:    logger,
	}

	radius := 10
	nextColor := 1

	middleX := img.Width() / 2
	middleY := img.Height() / 2

	xStep := img.Width() / (radius * 2)
	yStep := img.Height() / (radius * 2)

	err := utils.Walk(0, 0, radius, func(x, y int) error {
		startX := middleX + (x * xStep)
		startY := middleY + (y * yStep)

		found := utils.Walk(startX, startY, img.Width(),
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
			return nil
		}

		var walkErr MyWalkError
		if !errors.As(found, &walkErr) {
			return errors.Wrapf(found, "expected %T but got", walkErr)
		}
		start := walkErr.pos
		numPixels := ws.piece(start, nextColor)
		if options.Debug && numPixels < 10 {
			ws.logger.Debugf("only found %d pixels in the cluster @ %v", numPixels, start)
		}

		nextColor++

		return nil
	})
	if err != nil {
		return nil, err
	}

	ws.dots.createPalette()

	return ws.dots, nil
}
