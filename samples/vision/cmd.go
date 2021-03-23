package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rlog"
	"go.viam.com/robotcore/vision/segmentation"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/fogleman/gg"
)

var (
	xFlag, yFlag *int
	debug        *bool
	logger       = rlog.Logger.Named("vision")
)

func _getOutputfile() string {
	if flag.NArg() < 3 {
		panic("need to specify output file")
	}
	return flag.Arg(2)
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
		logger.Debugf("%v %v %v\n", p, hsv, diff)

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
	remoteView, err := gostream.NewView(x264.DefaultViewConfig)
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

		walked, err := segmentation.ShapeWalk(img, p, segmentation.ShapeWalkOptions{Debug: *debug}, logger)
		if err != nil {
			panic(err)
		}

		dc := gg.NewContextForImage(walked)
		rimage.DrawString(dc, text, image.Point{0, 20}, rimage.White, 16)
		imgs[0] = dc.Image()
	})

	server := gostream.NewViewServer(5555, remoteView, logger)
	if err := server.Start(); err != nil {
		logger.Fatal(err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		time.Sleep(2 * time.Second)
		gostream.StreamSource(
			cancelCtx,
			gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, func(), error) { return imgs[0], func() {}, nil }),
			remoteView)
	}()

	<-c
	cancelFunc()
	remoteView.Stop()
	return nil
}

func main() {

	xFlag = flag.Int("x", -1, "")
	yFlag = flag.Int("y", -1, "")
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
	case "shapeWalkEntire":
		out, err := segmentation.ShapeWalkEntireDebug(img, segmentation.ShapeWalkOptions{Debug: *debug}, logger)
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
