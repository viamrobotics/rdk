package viscapture

import (
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"image"
)

type VisCapture interface {
	Image() image.Image
	Detections() []objectdetection.Detection
	Classifications() classification.Classifications
	PointCloudObject() []*vision.Object
}

func NewVisCapture(img image.Image, dets []objectdetection.Detection, clas classification.Classifications, obj []*vision.Object) VisCapture {
	return &capture{image: img,
		detections:      dets,
		classifications: clas,
		pcd:             obj}
}

type capture struct {
	image           image.Image
	detections      []objectdetection.Detection
	classifications classification.Classifications
	pcd             []*vision.Object
}

func (c *capture) Image() image.Image                              { return c.image }
func (c *capture) Detections() []objectdetection.Detection         { return c.detections }
func (c *capture) Classifications() classification.Classifications { return c.classifications }
func (c *capture) PointCloudObject() []*vision.Object              { return c.pcd }
