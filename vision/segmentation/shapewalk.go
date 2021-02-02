package segmentation

import (
	"image"

	"github.com/echolabsinc/robotcore/vision"

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

func (ws *walkState) help(originalColor vision.HSV, lastColor vision.HSV, start image.Point, colorNumber int) {
	if start.X < 0 || start.X >= ws.img.Width() || start.Y < 0 || start.Y >= ws.img.Height() {
		return
	}

	if ws.dotValue(start) != 0 {
		return
	}

	myColor := ws.img.ColorHSV(start)

	originalDistance := originalColor.Distance(myColor)
	lastDistance := lastColor.Distance(myColor)

	good := originalDistance < ColorThreshold || (originalDistance < (ColorThreshold*1.1) && lastDistance < ColorThreshold/1)

	if ws.debug {
		golog.Global.Debugf("good: %v originalColor: %s point: %v myColor: %s originalDistance: %v lastDistance: %v",
			good, originalColor.ToColorful().Hex(), start, myColor.ToColorful().Hex(), originalDistance, lastDistance)
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

	ws := walkState{
		img:   img,
		dots:  make([]int, img.Width()*img.Height()),
		debug: debug,
	}

	if err := ws.piece(start, 1); err != nil {
		return nil, err
	}

	dc := img.ImageCopy()

	for k, v := range ws.dots {
		if v != 1 {
			continue
		}

		x, y := ws.fromK(k)
		dc.Set(x, y, vision.Red.C)
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
		myColor := vision.NewColorFromColorful(palette[v-1]).C
		dc.Set(x, y, myColor)
	}

	return dc, nil

}
