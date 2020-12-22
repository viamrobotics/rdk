package main

import (
	//"context"
	"context"
	"flag"
	"fmt"
	"image"
	"sort"
	"time"

	"github.com/echolabsinc/robotcore/utils/log"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	"fyne.io/fyne/test"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"

	"github.com/gonum/stat"
	"github.com/lucasb-eyer/go-colorful"
	"gocv.io/x/gocv"
)

var (
	xFlag, yFlag *int
	radius       *float64
	maxDistance  *float64
	debug        *bool
)

func _getOutputfile() string {
	if flag.NArg() < 3 {
		panic("need to specify output file")
	}
	return flag.Arg(2)
}

func _hsvHistogramHelp(name string, data []float64) {
	sort.Float64s(data)
	mean, stdDev := stat.MeanStdDev(data, nil)
	log.Global.Debugf("%s: mean: %f stdDev: %f min: %f max: %f\n", name, mean, stdDev, data[0], data[len(data)-1])
}

func hsvHistogram(img vision.Image) {

	H := []float64{}
	S := []float64{}
	V := []float64{}

	center := image.Point{-1, -1}
	if *xFlag >= 0 && *yFlag >= 0 && *radius > 0 {
		center = image.Point{*xFlag, *yFlag}
	}

	for x := 0; x < img.Width(); x = x + 1 {
		for y := 0; y < img.Height(); y = y + 1 {
			p := image.Point{x, y}
			if center.X >= 0 && vision.PointDistance(center, p) > *radius {
				continue
			}
			hsv := img.ColorHSV(p)
			H = append(H, hsv.H)
			S = append(S, hsv.S)
			V = append(V, hsv.V)
		}
	}

	if center.X > 0 {
		m := img.MatUnsafe()
		gocv.Circle(&m, center, int(*radius), vision.Red.C, 1)
		gocv.IMWrite(_getOutputfile(), m)
	}

	_hsvHistogramHelp("h", H)
	_hsvHistogramHelp("s", S)
	_hsvHistogramHelp("v", V)
}

func shapeWalkLine(img vision.Image, startX, startY int) error {
	m := img.MatUnsafe()

	init := img.ColorHSV(image.Point{startX, startY})

	mod := 0
	as := []image.Point{}
	bs := []image.Point{}

	for i := 0; i < 1000; i++ {
		p := image.Point{i + startX, startY}
		if p.X >= img.Width() {
			break
		}
		hsv := img.ColorHSV(p)

		diff := init.Distance(hsv)
		log.Global.Debugf("%v %v %v\n", p, hsv, diff)

		if diff > 12 {
			init = hsv
			mod = mod + 1
		}

		if mod%2 == 0 {
			as = append(as, p)
		} else {
			bs = append(bs, p)
		}
	}

	for _, p := range as {
		gocv.Circle(&m, p, 1, vision.Red.C, 1)
	}

	for _, p := range bs {
		gocv.Circle(&m, p, 1, vision.Green.C, 1)
	}

	gocv.IMWrite(_getOutputfile(), m)

	return nil
}

