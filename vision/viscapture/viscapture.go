// Package viscapture implements VisCapture interface returned by the CaptureAllFromCamera vision service method
package viscapture

import (
	"image"

	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisCapture returns a bundled capture of the requested vision objects.
type VisCapture interface {
	Image() image.Image
	Detections() []objectdetection.Detection
	Classifications() classification.Classifications
	PointCloudObject() []*vision.Object
}

// NewVisCapture returns a VisCapture.
func NewVisCapture(img image.Image,
	dets []objectdetection.Detection,
	class classification.Classifications,
	obj []*vision.Object,
) VisCapture {
	return &capture{
		image:           img,
		detections:      dets,
		classifications: class,
		pcd:             obj,
	}
}

type capture struct {
	image           image.Image
	detections      []objectdetection.Detection
	classifications classification.Classifications
	pcd             []*vision.Object
}

// Image returns the image input used to compute Detections, Classifications or PointCloudObject.
func (c *capture) Image() image.Image { return c.image }

// Detections returns a list of Detections from VisCapture.
func (c *capture) Detections() []objectdetection.Detection { return c.detections }

// Classifications returns a list of Classifications from VisCapture.
func (c *capture) Classifications() classification.Classifications { return c.classifications }

// PointCloudObject returns a list of PointCloudObjects from VisCapture().
func (c *capture) PointCloudObject() []*vision.Object { return c.pcd }
