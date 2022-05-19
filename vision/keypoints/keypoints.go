// Package keypoints contains the implementation of keypoints in an image. For now:
// - FAST keypoints
package keypoints

import (
	"image"

	"github.com/fogleman/gg"
)

type (
	// KeyPoint is an image.Point that contains coordinates of a kp.
	KeyPoint image.Point // keypoint type
	// KeyPoints is a slice of image.Point that contains several kps.
	KeyPoints []image.Point // set of keypoints type
)

// OrientedKeypoints stores keypoint locations and orientations (nil if not oriented).
type OrientedKeypoints struct {
	Points       KeyPoints
	Orientations []float64
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

// RescaleKeypoints rescales given keypoints wrt scaleFactor
func RescaleKeypoints(kps KeyPoints, scaleFactor int) KeyPoints {
	nKeypoints := len(kps)
	rescaledKeypoints := make(KeyPoints, nKeypoints)
	for i := 0; i < nKeypoints; i++ {
		currentKp := kps[i]
		rescaled := image.Point{currentKp.X * scaleFactor, currentKp.Y * scaleFactor}
		rescaledKeypoints[i] = rescaled
	}
	return rescaledKeypoints
}
