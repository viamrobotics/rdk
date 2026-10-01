// Package detection3d defines the 3D detection returned by the vision service GetDetections3D method.
package detection3d

import (
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/vision/classification"
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
