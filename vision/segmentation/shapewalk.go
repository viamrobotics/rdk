package segmentation

import (
	"fmt"
	"image"

	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	ColorThreshold = 1.0
)

type walkState struct {
	img   vision.Image
	dots  map[string]int
	debug bool
}

func (ws *walkState) help(originalColor vision.HSV, lastColor vision.HSV, start image.Point, colorNumber int) {
	if start.X < 0 || start.X >= ws.img.Width() || start.Y < 0 || start.Y >= ws.img.Height() {
		return
	}

	key := fmt.Sprintf("%d-%d", start.X, start.Y)
	if ws.dots[key] != 0 {
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
		ws.dots[key] = -1
		return
	}
	ws.dots[key] = colorNumber

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
	goImg, err := img.ToImage()
	if err != nil {
		return nil, err
	}
	dc := gg.NewContextForImage(goImg)

	start := image.Point{startX, startY}

	ws := walkState{
		img:   img,
		dots:  map[string]int{},
		debug: debug,
	}

	if err := ws.piece(start, 1); err != nil {
		return nil, err
	}

	for k, v := range ws.dots {
		if v < 0 {
			continue
		}

		var x, y int
		_, err := fmt.Sscanf(k, "%d-%d", &x, &y)
		if err != nil {
			return nil, fmt.Errorf("couldn't read key %s %s", k, err)
		}

		dc.DrawCircle(float64(x), float64(y), 1)
		dc.SetColor(vision.Red.C)
		dc.Fill()
	}

	return dc.Image(), nil
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
		dots:  map[string]int{},
		debug: debug,
	}

	palette := colorful.FastWarmPalette(1000)

	for color := 0; color < len(palette); color++ {

		found := vision.Walk(img.Width()/2, img.Height()/2, img.Width(),
			func(x, y int) error {
				if x < 0 || x >= img.Width() || y < 0 || y >= img.Height() {
					return nil
				}

				key := fmt.Sprintf("%d-%d", x, y)
				if ws.dots[key] != 0 {
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

	goImg, err := img.ToImage()
	if err != nil {
		return nil, err
	}
	dc := gg.NewContextForImage(goImg)

	for k, v := range ws.dots {
		if v <= 0 {
			continue
		}

		var x, y int
		_, err := fmt.Sscanf(k, "%d-%d", &x, &y)
		if err != nil {
			return nil, fmt.Errorf("couldn't read key %s %s", k, err)
		}

		myColor := vision.NewColorFromColorful(palette[v-1]).C
		dc.DrawCircle(float64(x), float64(y), 1)
		dc.SetColor(myColor)
		dc.Fill()
	}

	return dc.Image(), nil

}
