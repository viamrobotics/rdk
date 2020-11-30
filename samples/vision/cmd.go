package main

import (
	"flag"
	"fmt"
	"image"
	"sort"

	"github.com/gonum/stat"

	"gocv.io/x/gocv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"
	//"fyne.io/fyne/widget"

	"github.com/echolabsinc/robotcore/vision"
)

var (
	xFlag, yFlag *int
	radius       *float64
	maxDistance  *float64
)

func _hclHistogramHelp(name string, data []float64) {
	sort.Float64s(data)
	mean, stdDev := stat.MeanStdDev(data, nil)
	fmt.Printf("%s: mean: %f stdDev: %f min: %f max: %f\n", name, mean, stdDev, data[0], data[len(data)-1])
}

func hclHistogram(img vision.Image) {

	H := []float64{}
	C := []float64{}
	L := []float64{}

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
			hcl := img.ColorHCL(p)
			H = append(H, hcl.H)
			C = append(C, hcl.C)
			L = append(L, hcl.L)
		}
	}

	if center.X > 0 {
		m := img.MatUnsafe()
		gocv.Circle(&m, center, int(*radius), vision.Red.C, 1)
		gocv.IMWrite(flag.Arg(2), m)
	}

	_hclHistogramHelp("h", H)
	_hclHistogramHelp("c", C)
	_hclHistogramHelp("l", L)
}

func shapeWalkLine(img vision.Image, startX, startY int) error {
	m := img.MatUnsafe()

	init := img.ColorHCL(image.Point{startX, startY})

	mod := 0
	as := []image.Point{}
	bs := []image.Point{}

	for i := 0; i < 1000; i++ {
		p := image.Point{i + startX, startY}
		if p.X >= img.Width() {
			break
		}
		hcl := img.ColorHCL(p)

		diff := init.Distance(hcl)
		fmt.Printf("%v %v %v\n", p, hcl, diff)

		if diff > 12 {
			init = hcl
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

	gocv.IMWrite("/tmp/x.png", m)

	return nil
}

func _shapeWalkHelp(img vision.Image, dots map[string]int, clr vision.HCL, start image.Point) {
	if start.X < 0 || start.X >= img.Width() || start.Y < 0 || start.Y >= img.Height() {
		return
	}

	key := fmt.Sprintf("%d-%d", start.X, start.Y)
	if dots[key] != 0 {
		return
	}

	myColor := img.ColorHCL(start)
	if clr.Distance(myColor) > *maxDistance {
		dots[key] = -1
		return
	}
	dots[key] = 1

	// TODO: should i change clr to myColor ??
	clr = myColor

	_shapeWalkHelp(img, dots, clr, image.Point{start.X + 1, start.Y - 1})
	_shapeWalkHelp(img, dots, clr, image.Point{start.X + 1, start.Y + 0})
	_shapeWalkHelp(img, dots, clr, image.Point{start.X + 1, start.Y + 1})

	_shapeWalkHelp(img, dots, clr, image.Point{start.X - 1, start.Y - 1})
	_shapeWalkHelp(img, dots, clr, image.Point{start.X - 1, start.Y + 0})
	_shapeWalkHelp(img, dots, clr, image.Point{start.X - 1, start.Y + 1})

	_shapeWalkHelp(img, dots, clr, image.Point{start.X + 0, start.Y + 1})
	_shapeWalkHelp(img, dots, clr, image.Point{start.X + 0, start.Y + 1})
}

func shapeWalk(img vision.Image, startX, startY int) error {
	m := img.MatUnsafe()

	start := image.Point{startX, startY}
	init := img.ColorHCL(start)

	dots := map[string]int{} // 0 not seen, 1 seen and good, -1 seen and bad

	_shapeWalkHelp(img, dots, init, start)

	for k, v := range dots {
		if v != 1 {
			continue
		}

		var x, y int
		_, err := fmt.Sscanf(k, "%d-%d", &x, &y)
		if err != nil {
			return fmt.Errorf("couldn't read key %s %s", k, err)
		}

		gocv.Circle(&m, image.Point{x, y}, 1, vision.Red.C, 1)
	}

	gocv.IMWrite("/tmp/x.png", m)

	return nil
}

type myHover struct {
	fyne.CanvasObject
	img  vision.Image
	last image.Point
}

func (h *myHover) MouseIn(e *desktop.MouseEvent) {
	fmt.Printf("MouseIn: %v\n", e)
}

func (h *myHover) MouseMoved(e *desktop.MouseEvent) {
	p := image.Point{e.Position.X, e.Position.Y}
	if p.X == h.last.X && p.Y == h.last.Y {
		return
	}
	h.last = p
	fmt.Printf("MouseEvent: %v %v\n", e.Position, h.img.ColorHCL(p))
}

func (h *myHover) MouseOut() {
	fmt.Printf("MouseOut \n")
}

func view(img vision.Image) error {
	a := app.New()
	w := a.NewWindow("Hello")

	mat := img.MatUnsafe()
	i, err := mat.ToImage()
	if err != nil {
		return err
	}

	i2 := canvas.NewImageFromImage(i)
	i2.SetMinSize(fyne.Size{img.Width(), img.Height()})
	w.SetContent(i2)

	w.Canvas().Overlays().Add(&myHover{i2, img, image.Point{}})

	w.ShowAndRun()

	return nil

}

func main() {

	xFlag = flag.Int("x", -1, "")
	yFlag = flag.Int("y", -1, "")
	radius = flag.Float64("radius", -1, "")
	maxDistance = flag.Float64("maxDistance", 2.74, "")

	blur := flag.Bool("blur", false, "")
	blurSize := flag.Int("blurSize", 5, "")

	flag.Parse()

	if flag.NArg() < 2 {
		panic(fmt.Errorf("need two args <program> <filename>"))
		return
	}

	prog := flag.Arg(0)
	fn := flag.Arg(1)

	img, err := vision.NewImageFromFile(fn)
	if err != nil {
		panic(fmt.Errorf("error reading image from file (%s) %w", fn, err))
	}

	if *blur {
		m := img.MatUnsafe()
		gocv.Blur(m, &m, image.Point{*blurSize, *blurSize})
	}

	switch prog {
	case "hclHisto":
		hclHistogram(img)
	case "shapeWalk":
		err = shapeWalk(img, *xFlag, *yFlag)
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
