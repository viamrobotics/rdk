package keypoints

import (
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"

	uts "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// SamplingType stores 0 if a sampling of image points for BRIEF is uniform, 1 if gaussian.
type SamplingType int

const (
	uniform SamplingType = iota // 0
	normal                      // 1
	fixed                       // 2
)

// SamplePairs are N pairs of points used to create the BRIEF Descriptors of a patch.
type SamplePairs struct {
	P0 []image.Point
	P1 []image.Point
	N  int
}

// GenerateSamplePairs generates n samples for a patch size with the chosen Sampling Type.
func GenerateSamplePairs(dist SamplingType, n, patchSize int) *SamplePairs {
	// sample positions
	var xs0, ys0, xs1, ys1 []int
	if dist == fixed {
		xs0 = sampleIntegers(patchSize, n, dist)
		ys0 = sampleIntegers(patchSize, n, dist)
		xs1 = sampleIntegers(patchSize, n, dist)
		for i := 0; i < n; i++ {
			ys1 = append(ys1, -ys0[i])
			if i%2 == 0 {
				xs0[i] = 2 * xs0[i] / 3
				xs1[i] = -2 * xs1[i] / 3
				ys1[i] = ys0[i]
			}
		}
	} else {
		xs0 = sampleIntegers(patchSize, n, dist)
		ys0 = sampleIntegers(patchSize, n, dist)
		xs1 = sampleIntegers(patchSize, n, dist)
		ys1 = sampleIntegers(patchSize, n, dist)
	}
	p0 := make([]image.Point, 0, n)
	p1 := make([]image.Point, 0, n)
	for i := 0; i < n; i++ {
		p0 = append(p0, image.Point{X: xs0[i], Y: ys0[i]})
		p1 = append(p1, image.Point{X: xs1[i], Y: ys1[i]})
	}

	return &SamplePairs{P0: p0, P1: p1, N: n}
}

func sampleIntegers(patchSize, n int, sampling SamplingType) []int {
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

// ComputeBRIEFDescriptors computes BRIEF descriptors on image img at keypoints kps.
func ComputeBRIEFDescriptors(img *image.Gray, sp *SamplePairs, kps *FASTKeypoints, cfg *BRIEFConfig) ([]Descriptor, error) {
	// blur image
	kernel := rimage.GetGaussian5()
	normalized := kernel.Normalize()
	blurred, err := rimage.ConvolveGray(img, normalized, image.Point{2, 2}, 0)
	if err != nil {
		return nil, err
	}
	// compute descriptors

	descs := make([]Descriptor, len(kps.Points))
	bnd := blurred.Bounds()
	halfSize := cfg.PatchSize / 2
	for k, kp := range kps.Points {
		p1 := image.Point{kp.X + halfSize, kp.Y + halfSize}
		p2 := image.Point{kp.X + halfSize, kp.Y - halfSize}
		p3 := image.Point{kp.X - halfSize, kp.Y + halfSize}
		p4 := image.Point{kp.X - halfSize, kp.Y - halfSize}
		// Divide by 64 since we store a descriptor as a uint64 array.
		descriptor := make([]uint64, sp.N/64)
		if !p1.In(bnd) || !p2.In(bnd) || !p3.In(bnd) || !p4.In(bnd) {
			descs[k] = descriptor
			continue
		}
		cosTheta := 1.0
		sinTheta := 0.0
		// if use orientation and keypoints are oriented, compute rotation matrix
		if cfg.UseOrientation && kps.Orientations != nil {
			angle := kps.Orientations[k]
			cosTheta = math.Cos(angle)
			sinTheta = math.Sin(angle)
		}
		for i := 0; i < sp.N; i++ {
			x0, y0 := float64(sp.P0[i].X), float64(sp.P0[i].Y)
			x1, y1 := float64(sp.P1[i].X), float64(sp.P1[i].Y)
			// compute rotated sampled coordinates (Identity matrix if no orientation s)
			outx0 := int(math.Round(cosTheta*x0 - sinTheta*y0))
			outy0 := int(math.Round(sinTheta*x0 + cosTheta*y0))
			outx1 := int(math.Round(cosTheta*x1 - sinTheta*y1))
			outy1 := int(math.Round(sinTheta*x1 + cosTheta*y1))
			// fill BRIEF descriptor
			p0Val := blurred.GrayAt(kp.X+outx0, kp.Y+outy0).Y
			p1Val := blurred.GrayAt(kp.X+outx1, kp.Y+outy1).Y
			if p0Val > p1Val {
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
