// Package viscapture implements VisCapture struct returned by the CaptureAllFromCamera vision service method
package viscapture

import (
	"image"

	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

type VisCapture struct {
	Image           image.Image
	Detections      []objectdetection.Detection
	Classifications classification.Classifications
	Objects         []*vision.Object
}

type CaptureOptions struct {
	ReturnImage           bool
	ReturnDetections      bool
	ReturnClassifications bool
	ReturnObject          bool
}
