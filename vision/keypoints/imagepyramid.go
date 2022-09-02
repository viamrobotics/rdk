package keypoints

import (
	"image"
	"math"

	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
)

// ImagePyramid contains a slice of an image and its downscaled images as well as
// the corresponding scales wrt the original image.
type ImagePyramid struct {
	Images []*image.Gray
	Scales []int
}

// downscaleNearestGrayImage downscales an image.Gray by factor.
func downscaleNearestGrayImage(img *image.Gray, factor float64) (*image.Gray, error) {
	if img == nil {
		return nil, errors.New("input image is nil")
	}
	imgRect := img.Bounds()
	newRect := image.Rectangle{
		image.Point{0, 0},
		image.Point{
			int(float64(imgRect.Max.X) / factor),
			int(float64(imgRect.Max.Y) / factor),
		},
	}
	downsized := image.NewGray(newRect)
	utils.ParallelForEachPixel(newRect.Max, func(x, y int) {
		origXTemp := float64(x) * factor
		var origX int
		// round original float coordinates to the closest int coordinates
		if fraction := origXTemp - float64(int(origXTemp)); fraction >= 0.5 {
			origX = int(origXTemp + 1)
		} else {
			origX = int(origXTemp)
		}
		origYTemp := float64(y) * factor
		var origY int
		if fraction := origYTemp - float64(int(origYTemp)); fraction >= 0.5 {
			origY = int(origYTemp + 1)
		} else {
			origY = int(origYTemp)
		}
		downsized.SetGray(x, y, img.GrayAt(origX, origY))
	})
	return downsized, nil
}

// GetNumberOctaves returns the number of scales (or Octaves) in the Pyramid computed from the image size.
func GetNumberOctaves(imgSize image.Point) int {
	approxNumber := math.Log(math.Min(float64(imgSize.X), float64(imgSize.Y)))/math.Log(2.) - 1
	// handle the case where image is too small to be downscaled
	if approxNumber < 0 {
		approxNumber = 1.
	}

	return int(math.Round(approxNumber))
}

// GetImagePyramid return the images in the pyramid as well as the scales corresponding to each image in the
// ImagePyramid struct.
func GetImagePyramid(img *image.Gray) (*ImagePyramid, error) {
	imgSize := img.Bounds().Max
	// compute number of scales
	nOctaves := GetNumberOctaves(imgSize)
	// create image and scale slices
	scales := make([]int, nOctaves)
	images := make([]*image.Gray, nOctaves)
	// set first element
	images[0] = img
	scales[0] = 1
	for i := 1; i < nOctaves; i++ {
		im := images[i-1]
		downsized, err := downscaleNearestGrayImage(im, 2.)
		if err != nil {
			return nil, err
		}
		images[i] = downsized
		scales[i] = int(math.Pow(2, float64(i)))
	}
	pyramid := ImagePyramid{
		Images: images,
		Scales: scales,
	}
	return &pyramid, nil
}
