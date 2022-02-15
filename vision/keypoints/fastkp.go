package keypoints

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
)

// Implementation of FAST keypoint detector
// From paper :
// Rosten, Edward; Tom Drummond (2005). Fusing points and lines for high performance tracking.
// IEEE International Conference on Computer Vision. Vol. 2. pp. 1508–1511.
// Available at: http://www.edwardrosten.com/work/rosten_2005_tracking.pdf
// Resources:
// - https://medium.com/data-breach/introduction-to-fast-features-from-accelerated-segment-test-4ed33dde6d65
// - https://homepages.inf.ed.ac.uk/rbf/CVonline/LOCAL_COPIES/AV1011/AV1FeaturefromAcceleratedSegmentTest.pdf

// FASTConfig holds the parameters necessary to compute the FAST keypoints.
type FASTConfig struct {
	NMatchesCircle int     `json:"n_matches"`
	NMSWinSize     int     `json:"nms_win_size"`
	Threshold      float64 `json:"threshold"`
	// NumScales      int     `json:"num_scales"`
}

var (
	// CrossIdx contains the neighbors coordinates in a 3-cross neighborhood.
	CrossIdx = []image.Point{{0, 3}, {3, 0}, {0, -3}, {-3, 0}}
	// CircleIdx contains the neighbors coordinates in a circle of radius 3 neighborhood.
	CircleIdx = []image.Point{
		{0, -3},
		{1, -3},
		{2, -2},
		{3, -1},
		{3, 0},
		{3, 1},
		{2, 2},
		{1, 3},
		{0, 3},
		{-1, 3},
		{-2, 2},
		{-3, 1},
		{-3, 0},
		{-3, -1},
		{-2, -2},
		{-1, -3},
	}
	logger = golog.NewLogger("fast_kp")
)

// LoadFASTConfiguration loads a FASTConfig from a json file.
func LoadFASTConfiguration(file string) (FASTConfig, error) {
	var config FASTConfig
	filePath := filepath.Clean(file)
	configFile, err := os.Open(filePath)
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return FASTConfig{}, err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return FASTConfig{}, err
	}
	return config, nil
}

// GetPointValuesInNeighborhood returns a slice of floats containing the values of neighborhood pixels in image img.
func GetPointValuesInNeighborhood(img *image.Gray, coords image.Point, neighborhood []image.Point) []float64 {
	vals := make([]float64, len(neighborhood))
	for i := 0; i < len(neighborhood); i++ {
		c := img.At(coords.X+neighborhood[i].X, coords.Y+neighborhood[i].Y).(color.Gray).Y
		vals[i] = float64(c)
	}
	return vals
}

// ComputeFAST computes the location of FAST keypoints.
func ComputeFAST(img *image.Gray, nMatchesCircle int, nmsWin int, threshold float64) (KeyPoints, error) {
	if nmsWin <= 0 {
		logger.Warn("NMS window size is negative. Setting it to 3.")
		nmsWin = 3
	}
	keypoints := make([]image.Point, 0)
	cornerImg := image.NewGray(img.Bounds())
	h, w := img.Bounds().Max.Y, img.Bounds().Max.X
	for y := 3; y < h-3; y++ {
		for x := 3; x < w-3; x++ {
			v := float64(img.At(x, y).(color.Gray).Y)
			t := threshold
			if t < 1. {
				t *= v
			}
			imMin := v - t
			imMax := v + t
			// count numbers of pixels in good range for fast check
			p := GetPointValuesInNeighborhood(img, image.Point{x, y}, CircleIdx)
			// check p1 and p9 first
			if (p[0] > imMax || p[0] < imMin) && (p[8] > imMax || p[8] < imMin) {
				// check p5 and 13
				if (p[4] > imMax || p[4] < imMin) && (p[12] > imMax || p[12] < imMin) {
					otherPoints := []float64{p[1], p[2], p[3], p[5], p[6], p[7], p[9], p[10], p[11], p[13], p[14], p[15]}
					count := 0
					for _, val := range otherPoints {
						if val > imMax || val < imMin {
							count++
						}
					}
					if count+4 >= nMatchesCircle {
						keypoints = append(keypoints, image.Point{x, y})
						cornerVal := 0.
						for _, vals := range otherPoints {
							cornerVal += math.Abs(v - vals)
						}
						cornerImg.Set(x, y, color.Gray{uint8(math.Round(cornerVal))})
					}
				}
			}
		}
	}
	// perform non-maximum suppression to remove redundant keypoints
	reducedKeypointsSet := make(map[image.Point]bool)
	reducedKeypoints := make([]image.Point, 0)
	for _, kp := range keypoints {
		// get pixel coordinates of maximum in window
		maxVal := 0
		maxPoint := image.Point{-nmsWin, -nmsWin}
		for nmsX := -nmsWin; nmsX < nmsWin+1; nmsX++ {
			for nmsY := -nmsWin; nmsY < nmsWin+1; nmsY++ {
				winPixel := image.Point{kp.X + nmsX, kp.Y + nmsY}
				winPixelVal := int(cornerImg.At(winPixel.X, winPixel.Y).(color.Gray).Y)
				if winPixelVal > maxVal {
					maxVal = winPixelVal
					maxPoint = winPixel
				}
			}
		}
		// get max keypoint coords in window
		kpNew := image.Point{maxPoint.X, maxPoint.Y}
		if _, ok := reducedKeypointsSet[kpNew]; !ok {
			reducedKeypoints = append(reducedKeypoints, kpNew)
			reducedKeypointsSet[kpNew] = true
		}
	}
	return reducedKeypoints, nil
}
