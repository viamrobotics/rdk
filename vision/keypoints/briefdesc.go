package keypoints

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"

	uts "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/keypoints/descriptors"
)

// SamplingType stores 0 if a sampling of image points for BRIEF is uniform, 1 if gaussian.
type SamplingType int

const (
	uniform SamplingType = iota // 0
	normal                      // 1
	fixed                       // 2
)

// BRIEFConfig stores the parameters.
type BRIEFConfig struct {
	N              int          `json:"n"` // number of samples taken
	Sampling       SamplingType `json:"sampling"`
	UseOrientation bool         `json:"use_orientation"`
	PatchSize      int          `json:"patch_size"`
}

// LoadBRIEFConfiguration loads a BRIEFConfig from a json file.
func LoadBRIEFConfiguration(file string) *BRIEFConfig {
	var config BRIEFConfig
	filePath := filepath.Clean(file)
	configFile, err := os.Open(filePath)
	defer uts.UncheckedErrorFunc(configFile.Close)
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

func sampleIntegers(patchSize int, n int, sampling SamplingType) []int {
	vMin := math.Round(-(float64(patchSize) - 2) / 2.)
	vMax := math.Round(float64(patchSize) / 2.)
	switch sampling {
	case uniform:
		return utils.SampleNIntegersUniform(n, vMin, vMax)
	case normal:
		return utils.SampleNIntegersNormal(n, vMin, vMax)
	case fixed:
		return utils.SampleNRegularlySpaced(n, vMin, vMax)
	default:
		return utils.SampleNIntegersUniform(n, vMin, vMax)
	}
}

// ComputeBRIEFDescriptors computes BRIEF descriptors on image img at keypoints kps.
func ComputeBRIEFDescriptors(img *image.Gray, kps *FASTKeypoints, cfg *BRIEFConfig) (descriptors.Descriptors, error) {
	// blur image
	kernel := rimage.GetGaussian5()
	normalized := kernel.Normalize()
	blurred, err := rimage.ConvolveGray(img, normalized, image.Point{2, 2}, 0)
	if err != nil {
		return nil, err
	}
	// sample positions
	xs0 := sampleIntegers(cfg.PatchSize, cfg.N, cfg.Sampling)
	ys0 := utils.CycleIntSliceByN(sampleIntegers(cfg.PatchSize, cfg.N, cfg.Sampling), cfg.N/4)
	xs1 := utils.CycleIntSliceByN(sampleIntegers(cfg.PatchSize, cfg.N, cfg.Sampling), cfg.N/2)
	ys1 := utils.CycleIntSliceByN(sampleIntegers(cfg.PatchSize, cfg.N, cfg.Sampling), 3*cfg.N/4)

	// compute descriptors

	descs := make([]descriptors.Descriptor, len(kps.Points))
	padded, err := rimage.PaddingGray(blurred, image.Point{17, 17}, image.Point{8, 8}, rimage.BorderConstant)
	if err != nil {
		return nil, err
	}
	for k, kp := range kps.Points {
		paddedKp := image.Point{kp.X + 8, kp.Y + 8}
		// Divide by 64 since we store a descriptor as a uint64 array.
		descriptor := make([]uint64, cfg.N/64)
		cosTheta := 1.0
		sinTheta := 0.0
		// if use orientation and keypoints are oriented, compute rotation matrix
		if cfg.UseOrientation && kps.Orientations != nil {
			angle := kps.Orientations[k]
			cosTheta = math.Cos(angle)
			sinTheta = math.Sin(angle)
		}
		for i := 0; i < cfg.N; i++ {
			x0, y0 := float64(xs0[i]), float64(ys0[i])
			x1, y1 := float64(xs1[i]), float64(ys1[i])
			// compute rotated sampled coordinates (Identity matrix if no orientation s)
			outx0 := int(math.Round(cosTheta*x0 - sinTheta*y0))
			outy0 := int(math.Round(sinTheta*x0 + cosTheta*y0))
			outx1 := int(math.Round(cosTheta*x1 - sinTheta*y1))
			outy1 := int(math.Round(sinTheta*x1 + cosTheta*y1))
			// fill BRIEF descriptor
			if padded.At(paddedKp.X+outx0, paddedKp.Y+outy0).(color.Gray).Y < padded.At(paddedKp.X+outx1, paddedKp.Y+outy1).(color.Gray).Y {
				// Casting to an int truncates the float, which is what we want.
				descriptorIndex := int64(i / 64)
				numPos := i % 64
				// This flips the bit at numPos to 1.
				descriptor[descriptorIndex] |= (1 << numPos)
			}
		}
		descs[k] = descriptor
	}
	return descs, nil
}
