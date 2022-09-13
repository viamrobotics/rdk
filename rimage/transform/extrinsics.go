package transform

import (
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/pointcloud"
)

// Extrinsics holds the rotation matrix and the translation vector necessary to transform from the camera's origin to
// another reference frame.
type Extrinsics struct {
	RotationMatrix    []float64 `json:"rotation"`
	TranslationVector []float64 `json:"translation"`
}

// TransformPointToPoint applies a rigid body transform between two cameras to a 3D point.
func (params *Extrinsics) TransformPointToPoint(x, y, z float64) (float64, float64, float64) {
	rotationMatrix := params.RotationMatrix
	translationVector := params.TranslationVector
	if len(rotationMatrix) != 9 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	xTransformed := rotationMatrix[0]*x + rotationMatrix[1]*y + rotationMatrix[2]*z + translationVector[0]
	yTransformed := rotationMatrix[3]*x + rotationMatrix[4]*y + rotationMatrix[5]*z + translationVector[1]
	zTransformed := rotationMatrix[6]*x + rotationMatrix[7]*y + rotationMatrix[8]*z + translationVector[2]

	return xTransformed, yTransformed, zTransformed
}

// ApplyRigidBodyTransform projects a 3D point in a given camera image plane and return a
// new point cloud leaving the original unchanged.
func ApplyRigidBodyTransform(pts pointcloud.PointCloud, params *Extrinsics) (pointcloud.PointCloud, error) {
	transformedPoints := pointcloud.New()
	var err error
	pts.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		x, y, z := params.TransformPointToPoint(pt.X, pt.Y, pt.Z)
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
