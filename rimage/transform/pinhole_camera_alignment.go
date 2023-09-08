//go:build cgo
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
	"go.viam.com/rdk/spatialmath"
)

// DepthColorIntrinsicsExtrinsics holds the intrinsics for a color camera, a depth camera, and the pose transformation that
// transforms a point from being in the reference frame of the depth camera to the reference frame of the color camera.
type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics
	DepthCamera  PinholeCameraIntrinsics
	ExtrinsicD2C spatialmath.Pose
}

// DepthColorIntrinsicsExtrinsicsConfig is the config file that will be parsed into the proper interface.
type DepthColorIntrinsicsExtrinsicsConfig struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color_intrinsic_parameters"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth_intrinsic_parameters"`
	ExtrinsicD2C json.RawMessage         `json:"depth_to_color_extrinsic_parameters"`
}

// NewEmptyDepthColorIntrinsicsExtrinsics creates an zero initialized DepthColorIntrinsicsExtrinsics.
func NewEmptyDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0},
		ExtrinsicD2C: spatialmath.NewZeroPose(),
	}
}

// NewDepthColorIntrinsicsExtrinsicsFromBytes reads a JSON byte stream and turns it into DepthColorIntrinsicsExtrinsics.
func NewDepthColorIntrinsicsExtrinsicsFromBytes(byteJSON []byte) (*DepthColorIntrinsicsExtrinsics, error) {
	intrinExtrin := &DepthColorIntrinsicsExtrinsicsConfig{}
	// Parse into map
	err := json.Unmarshal(byteJSON, intrinExtrin)
	if err != nil {
		err = errors.Wrap(err, "error parsing byte array")
		return nil, err
	}
	temp := struct {
		R []float64 `json:"rotation_rads"`
		T []float64 `json:"translation_mm"`
	}{}
	err = json.Unmarshal(intrinExtrin.ExtrinsicD2C, &temp)
	if err != nil {
		err = errors.Wrap(err, "error parsing byte array")
		return nil, err
	}
	if len(temp.T) != 3 {
		return nil, errors.Errorf("length of translation is %d, should be 3", len(temp.T))
	}
	orientation, err := spatialmath.NewRotationMatrix(temp.R)
	if err != nil {
		return nil, err
	}
	pose := spatialmath.NewPose(r3.Vector{temp.T[0], temp.T[1], temp.T[2]}, orientation)
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	intrinsics.ColorCamera = intrinExtrin.ColorCamera
	intrinsics.DepthCamera = intrinExtrin.DepthCamera
	intrinsics.ExtrinsicD2C = pose
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
	x, y, z = dcie.TransformPointToPoint(x, y, z)
	x, y, z = x*m2mm, y*m2mm, z*m2mm
	cx, cy := dcie.ColorCamera.PointToPixel(x, y, z)
	return cx, cy, z
}

// TransformPointToPoint applies a rigid body transform specified as a Pose to two points.
func (dcie *DepthColorIntrinsicsExtrinsics) TransformPointToPoint(x, y, z float64) (float64, float64, float64) {
	pose := dcie.ExtrinsicD2C
	rotationMatrix := pose.Orientation().RotationMatrix()
	translationVector := pose.Point()
	xTransformed := rotationMatrix.At(0, 0)*x + rotationMatrix.At(0, 1)*y + rotationMatrix.At(0, 2)*z + translationVector.X
	yTransformed := rotationMatrix.At(1, 0)*x + rotationMatrix.At(1, 1)*y + rotationMatrix.At(1, 2)*z + translationVector.Y
	zTransformed := rotationMatrix.At(2, 0)*x + rotationMatrix.At(2, 1)*y + rotationMatrix.At(2, 2)*z + translationVector.Z

	return xTransformed, yTransformed, zTransformed
}

// ApplyRigidBodyTransform projects a 3D point in a given camera image plane and return a
// new point cloud leaving the original unchanged.
func (dcie *DepthColorIntrinsicsExtrinsics) ApplyRigidBodyTransform(pts pointcloud.PointCloud) (pointcloud.PointCloud, error) {
	transformedPoints := pointcloud.New()
	var err error
	pts.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		x, y, z := dcie.TransformPointToPoint(pt.X, pt.Y, pt.Z)
		err = transformedPoints.Set(pointcloud.NewVector(x, y, z), data)
		if err != nil {
			err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", x, y, z)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return transformedPoints, nil
}
