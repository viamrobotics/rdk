// Package main is a rosbag parser.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/ros"
)

var logger = logging.NewDebugLogger("rosbag_parser")

// Arguments for the rosbag parser.
type Arguments struct {
	RosbagFile string `flag:"0"`
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

// saveImageAsPng saves image as png in current directory.
func saveImageAsPng(img image.Image, filename string) error {
	path := ""
	//nolint:gosec
	f, err := os.Create(path + filename)
	if err != nil {
		return err
	}
	err = png.Encode(f, img)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	return nil
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	var argsParsed Arguments

	if err := goutils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	rb, err := ros.ReadBag(argsParsed.RosbagFile)
	if err != nil {
		return err
	}

	topics := []string{"/L515_ImageWithDepth"}
	err = ros.WriteTopicsJSON(rb, 0, 0, topics)
	if err != nil {
		return err
	}

	var message ros.L515Message
	for _, v := range rb.TopicsAsJSON {
		count := 0
		for {
			// Read bytes into JSON structure
			measurement, err := v.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
			err = json.Unmarshal(measurement[:len(measurement)-1], &message)
			if err != nil {
				return err
			}

			// Create & display image
			img1, _, err := image.Decode(bytes.NewReader(message.ColorData.Data))
			if err != nil {
				return err
			}
			img2, _, err := image.Decode(bytes.NewReader(message.DepthData.Data))
			if err != nil {
				return err
			}
			img := rimage.ConvertImage(img1)
			dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
			if err != nil {
				return err
			}
			imgNrgba := rimage.Overlay(img, dm)
			err = saveImageAsPng(imgNrgba, "img_"+fmt.Sprint(count)+".png")
			if err != nil {
				return err
			}

			count++
		}
	}
	return nil
}
