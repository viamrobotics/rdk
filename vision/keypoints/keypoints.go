// Package keypoints contains the implementation of keypoints in an image. For now:
// - FAST keypoints
package keypoints

import (
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"

	"go.viam.com/rdk/rimage"
)

type (
	// KeyPoint is an image.Point that contains coordinates of a kp.
	KeyPoint image.Point // keypoint type
	// KeyPoints is a slice of image.Point that contains several kps.
	KeyPoints []image.Point // set of keypoints type
)

// OrientedKeypoints contains keypoints and their corresponding orientations.
type OrientedKeypoints struct {
	Points       KeyPoints
	Orientations []float64
}

// computeMaskOrientationFAST creates the mask used to compute orientations of corners.
func computeMaskOrientationFAST() *image.Gray {
	mask := image.NewGray(image.Rect(0, 0, 31, 31))
	indices := []int{15, 15, 15, 15, 14, 14, 14, 13, 13, 12, 11, 10, 9, 8, 6, 3}
	for i := -15; i < 16; i++ {
		for j := -indices[int(math.Abs(float64(i)))]; j < indices[int(math.Abs(float64(i)))]+1; j++ {
			mask.Set(j+15, i+15, color.Gray{1})
		}
	}
	return mask
}

func computeKeypointsOrientations(img *image.Gray, kps KeyPoints) ([]float64, error) {
	nRows, nCols := 31, 31
	nRows2 := (nRows - 1) / 2
	nCols2 := (nCols - 1) / 2
	mask := computeMaskOrientationFAST()
	padded, err := rimage.PaddingGray(img, image.Point{nCols2, nRows2}, image.Point{1, 1}, rimage.BorderConstant)
	if err != nil {
		return nil, err
	}
	orientations := make([]float64, len(kps))
	for i, kp := range kps {
		m01, m10 := 0, 0
		for y := 0; y < nRows; y++ {
			m01Temp := 0
			for x := 0; x < nCols; x++ {
				if mask.At(x, y).(color.Gray).Y > 0 {
					pixVal := int(padded.At(x+kp.X, y+kp.Y).(color.Gray).Y)
					m10 += pixVal * (x - nCols2)
					m01Temp += pixVal
				}
			}
			m01 += m01Temp * (y - nRows2)
		}
		orientations[i] = math.Atan2(float64(m01), float64(m10))
	}
	return orientations, nil
}

// GetOrientedKeyPointsFromKeyPoints computes the orientation of keypoints in the corresponding image
// and return kps and corresponding orientations in a OrientedKeypoints struct.
func GetOrientedKeyPointsFromKeyPoints(img *image.Gray, kps KeyPoints) (*OrientedKeypoints, error) {
	orientations, err := computeKeypointsOrientations(img, kps)
	if err != nil {
		return nil, err
	}
	return &OrientedKeypoints{
		kps,
		orientations,
	}, nil
}

// PlotKeypoints plots keypoints on image.
func PlotKeypoints(img *image.Gray, kps []image.Point, outName string) error {
	w, h := img.Bounds().Max.X, img.Bounds().Max.Y

	dc := gg.NewContext(w, h)
	dc.DrawImage(img, 0, 0)

	// draw keypoints on image
	dc.SetRGBA(0, 0, 1, 0.5)
	for _, p := range kps {
		dc.DrawCircle(float64(p.X), float64(p.Y), float64(3.0))
		dc.Fill()
	}
	return dc.SavePNG(outName)
}
