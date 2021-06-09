package main

import (
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

func main() {
	err := realMain(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}
}

func SaveImageAsPng(img image.Image, filename string) {
	path := ""
	f, err := os.Create(path + filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

// Extract planes from an image with depth.
func ExtractPlanes(imgWd *rimage.ImageWithDepth) (*segmentation.SegmentedImage, error) {
	// Set camera matrices in image-with-depth
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
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

	return segImage, nil
}

func realMain(args []string) error {
	if len(args) == 0 {
		return errors.New("need to specify a rosbag file path")
	}
	filename := args[0]
	rb, err := ros.ReadBag(filename)
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
				if err == io.EOF {
					break
				}
				return err
			}
			json.Unmarshal(measurement[:len(measurement)-1], &message)

			// Create & display image
			imgWd, err := rimage.ReadBothFromBytes(message.Data.Data, true)
			if err != nil {
				return err
			}
			imgNrgba := imgWd.Overlay()
			SaveImageAsPng(imgNrgba, "img_"+fmt.Sprint(count)+".png")

			// Apply plane segmentation on image
			segImg, err := ExtractPlanes(imgWd)
			SaveImageAsPng(segImg, "seg_img_"+fmt.Sprint(count)+".png")

			count += 1
		}
	}
	return nil
}
