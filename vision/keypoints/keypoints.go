// Package keypoints contains the implementation of keypoints in an image. For now:
// - FAST keypoints
package keypoints

import (
	"image"

	"github.com/fogleman/gg"

	"go.viam.com/rdk/utils"
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
func PlotKeypoints(img *image.Gray, kps []image.Point) image.Image {
	w, h := img.Bounds().Max.X, img.Bounds().Max.Y

	dc := gg.NewContext(w, h)
	dc.DrawImage(img, 0, 0)

	// draw keypoints on image
	dc.SetRGBA(0, 0, 1, 0.5)
	for _, p := range kps {
		dc.DrawCircle(float64(p.X), float64(p.Y), float64(3.0))
		dc.Fill()
	}
	return dc.Image()
}

// PlotMatchedLines plots matched keypoints on both images. vertical is true if the images should be stacked on top of each other.
func PlotMatchedLines(im1, im2 image.Image, kps1, kps2 []image.Point, vertical bool) image.Image {
	w, h := im1.Bounds().Max.X+im2.Bounds().Max.X, utils.MaxInt(im1.Bounds().Max.Y, im2.Bounds().Max.Y)
	if vertical {
		w, h = utils.MaxInt(im1.Bounds().Max.X, im2.Bounds().Max.X), im1.Bounds().Max.Y+im2.Bounds().Max.Y
	}

	dc := gg.NewContext(w, h)
	dc.DrawImage(im1, 0, 0)
	if vertical {
		dc.DrawImage(im2, 0, im1.Bounds().Max.Y)
	} else {
		dc.DrawImage(im2, im1.Bounds().Max.X, 0)
	}

	// draw keypoint matches on image
	dc.SetRGBA(0, 1, 0, 0.5)
	for i, p1 := range kps1 {
		// Plot every other dot so the lines are decipherable.
		if i%2 == 0 {
			continue
		}
		p2 := kps2[i]
		dc.SetLineWidth(1.25)
		if vertical {
			dc.DrawLine(float64(p1.X), float64(p1.Y), float64(p2.X), float64(im1.Bounds().Max.Y+p2.Y))
		} else {
			dc.DrawLine(float64(p1.X), float64(p1.Y), float64(im1.Bounds().Max.X+p2.X), float64(p2.Y))
		}
		dc.Stroke()
	}
	return dc.Image()
}

// RescaleKeypoints rescales given keypoints wrt scaleFactor.
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
