package transform

import (
	"encoding/json"
	"image"
	"io"
	"os"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// DepthColorIntrinsicsExtrinsics holds the intrinsics for a color camera, a depth camera, and the pose transformation that
// transforms a point from being in the reference frame of the depth camera to the reference frame of the color camera.
type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth"`
	ExtrinsicD2C Extrinsics              `json:"extrinsics_depth_to_color"`
}

// NewEmptyDepthColorIntrinsicsExtrinsics creates an zero initialized DepthColorIntrinsicsExtrinsics.
func NewEmptyDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

// NewDepthColorIntrinsicsExtrinsicsFromBytes reads a JSON byte stream and turns it into DepthColorIntrinsicsExtrinsics.
func NewDepthColorIntrinsicsExtrinsicsFromBytes(byteJSON []byte) (*DepthColorIntrinsicsExtrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// Parse into map
	err := json.Unmarshal(byteJSON, intrinsics)
	if err != nil {
		err = errors.Wrap(err, "error parsing byte array")
		return nil, err
	}
	return intrinsics, nil
}

// NewDepthColorIntrinsicsExtrinsicsFromJSONFile takes in a file path to a JSON and turns it into DepthColorIntrinsicsExtrinsics.
func NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonPath string) (*DepthColorIntrinsicsExtrinsics, error) {
	// open json file
	//nolint:gosec
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Wrap(err, "error opening JSON file")
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		err = errors.Wrap(err, "error reading JSON data")
		return nil, err
	}
	return NewDepthColorIntrinsicsExtrinsicsFromBytes(byteValue)
}

// CheckValid checks if the fields for DepthColorIntrinsicsExtrinsics have valid inputs.
func (dcie *DepthColorIntrinsicsExtrinsics) CheckValid() error {
	if dcie == nil {
		return errors.New("pointer to DepthColorIntrinsicsExtrinsics is nil")
	}
	if dcie.ColorCamera.Width == 0 || dcie.ColorCamera.Height == 0 {
		return errors.Errorf("invalid ColorSize (%#v, %#v)", dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	}
	if dcie.DepthCamera.Width == 0 || dcie.DepthCamera.Height == 0 {
		return errors.Errorf("invalid DepthSize (%#v, %#v)", dcie.DepthCamera.Width, dcie.DepthCamera.Height)
	}
	return nil
}

// AlignColorAndDepthImage takes in a RGB image and Depth map and aligns them according to the Aligner,
// returning a new Image and DepthMap.
func (dcie *DepthColorIntrinsicsExtrinsics) AlignColorAndDepthImage(c *rimage.Image, d *rimage.DepthMap,
) (*rimage.Image, *rimage.DepthMap, error) {
	if c == nil {
		return nil, nil, errors.New("no color image present to align")
	}
	if d == nil {
		return nil, nil, errors.New("no depth image present to align")
	}
	return dcie.TransformDepthCoordToColorCoord(c, d)
}

// TransformDepthCoordToColorCoord changes the coordinate system of the depth map to be in same coordinate system
// as the color image.
func (dcie *DepthColorIntrinsicsExtrinsics) TransformDepthCoordToColorCoord(
	col *rimage.Image, dep *rimage.DepthMap,
) (*rimage.Image, *rimage.DepthMap, error) {
	if col.Height() != dcie.ColorCamera.Height || col.Width() != dcie.ColorCamera.Width {
		return nil, nil,
			errors.Errorf("camera matrices expected color image of (%#v,%#v), got (%#v, %#v)",
				dcie.ColorCamera.Width, dcie.ColorCamera.Height, col.Width(), col.Height())
	}
	if dep.Height() != dcie.DepthCamera.Height || dep.Width() != dcie.DepthCamera.Width {
		return nil, nil,
			errors.Errorf("camera matrices expected depth image of (%#v,%#v), got (%#v, %#v)",
				dcie.DepthCamera.Width, dcie.DepthCamera.Height, dep.Width(), dep.Height())
	}
	outmap := rimage.NewEmptyDepthMap(dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	for dy := 0; dy < dcie.DepthCamera.Height; dy++ {
		for dx := 0; dx < dcie.DepthCamera.Width; dx++ {
			dz := dep.GetDepth(dx, dy)
			if dz == 0 {
				continue
			}
			// if depth pixels are bigger than color pixel, will cause a grid effect. Take into account size of pixel
			// get top-left corner of depth pixel
			cx, cy, cz0 := dcie.DepthPixelToColorPixel(float64(dx)-0.5, float64(dy)-0.5, float64(dz))
			cx0, cy0 := int(cx+0.5), int(cy+0.5)
			// get bottom-right corner of depth pixel
			cx, cy, cz1 := dcie.DepthPixelToColorPixel(float64(dx)+0.5, float64(dy)+0.5, float64(dz))
			cx1, cy1 := int(cx+0.5), int(cy+0.5)
			if cx0 < 0 || cy0 < 0 || cx1 > dcie.ColorCamera.Width-1 || cy1 > dcie.ColorCamera.Height-1 {
				continue
			}
			z := rimage.Depth((cz0 + cz1) / 2.0) // average of depth within color pixel
			for y := cy0; y <= cy1; y++ {
				for x := cx0; x <= cx1; x++ {
					outmap.Set(x, y, z)
				}
			}
		}
	}
	return col, outmap, nil
}

// ImagePointTo3DPoint takes in a image coordinate and returns the 3D point from the camera matrix.
func (dcie *DepthColorIntrinsicsExtrinsics) ImagePointTo3DPoint(point image.Point, depth rimage.Depth) (r3.Vector, error) {
	return intrinsics2DPtTo3DPt(point, depth, &dcie.ColorCamera)
}

// RGBDToPointCloud takes an Image and DepthMap and uses the camera parameters to project it to a pointcloud.
func (dcie *DepthColorIntrinsicsExtrinsics) RGBDToPointCloud(
	img *rimage.Image, dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	var rect *image.Rectangle
	if len(crop) > 1 {
		return nil, errors.Errorf("cannot have more than one cropping rectangle, got %v", crop)
	}
	if len(crop) == 1 {
		rect = &crop[0]
	}
	return intrinsics2DTo3D(img, dm, &dcie.ColorCamera, rect)
}

// PointCloudToRGBD takes a PointCloud with color info and returns an Image and DepthMap
// from the perspective of the color camera referenceframe.
func (dcie *DepthColorIntrinsicsExtrinsics) PointCloudToRGBD(
	cloud pointcloud.PointCloud,
) (*rimage.Image, *rimage.DepthMap, error) {
	return intrinsics3DTo2D(cloud, &dcie.ColorCamera)
}

// DepthPixelToColorPixel takes a pixel+depth (x,y, depth) from the depth camera and output is the coordinates
// of the color camera. Extrinsic matrices in meters, points are in mm, need to convert to m and then back.
func (dcie *DepthColorIntrinsicsExtrinsics) DepthPixelToColorPixel(dx, dy, dz float64) (float64, float64, float64) {
	m2mm := 1000.0
	x, y, z := dcie.DepthCamera.PixelToPoint(dx, dy, dz)
	x, y, z = x/m2mm, y/m2mm, z/m2mm
	x, y, z = dcie.ExtrinsicD2C.TransformPointToPoint(x, y, z)
	x, y, z = x*m2mm, y*m2mm, z*m2mm
	cx, cy := dcie.ColorCamera.PointToPixel(x, y, z)
	return cx, cy, z
}
