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

type walkState struct {
	img   vision.Image
	dots  []int
	debug bool
}

func (ws *walkState) toK(p image.Point) int {
	return (p.Y * ws.img.Width()) + p.X
}

func (ws *walkState) fromK(k int) (int, int) {
	y := k / ws.img.Width()
	x := k - (y * ws.img.Width())
	return x, y
}

func (ws *walkState) dotValue(p image.Point) int {
	k := ws.toK(p)
	return ws.dots[k]
}

func (ws *walkState) setDotValue(p image.Point, val int) {
	k := ws.toK(p)
	ws.dots[k] = val
}

func (ws *walkState) help(originalColor utils.HSV, lastColor utils.HSV, start image.Point, colorNumber int) {
	if start.X < 0 || start.X >= ws.img.Width() || start.Y < 0 || start.Y >= ws.img.Height() {
		return
	}

	if ws.dotValue(start) != 0 {
		return
	}

	myColor := ws.img.ColorHSV(start)

	originalDistance := originalColor.Distance(myColor)
	lastDistance := lastColor.Distance(myColor)

	good := (originalDistance < ColorThreshold && lastDistance < (ColorThreshold*2)) ||
		(originalDistance < (ColorThreshold*2) && lastDistance < ColorThreshold)

	if ws.debug {
		golog.Global.Debugf("\t %v g: %v origColor: %s myColor: %s origDistance: %v lastDistance: %v", // avgDistance: %v",
			start, good, originalColor.ToColorful().Hex(), myColor.ToColorful().Hex(), originalDistance, lastDistance) //, avgDistance)
	}
	if !good {
		ws.setDotValue(start, -1)
		return
	}
	ws.setDotValue(start, colorNumber)

	ws.help(originalColor, myColor, image.Point{start.X + 1, start.Y - 1}, colorNumber)
	ws.help(originalColor, myColor, image.Point{start.X + 1, start.Y + 0}, colorNumber)
	ws.help(originalColor, myColor, image.Point{start.X + 1, start.Y + 1}, colorNumber)

	ws.help(originalColor, myColor, image.Point{start.X - 1, start.Y - 1}, colorNumber)
	ws.help(originalColor, myColor, image.Point{start.X - 1, start.Y + 0}, colorNumber)
	ws.help(originalColor, myColor, image.Point{start.X - 1, start.Y + 1}, colorNumber)

	ws.help(originalColor, myColor, image.Point{start.X + 0, start.Y + 1}, colorNumber)
	ws.help(originalColor, myColor, image.Point{start.X + 0, start.Y + 1}, colorNumber)
}

func (ws *walkState) piece(start image.Point, colorNumber int) error {
	if ws.debug {
		golog.Global.Debugf("segmentation.piece start: %v", start)
	}

	init := ws.img.ColorHSV(start)

	ws.help(init, init, start, colorNumber)

	for k, v := range ws.dots {
		if v == -1 {
			ws.dots[k] = 0
		}
	}

	return nil
}

func ShapeWalk(img vision.Image, startX, startY int, debug bool) (image.Image, error) {

	start := image.Point{startX, startY}
	return ShapeWalkMultiple(img, []image.Point{start}, debug)
}

func ShapeWalkMultiple(img vision.Image, starts []image.Point, debug bool) (image.Image, error) {

	ws := walkState{
		img:   img,
		dots:  make([]int, img.Width()*img.Height()),
		debug: debug,
	}

	palette := colorful.FastWarmPalette(len(starts))
	p2 := []color.RGBA{}
	for _, p := range palette {
		p2 = append(p2, utils.NewColorFromColorful(p).C)
	}

	for idx, start := range starts {
		err := ws.piece(start, idx+1)
		if err != nil {
			return nil, err
		}
	}

	dc := img.ImageCopy()

	for k, v := range ws.dots {
		if v < 1 {
			continue
		}

		x, y := ws.fromK(k)
		dc.Set(x, y, p2[v-1])
	}

	return dc, nil
}

type MyWalkError struct {
	pos image.Point
}

func (e MyWalkError) Error() string {
	return "MyWalkError"
}

func ShapeWalkEntireDebug(img vision.Image, debug bool) (image.Image, error) {
	ws := walkState{
		img:   img,
		dots:  make([]int, img.Width()*img.Height()),
		debug: debug,
	}

	palette := colorful.FastWarmPalette(1000)

	for color := 0; color < len(palette); color++ {

		found := vision.Walk(img.Width()/2, img.Height()/2, img.Width(),
			func(x, y int) error {
				if x < 0 || x >= img.Width() || y < 0 || y >= img.Height() {
					return nil
				}

				if ws.dotValue(image.Point{x, y}) != 0 {
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

	dc := img.ImageCopy()

	for k, v := range ws.dots {
		if v <= 0 {
			continue
		}

		x, y := ws.fromK(k)
		myColor := utils.NewColorFromColorful(palette[v-1]).C
		dc.Set(x, y, myColor)
	}

	return dc, nil

}
