// Package viscapture implements VisCapture struct returned by the CaptureAllFromCamera vision service method
package viscapture

import (
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisCapture is a struct providing bundled capture of vision objects.
type VisCapture struct {
	Image           *camera.NamedImage
	Detections      []objectdetection.Detection
	Classifications classification.Classifications
	Objects         []*vision.Object
	Extra           map[string]interface{}
}

// CaptureOptions is a struct to configure CaptureAllFromCamera request.s.
type CaptureOptions struct {
	ReturnImage           bool
	ReturnDetections      bool
	ReturnClassifications bool
	ReturnObject          bool
}
