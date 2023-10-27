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
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/ros"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
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

// extractPlanes extract planes from an image and depth map.
func extractPlanes(ctx context.Context, img *rimage.Image, dm *rimage.DepthMap) (*segmentation.SegmentedImage, error) {
	// Set camera matrices in image-with-depth
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("ros/data/intel515_parameters.json"))
	if err != nil {
		return nil, err
	}

	// Get the pointcloud from the image-with-depth
	pcl, err := camera.RGBDToPointCloud(img, dm)
	if err != nil {
		return nil, err
	}

	// Extract the planes from the point cloud
	planeSeg := segmentation.NewPointCloudPlaneSegmentation(pcl, 50, 150000)
	planes, _, err := planeSeg.FindPlanes(ctx)
	if err != nil {
		return nil, err
	}

	// Project the pointcloud planes into an image
	segments := make([]pointcloud.PointCloud, 0, len(planes))
	for _, plane := range planes {
		cloud, err := plane.PointCloud()
		if err != nil {
			return nil, err
		}
		segments = append(segments, cloud)
	}
	segImage, err := segmentation.PointCloudSegmentsToMask(camera.ColorCamera, segments)
	if err != nil {
		return nil, err
	}

	return segImage, nil
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

			// Apply plane segmentation on image
			segImg, err := extractPlanes(ctx, img, dm)
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
