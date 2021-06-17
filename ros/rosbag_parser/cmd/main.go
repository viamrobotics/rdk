package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"

	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/ros"
	"go.viam.com/core/utils"
	"go.viam.com/core/vision/segmentation"
)

var logger = golog.NewDevelopmentLogger("rosbag_parser")

// Arguments for the rosbag parser
type Arguments struct {
	RosbagFile string `flag:"0"`
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// saveImageAsPng saves image as png in current directory
func saveImageAsPng(img image.Image, filename string) error {
	path := ""
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

// extractPlanes extract planes from an image with depth.
func extractPlanes(imgWd *rimage.ImageWithDepth) (*segmentation.SegmentedImage, error) {
	// Set camera matrices in image-with-depth
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	if err != nil {
		return nil, err
	}
	imgWd.SetCameraSystem(camera)

	// Get the pointcloud from the image-with-depth
	pcl, err := imgWd.ToPointCloud()
	if err != nil {
		return nil, err
	}

	// Extract the planes from the point cloud
	planes, _, err := segmentation.GetPlanesInPointCloud(pcl, 50, 150000)
	if err != nil {
		return nil, err
	}

	// Project the pointcloud planes into an image
	segImage, err := segmentation.PointCloudSegmentsToMask(camera.ColorCamera, planes)
	if err != nil {
		return nil, err
	}

	return segImage, nil
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	rb, err := ros.ReadBag(argsParsed.RosbagFile, logger)
	if err != nil {
		return err
	}

	topics := []string{"/L515_ImageWithDepth"}
	err = ros.WriteTopicsJSON(rb, 0, 0, topics, logger)
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
			imgWd, err := rimage.ReadBothFromBytes(message.Data.Data, true)
			if err != nil {
				return err
			}
			imgNrgba := imgWd.Overlay()
			err = saveImageAsPng(imgNrgba, "img_"+fmt.Sprint(count)+".png")
			if err != nil {
				return err
			}

			// Apply plane segmentation on image
			segImg, err := extractPlanes(imgWd)
			if err != nil {
				return err
			}
			err = saveImageAsPng(segImg, "seg_img_"+fmt.Sprint(count)+".png")
			if err != nil {
				return err
			}

			count++
		}
	}
	return nil
}
