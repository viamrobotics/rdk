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
	return si.dots[k]
}

func (si *SegmentedImage) set(p image.Point, val int) {
	k := si.toK(p)
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

// -----

type walkState struct {
	img   vision.Image
	dots  *SegmentedImage
	debug bool

	originalColor utils.HSV
	originalPoint image.Point
}

func (ws *walkState) towardsCenter(p image.Point) image.Point {
	xd := p.X - ws.originalPoint.X
	yd := p.Y - ws.originalPoint.Y

	ret := p

	if xd < 0 {
		ret.X++
	} else if xd > 0 {
		ret.X--
	}

	if yd < 0 {
		ret.Y++
	} else if yd > 0 {
		ret.Y--
	}

	return ret
}

func (ws *walkState) piece(start image.Point, colorNumber int) error {
	if ws.debug {
		golog.Global.Debugf("segmentation.piece start: %v", start)
	}

	ws.dots.set(start, colorNumber)

	ws.originalColor = ws.img.ColorHSV(start)
	ws.originalPoint = start

	// TODO(erh): if i do a full "circle" without a point, stop
	return vision.Walk(start.X, start.Y, ws.img.Width(),
		func(x, y int) error {
			start := image.Point{x, y}
			if start.X < 0 || start.X >= ws.img.Width() || start.Y < 0 || start.Y >= ws.img.Height() {
				return nil
			}

			lastPoint := ws.towardsCenter(start)
			lastCluster := ws.dots.get(lastPoint)
			if lastCluster != colorNumber {
				return nil
			}

			lastColor := ws.img.ColorHSV(lastPoint)
			myColor := ws.img.ColorHSV(start)

			originalDistance := ws.originalColor.Distance(myColor)
			lastDistance := lastColor.Distance(myColor)

			good := (originalDistance < ColorThreshold && lastDistance < (ColorThreshold*2)) ||
				(originalDistance < (ColorThreshold*2) && lastDistance < ColorThreshold)

			if ws.debug {
				golog.Global.Debugf("\t %v "+
					"g: %v "+
					"origColor: %s "+
					"myColor: %s "+
					"origDistance: %v "+
					"lastDistance: %v",
					start,
					good,
					ws.originalColor.ToColorful().Hex(),
					myColor.ToColorful().Hex(),
					originalDistance,
					lastDistance)
			}

			if good {
				ws.dots.set(start, colorNumber)
			}

			return nil

		})
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
		err := ws.piece(start, idx+1)
		if err != nil {
			return nil, err
		}
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

		if err := ws.piece(start, color+1); err != nil {
			return nil, err
		}
	}

	ws.dots.createPalette()

	return ws.dots, nil

}
