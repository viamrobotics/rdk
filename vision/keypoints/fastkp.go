package keypoints

import (
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"

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
	NMatchesCircle int  `json:"n_matches"`
	NMSWinSize     int  `json:"nms_win_size"`
	Threshold      int  `json:"threshold"`
	Oriented       bool `json:"oriented"`
	Radius         int  `json:"radius"`
}

// PixelType stores 0 if a pixel is darker than center pixel, and 1 if brighter.
type PixelType int

const (
	darker   PixelType = iota // 0
	brighter                  // 1
)

// FASTPixel stores coordinates of an image point in a neighborhood and its PixelType.
type FASTPixel struct {
	Point image.Point
	Type  PixelType
}

// FASTKeypoints stores keypoint locations and orientations (nil if not oriented).
type FASTKeypoints OrientedKeypoints

// NewFASTKeypointsFromImage returns a pointer to a FASTKeypoints struct containing keypoints locations and
// orientations if Oriented is set to true in the configuration.
func NewFASTKeypointsFromImage(img *image.Gray, cfg *FASTConfig) *FASTKeypoints {
	kps := ComputeFAST(img, cfg)
	var orientations []float64
	if cfg.Oriented {
		orientations = ComputeKeypointsOrientations(img, kps, cfg.Radius)
	}
	return &FASTKeypoints{
		kps,
		orientations,
	}
}

// IsOriented returns true if FASTKeypoints contains orientations.
func (kps *FASTKeypoints) IsOriented() bool {
	return !(kps.Orientations == nil)
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
)

// LoadFASTConfiguration loads a FASTConfig from a json file.
func LoadFASTConfiguration(file string) *FASTConfig {
	var config FASTConfig
	filePath := filepath.Clean(file)
	configFile, err := os.Open(filePath)
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return nil
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return nil
	}
	return &config
}

// GetPointValuesInNeighborhood returns a slice of floats containing the values of neighborhood pixels in image img.
func GetPointValuesInNeighborhood(img *image.Gray, coords image.Point, neighborhood []image.Point) []float64 {
	vals := make([]float64, len(neighborhood))
	for i := 0; i < len(neighborhood); i++ {
		c := img.GrayAt(coords.X+neighborhood[i].X, coords.Y+neighborhood[i].Y).Y
		vals[i] = float64(c)
	}
	return vals
}

func isValidSliceVals(vals []float64, n int) bool {
	cnt := 0
	for _, val := range vals {
		// if value is positive, increment count of consecutive darker or brighter pixels in neighborhood
		if val > 0 {
			cnt++
		} else {
			// otherwise, reset count
			cnt = 0
		}
		if cnt > n {
			return true
		}
	}
	return false
}

func sumOfPositiveValuesSlice(s []float64) float64 {
	sum := 0.
	for _, it := range s {
		if it > 0 {
			sum += it
		}
	}
	return sum
}

func sumOfNegativeValuesSlice(s []float64) float64 {
	sum := 0.
	for _, it := range s {
		if it < 0 {
			sum += it
		}
	}
	return sum
}

func computeNMSScore(img *image.Gray, pix FASTPixel) float64 {
	val := float64(img.GrayAt(pix.Point.X, pix.Point.Y).Y)
	circleValues := GetPointValuesInNeighborhood(img, pix.Point, CircleIdx)
	diffValues := make([]float64, len(circleValues))
	for i, v := range circleValues {
		diffValues[i] = v - val
	}
	if pix.Type == brighter {
		return sumOfPositiveValuesSlice(diffValues)
	}
	return -1. * sumOfNegativeValuesSlice(diffValues)
}

func canDeleteNMS(pix FASTPixel, pix2Score map[image.Point]float64, winSize int) bool {
	for dx := -winSize; dx < winSize+1; dx++ {
		for dy := -winSize; dy < winSize+1; dy++ {
			neighbor := image.Point{pix.Point.X + dx, pix.Point.Y + dy}
			if neighborScore, ok := pix2Score[neighbor]; ok && neighborScore > pix2Score[pix.Point] {
				return true
			}
		}
	}

	return false
}

