// Package main provides a command for computer vision utilities.
package main

import (
	"context"
	"flag"
	"image"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/fogleman/gg"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/segmentation"
)

var (
	xFlag, yFlag *int
	debug        *bool
	logger       = golog.NewDevelopmentLogger("vision")
)

func _getOutputfile() string {
	if flag.NArg() < 3 {
		panic("need to specify output file")
	}
	return flag.Arg(2)
}

func shapeWalkLine(img *rimage.Image, startX, startY int) {
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
			mod++
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
}

func view(img *rimage.Image) error {
	remoteStream, err := gostream.NewStream(x264.DefaultStreamConfig)
	if err != nil {
		return err
	}

	imgs := []image.Image{img}
	server, err := gostream.NewStandaloneStreamServer(5555, logger, remoteStream)
	if err != nil {
		return err
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	if err := server.Start(cancelCtx); err != nil {
		logger.Fatal(err)
	}

	utils.PanicCapturingGo(func() {
		if !utils.SelectContextOrWait(cancelCtx, 2*time.Second) {
			return
		}
		gostream.StreamSource(
			cancelCtx,
			gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, func(), error) { return imgs[0], func() {}, nil }),
			remoteStream)
	})

	<-c
	cancelFunc()
	remoteStream.Stop()
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
		panic("need two args <program> <filename>")
	}

	prog := flag.Arg(0)
	fn := flag.Arg(1)

	img, err := rimage.NewImageFromFile(fn)
	if err != nil {
		panic(errors.Wrapf(err, "error reading image from file (%s)", fn))
	}

	if *blur {
		newImg := imaging.Blur(img, float64(*blurSize))
		img = rimage.ConvertImage(newImg)
	}

	switch prog {
	case "shapeWalkEntire":
		out, err := segmentation.ShapeWalkEntireDebug(rimage.ConvertToImageWithDepth(img), segmentation.ShapeWalkOptions{Debug: *debug}, logger)
		if err == nil {
			err = rimage.WriteImageToFile(_getOutputfile(), out)
			if err != nil {
				panic(err)
			}
		}
	case "shapeWalkLine":
		shapeWalkLine(img, *xFlag, *yFlag)
	case "view":
		err = view(img)
	default:
		panic(errors.Errorf("unknown program: %s", prog))
	}

	if err != nil {
		panic(errors.Wrap(err, "error running command"))
	}
}