func _shapeWalkHelp(img vision.Image, dots map[string]int, originalColor vision.HSV, lastColor vision.HSV, start image.Point, colorNumber int) {
	if start.X < 0 || start.X >= img.Width() || start.Y < 0 || start.Y >= img.Height() {
		return
	}

	if *xFlag >= 0 && *yFlag >= 0 && *radius > 0 {
		center := image.Point{*xFlag, *yFlag}
		if vision.PointDistance(center, start) > *radius {
			return
		}

	}

	key := fmt.Sprintf("%d-%d", start.X, start.Y)
	if dots[key] != 0 {
		return
	}

	myColor := img.ColorHSV(start)

	originalDistance := originalColor.Distance(myColor)
	lastDistance := lastColor.Distance(myColor)

	good := originalDistance < *maxDistance || (originalDistance < (*maxDistance*1.1) && lastDistance < *maxDistance/1)

	if *debug {
		distanceFromPoint := vision.PointDistance(start, image.Point{*xFlag, *yFlag})
		log.Global.Debugf("good: %v originalColor: %s point: %v myColor: %s originalDistance: %v lastDistance: %v distanceFromPoint: %f\n",
			good, originalColor.ToColorful().Hex(), start, myColor.ToColorful().Hex(), originalDistance, lastDistance, distanceFromPoint)
	}
	if !good {
		dots[key] = -1
		return
	}
	dots[key] = colorNumber

	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X + 1, start.Y - 1}, colorNumber)
	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X + 1, start.Y + 0}, colorNumber)
	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X + 1, start.Y + 1}, colorNumber)

	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X - 1, start.Y - 1}, colorNumber)
	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X - 1, start.Y + 0}, colorNumber)
	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X - 1, start.Y + 1}, colorNumber)

	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X + 0, start.Y + 1}, colorNumber)
	_shapeWalkHelp(img, dots, originalColor, myColor, image.Point{start.X + 0, start.Y + 1}, colorNumber)
}

func shapeWalkPiece(img vision.Image, start image.Point, dots map[string]int, colorNumber int) error {
	log.Global.Debug("shapeWalkPiece")
	init := img.ColorHSV(start)

	_shapeWalkHelp(img, dots, init, init, start, colorNumber)

	for k, v := range dots {
		if v == -1 {
			dots[k] = 0
		}
	}

	return nil
}

func shapeWalk(img vision.Image, startX, startY int) error {
	m := img.MatUnsafe()

	start := image.Point{startX, startY}

	dots := map[string]int{} // 0 not seen, 1 seen and good, -1 seen and bad

	shapeWalkPiece(img, start, dots, 1)

	for k, v := range dots {
		if v < 0 {
			continue
		}

		var x, y int
		_, err := fmt.Sscanf(k, "%d-%d", &x, &y)
		if err != nil {
			return fmt.Errorf("couldn't read key %s %s", k, err)
		}

		gocv.Circle(&m, image.Point{x, y}, 1, vision.Red.C, 1)
	}

	if *xFlag >= 0 && *yFlag >= 0 && *radius > 0 {
		center := image.Point{*xFlag, *yFlag}
		gocv.Circle(&m, center, int(*radius), vision.Green.C, 1)
	}

	gocv.IMWrite(_getOutputfile(), m)

	return nil
}

type MyWalkError struct {
	pos image.Point
}

func (e MyWalkError) Error() string {
	return "MyWalkError"
}

func shapeWalkEntire(img vision.Image) error {
	m := img.MatUnsafe()

	palette := colorful.FastWarmPalette(10)
	dots := map[string]int{}

	for color := 0; color < len(palette); color++ {

		found := vision.Walk(img.Width()/2, img.Height()/2, img.Width(),
			func(x, y int) error {
				if x < 0 || x >= img.Width() || y < 0 || y >= img.Height() {
					return nil
				}

				key := fmt.Sprintf("%d-%d", x, y)
				if dots[key] != 0 {
					return nil
				}
				return MyWalkError{image.Point{x, y}}
			})

		if found == nil {
			break
		}

		start := found.(MyWalkError).pos
		shapeWalkPiece(img, start, dots, color+1)
	}

	for k, v := range dots {
		if v <= 0 {
			continue
		}

		var x, y int
		_, err := fmt.Sscanf(k, "%d-%d", &x, &y)
		if err != nil {
			return fmt.Errorf("couldn't read key %s %s", k, err)
		}

		myColor := vision.NewColorFromColorful(palette[v-1]).C
		gocv.Circle(&m, image.Point{x, y}, 1, myColor, 1)
	}

	gocv.IMWrite(_getOutputfile(), m)

	return nil
}

type colorHover struct {
	fyne.CanvasObject
	img      vision.Image
	last     image.Point
	textGrid *widget.TextGrid
}

func (ch *colorHover) MouseIn(e *desktop.MouseEvent) {}