// nonMaximumSuppression returns maximal keypoints in a window of size 2*winSize+1 around keypoints.
func nonMaximumSuppression(img *image.Gray, kps []FASTPixel, winSize int) KeyPoints {
	// compute score map
	pix2score := make(map[image.Point]float64)
	for _, kp := range kps {
		pix2score[kp.Point] = computeNMSScore(img, kp)
	}
	// initialize keypoints
	nmsKps := make([]image.Point, 0)
	for _, kp := range kps {
		if !canDeleteNMS(kp, pix2score, winSize) {
			nmsKps = append(nmsKps, kp.Point)
		}
	}
	return nmsKps
}

func computeAngle(img *image.Gray, kp image.Point, radius int, halfWidthMax []int) float64 {
	w, h := img.Bounds().Max.X, img.Bounds().Max.Y
	m10, m01 := 0, 0
	for y := -radius + 1; y < radius; y++ {
		if y+kp.Y < 0 || y+kp.Y >= h {
			continue
		}
		currentWidth := halfWidthMax[int(math.Abs(float64(y)))]
		for x := 0; x < currentWidth; x++ {
			if x+kp.X < w {
				m10 += x * int(img.GrayAt(x+kp.X, y+kp.Y).Y)
				m01 += y * int(img.GrayAt(x+kp.X, y+kp.Y).Y)
			}
		}
	}
	return math.Atan2(float64(m01), float64(m10))
}

// ComputeKeypointsOrientations compute keypoints orientations in image.
func ComputeKeypointsOrientations(img *image.Gray, kps KeyPoints, radius int) []float64 {
	halfWidthMax := make([]int, radius)
	for i := 0; i < radius; i++ {
		halfWidthMax[i] = int(math.Sqrt(float64(radius*radius - i*i)))
	}
	orientations := make([]float64, len(kps))
	for i, kp := range kps {
		orientations[i] = computeAngle(img, kp, radius, halfWidthMax)
	}
	return orientations
}

func getBrighterValues(s []float64, t float64) []float64 {
	brighterValues := make([]float64, len(s))
	for i, v := range s {
		if v > t {
			brighterValues[i] = 1
		} else {
			brighterValues[i] = 0
		}
	}
	return brighterValues
}

func getDarkerValues(s []float64, t float64) []float64 {
	darkerValues := make([]float64, len(s))
	for i, v := range s {
		if v < t {
			darkerValues[i] = 1
		} else {
			darkerValues[i] = 0
		}
	}
	return darkerValues
}

// ComputeFAST computes the location of FAST keypoints.
// The configuration should contain the following parameters
//   - nMatchCircle - Minimum number of consecutive pixels out of 16 pixels on the
//     circle that should all be either brighter or darker w.r.t
//     test-pixel. A point c on the circle is darker w.r.t test pixel p
//     if “Ic < Ip - threshold“ and brighter if
//     “Ic > Ip + threshold“.
//   - nmsWin - int, size of window to perform non-maximum suppression
//   - threshold - int, Threshold used to decide whether the pixels on the
//     circle are brighter, darker or similar w.r.t. the test pixel. Given in absolute units.
//     Decrease the threshold when more corners are desired and
//     vice-versa.
func ComputeFAST(img *image.Gray, cfg *FASTConfig) KeyPoints {
	kps := make([]FASTPixel, 0)
	w, h := img.Bounds().Max.X, img.Bounds().Max.Y
	for y := 3; y < h-3; y++ {
		for x := 3; x < w-3; x++ {
			pixel := float64(img.GrayAt(x, y).Y)
			circleValues := GetPointValuesInNeighborhood(img, image.Point{x, y}, CircleIdx)
			brighterValues := getBrighterValues(circleValues, pixel+float64(cfg.Threshold))
			darkerValues := getDarkerValues(circleValues, pixel-float64(cfg.Threshold))
			if isValidSliceVals(brighterValues, cfg.NMatchesCircle) {
				kps = append(kps, FASTPixel{image.Point{x, y}, brighter})
			} else if isValidSliceVals(darkerValues, cfg.NMatchesCircle) {
				kps = append(kps, FASTPixel{image.Point{x, y}, darker})
			}
		}
	}
	// nonMaximumSuppression
	nmsKps := nonMaximumSuppression(img, kps, cfg.NMSWinSize)

	return nmsKps
}
