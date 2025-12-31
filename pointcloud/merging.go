package pointcloud

import (
	"context"
	"errors"
	"image/color"

	"github.com/golang/geo/r3"
	"github.com/lucasb-eyer/go-colorful"

	"go.viam.com/rdk/spatialmath"
)

// ApplyOffset takes a point cloud and an offset pose and applies the offset to each of the points in the source point cloud.
func ApplyOffset(srcpc PointCloud, offset spatialmath.Pose, pcTo PointCloud) error {
	// Fast path: no offset, just copy points directly
	if offset == nil {
		var err error
		srcpc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			err = pcTo.Set(p, d)
			return err == nil
		})
		return err
	}

	// Pre-convert offset to DualQuaternion once (avoids repeated conversion in loop)
	offsetDQ := spatialmath.DualQuaternionFromPose(offset)

	// Pre-allocate reusable DualQuaternions to avoid per-point allocations
	pointDQ := spatialmath.NewZeroPose().(*spatialmath.DualQuaternion)
	resultDQ := spatialmath.NewZeroPose().(*spatialmath.DualQuaternion)

	var err error
	srcpc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		spatialmath.ResetPoseDQTranslation(pointDQ, p)
		resultDQ.Number = offsetDQ.Transformation(pointDQ.Number)
		err = pcTo.Set(resultDQ.Point(), d)
		return err == nil
	})
	return err
}

// MergePointCloudsWithColor creates a union of point clouds from the slice of point clouds, giving
// each element of the slice a unique color.
func MergePointCloudsWithColor(clusters []PointCloud, colorSegmentation PointCloud) error {
	palette := colorful.FastWarmPalette(len(clusters))
	for i, cluster := range clusters {
		var err error
		col, ok := color.NRGBAModel.Convert(palette[i]).(color.NRGBA)
		if !ok {
			return errors.New("impossible color conversion??")
		}
		cluster.Iterate(0, 0, func(v r3.Vector, d Data) bool {
			err = colorSegmentation.Set(v, NewColoredData(col))
			return err == nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// CloudAndOffsetFunc for pairing clouds and offsets...
type CloudAndOffsetFunc func(context context.Context) (PointCloud, spatialmath.Pose, error)

// MergePointClouds merges point clouds.
func MergePointClouds(ctx context.Context, cloudFuncs []CloudAndOffsetFunc, out PointCloud) error {
	for _, f := range cloudFuncs {
		in, offset, err := f(ctx)
		if err != nil {
			return err
		}

		err = ApplyOffset(in, offset, out)
		if err != nil {
			return err
		}
	}
	return nil
}
