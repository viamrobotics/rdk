package main

import (
	"flag"
	"fmt"
	"image"
	"sort"

	"github.com/gonum/stat"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
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
	for x := 0; x < img.Width(); x = x + 1 {
		for y := 0; y < img.Height(); y = y + 1 {
			p := image.Point{x, y}
			h, c, l := img.ColorHCL(p)
			H = append(H, h)
			C = append(C, c)
			L = append(L, l)
		}
	}

	_hclHistogramHelp("h", H)
	_hclHistogramHelp("c", C)
	_hclHistogramHelp("l", L)
}

func shapeWalk(img vision.Image, startX, startY int) error {

	for i := 0; i < 100; i++ {
		p := image.Point{i + startX, startY}
		h, c, l := img.ColorHCL(p)

		fmt.Printf("%v %v %v %v\n", img.Color(p), h, c, l)

	}

	m := img.MatUnsafe()
	gocv.Circle(&m, image.Point{startX, startY}, 1, vision.Red.C, 1)
	gocv.IMWrite("/tmp/x.png", m)

	return nil
}

func main() {

	xFlag := flag.Int("x", -1, "")
	yFlag := flag.Int("y", -1, "")

	flag.Parse()

	if flag.NArg() < 2 {
		panic(fmt.Errorf("need two args <program> <filename>"))
		return
	}

	prog := flag.Arg(0)
	fn := flag.Arg(1)

	img, err := vision.NewImageFromFile(fn)
	if err != nil {
		panic(err)
	}

	switch prog {
	case "hclHisto":
		hclHistogram(img)
	case "shapeWalk":
		err := shapeWalk(img, *xFlag, *yFlag)
		if err != nil {
			panic(err)
		}
	default:
		panic(fmt.Errorf("unknown program: %s", prog))
	}

}
