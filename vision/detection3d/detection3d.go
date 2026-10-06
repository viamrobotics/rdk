// Package detection3d defines the 3D detection returned by the vision service GetDetections3D method.
package detection3d

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/segmentation"
)

// Detection is one perceived object, described as a tree of transforms.
//
// Transforms[0] is the root, parented to a frame the robot already knows, typically the camera the
// service read from. Every later transform is parented to the root or to an earlier transform in the
// same slice. Each transform's geometry is expressed relative to that transform's own origin, so a box
// centered on the part has a zero pose. The transforms can be passed directly to
// referenceframe.NewWorldState or referenceframe.NewFrameSystem.
type Detection struct {
	Transforms      []*referenceframe.LinkInFrame
	Classifications classification.Classifications
	Metadata        map[string]interface{}
}

// Segmenter returns the 3D detections perceived through src.
type Segmenter func(ctx context.Context, src camera.Camera) ([]*Detection, error)

// FromSegmenter adapts a segmentation.Segmenter whose objects are point cloud clusters in src's frame into a Segmenter.
//
// Each object becomes a root transform named "<namePrefix>/object-<i>", parented to src at the object's geometry pose
// and carrying that geometry recentered on the root, plus a "<root>/points" child carrying the cluster's points in the
// root frame. Root names are only unique within one call, since segmenters do not track objects across calls.
//
// An object carries one geometry and no confidence scores, so producers that know more, such as an object's parts,
// its class scores, or a frame other than src's, should build Detections directly instead.
func FromSegmenter(namePrefix string, seg segmentation.Segmenter) Segmenter {
	return func(ctx context.Context, src camera.Camera) ([]*Detection, error) {
		objects, err := seg(ctx, src)
		if err != nil {
			return nil, err
		}
		parent := src.Name().ShortName()
		detections := make([]*Detection, 0, len(objects))
		for i, obj := range objects {
			name := fmt.Sprintf("%s/object-%d", namePrefix, i)
			rootPose := spatialmath.NewZeroPose()
			var rootGeom spatialmath.Geometry
			if obj.Geometry != nil {
				rootPose = obj.Geometry.Pose()
				rootGeom = obj.Geometry.Transform(spatialmath.PoseInverse(rootPose))
			}
			transforms := []*referenceframe.LinkInFrame{referenceframe.NewLinkInFrame(parent, rootPose, name, rootGeom)}

			if obj.PointCloud != nil && obj.PointCloud.Size() > 0 {
				octree, err := pointcloud.ToBasicOctree(obj.PointCloud, 0)
				if err != nil {
					return nil, errors.Wrapf(err, "object %d", i)
				}
				points := octree.Transform(spatialmath.PoseInverse(rootPose))
				transforms = append(transforms, referenceframe.NewLinkInFrame(name, spatialmath.NewZeroPose(), name+"/points", points))
			}
			detections = append(detections, &Detection{Transforms: transforms})
		}
		return detections, nil
	}
}