func (ch *colorHover) MouseMoved(e *desktop.MouseEvent) {
	p := image.Point{e.Position.X, e.Position.Y}
	if p.X == ch.last.X && p.Y == ch.last.Y {
		return
	}
	ch.last = p
	color := ch.img.Color(p)
	colorHSV := ch.img.ColorHSV(p)
	ch.textGrid.SetText(fmt.Sprintf("(x, y): (%d, %d)\nHSV: (%f, %f, %f)\nRGBA: (%d, %d, %d, %d)\n",
		e.Position.X, e.Position.Y,
		colorHSV.H, colorHSV.S, colorHSV.V,
		color.R, color.G, color.B, color.A))
}
func (ch *colorHover) MouseOut() {}

type ViewApp struct {
	fyne.App
	colorHoverElem *colorHover
	mainWindow     fyne.Window
}

func newViewApp(img vision.Image) (*ViewApp, error) {
	a := test.NewApp()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Hello")

	mat := img.MatUnsafe()
	i, err := mat.ToImage()
	if err != nil {
		return nil, err
	}

	i2 := canvas.NewImageFromImage(i)
	i2.SetMinSize(fyne.Size{img.Width(), img.Height()})
	w.SetContent(i2)

	info := widget.NewTextGridFromString("?")
	w.Canvas().Overlays().Add(info)
	colorHoverElem := &colorHover{i2, img, image.Point{}, info}
	w.Canvas().Overlays().Add(colorHoverElem)

	return &ViewApp{
		App:            a,
		colorHoverElem: colorHoverElem,
		mainWindow:     w,
	}, nil
}

func view(img vision.Image) error {
	remoteView, err := stream.NewRemoteView(stream.DefaultRemoteViewConfig)
	if err != nil {
		return err
	}

	app, err := newViewApp(img)
	if err != nil {
		return err
	}
	remoteView.SetOnClickHandler(func(x, y int) {
		app.colorHoverElem.MouseMoved(&desktop.MouseEvent{
			PointEvent: fyne.PointEvent{
				Position: fyne.Position{X: x / 2, Y: y / 2}, // TODO(erd): way to do this with no downscale?
			},
		})
	})

	server := stream.NewRemoteViewServer(5555, remoteView, log.Global)
	if err := server.Run(context.Background()); err != nil {
		panic(err)
	}

	go stream.StreamWindow(app.mainWindow, remoteView, 250*time.Millisecond)
	app.mainWindow.ShowAndRun()

	// TODO(erd): some defer to stop everything and clean up
	select {}
}

func main() {

	xFlag = flag.Int("x", -1, "")
	yFlag = flag.Int("y", -1, "")
	radius = flag.Float64("radius", -1, "")
	maxDistance = flag.Float64("maxDistance", 1.0, "")
	debug = flag.Bool("debug", false, "")

	blur := flag.Bool("blur", false, "")
	blurSize := flag.Int("blurSize", 5, "")

	flag.Parse()

	if flag.NArg() < 2 {
		panic(fmt.Errorf("need two args <program> <filename>"))
	}

	prog := flag.Arg(0)
	fn := flag.Arg(1)

	img, err := vision.NewImageFromFile(fn)
	if err != nil {
		panic(fmt.Errorf("error reading image from file (%s) %w", fn, err))
	}

	if *blur {
		m := img.MatUnsafe()
		gocv.GaussianBlur(m, &m, image.Point{*blurSize, *blurSize}, 0, 0, gocv.BorderDefault)
	}

	switch prog {
	case "hsvHisto":
		hsvHistogram(img)
	case "shapeWalk":
		// TODO(erd): view version
		err = shapeWalk(img, *xFlag, *yFlag)
	case "shapeWalkEntire":
		err = shapeWalkEntire(img)
	case "shapeWalkLine":
		err = shapeWalkLine(img, *xFlag, *yFlag)
	case "view":
		err = view(img)
	default:
		panic(fmt.Errorf("unknown program: %s", prog))
	}

	if err != nil {
		panic(fmt.Errorf("error running command: %w", err))
	}

}
