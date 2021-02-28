package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"github.com/fogleman/gg"
	"github.com/gonum/stat"
)

var (
	xFlag, yFlag *int
	radius       *float64
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
	golog.Global.Debugf("%s: mean: %f stdDev: %f min: %f max: %f\n", name, mean, stdDev, data[0], data[len(data)-1])
}

func hsvHistogram(img *rimage.Image) {

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
			if center.X >= 0 && rimage.PointDistance(center, p) > *radius {
				continue
			}
			hsv := img.Get(p)
			H = append(H, hsv.H)
			S = append(S, hsv.S)
			V = append(V, hsv.V)
		}
	}

	if center.X > 0 {
		dc := gg.NewContextForImage(img)
		dc.DrawCircle(float64(center.X), float64(center.Y), *radius)
		dc.SetColor(rimage.Red)
		dc.Fill()
		rimage.WriteImageToFile(_getOutputfile(), dc.Image())
	}

	_hsvHistogramHelp("h", H)
	_hsvHistogramHelp("s", S)
	_hsvHistogramHelp("v", V)
}

func shapeWalkLine(img *rimage.Image, startX, startY int) error {
	init := img.Get(image.Point{startX, startY})

	mod := 0
	as := []image.Point{}
	bs := []image.Point{}

	for i := 0; i < 1000; i++ {
		p := image.Point{i + startX, startY}
		if p.X >= img.Width() {
			break
		}
		hsv := img.Get(p)

		diff := init.Distance(hsv)
		golog.Global.Debugf("%v %v %v\n", p, hsv, diff)

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

	dc := gg.NewContextForImage(img)
	for _, p := range as {
		dc.DrawCircle(float64(p.X), float64(p.Y), 1)
		dc.SetColor(rimage.Red)
		dc.Fill()
	}

	for _, p := range bs {
		dc.DrawCircle(float64(p.X), float64(p.Y), 1)
		dc.SetColor(rimage.Green)
		dc.Fill()
	}

	rimage.WriteImageToFile(_getOutputfile(), dc.Image())

	return nil
}

func view(img *rimage.Image) error {
	remoteView, err := gostream.NewRemoteView(vpx.DefaultRemoteViewConfig)
	if err != nil {
		return err
	}

	var last image.Point

	imgs := []image.Image{img}

	remoteView.SetOnClickHandler(func(x, y int) {
		if x < 0 || y < 0 {
			return
		}
		p := image.Point{x, y}
		if p.X == last.X && p.Y == last.Y {
			return
		}
		last = p
		color := img.Get(p)
		text := fmt.Sprintf("(x, y): (%d, %d); %s",
			x, y,
			color.String())

		walked, err := segmentation.ShapeWalk(img, p, segmentation.ShapeWalkOptions{Debug: *debug})
		if err != nil {
			panic(err)
		}

		dc := gg.NewContextForImage(walked)
		rimage.DrawString(dc, text, image.Point{0, 20}, rimage.White, 16)
		imgs[0] = dc.Image()
	})

	server := gostream.NewRemoteViewServer(5555, remoteView, golog.Global)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go gostream.StreamFuncOnce(
		cancelCtx,
		func() { time.Sleep(2 * time.Second) },
		func(ctx context.Context) (image.Image, error) { return imgs[0], nil },
		remoteView)

	<-c
	cancelFunc()
	remoteView.Stop()
	return nil
}

func main() {

	xFlag = flag.Int("x", -1, "")
	yFlag = flag.Int("y", -1, "")
	radius = flag.Float64("radius", -1, "")
	debug = flag.Bool("debug", false, "")

	blur := flag.Bool("blur", false, "")
	blurSize := flag.Int("blurSize", 5, "")

	flag.Parse()

	if flag.NArg() < 2 {
		panic(fmt.Errorf("need two args <program> <filename>"))
	}

	prog := flag.Arg(0)
	fn := flag.Arg(1)

	img, err := rimage.NewImageFromFile(fn)
	if err != nil {
		panic(fmt.Errorf("error reading image from file (%s) %w", fn, err))
	}

	if *blur {
		newImg := imaging.Blur(img, float64(*blurSize))
		img = rimage.ConvertImage(newImg)
	}

	switch prog {
	case "hsvHisto":
		hsvHistogram(img)
	case "shapeWalkEntire":
		out, err := segmentation.ShapeWalkEntireDebug(img, segmentation.ShapeWalkOptions{Debug: *debug})
		if err == nil {
			err = rimage.WriteImageToFile(_getOutputfile(), out)
			if err != nil {
				panic(err)
			}
		}
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
