package imagesource

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"join_pointclouds",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*JoinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newJoinPointCloudSource(ctx, r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "join_pointclouds",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf JoinAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*JoinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&JoinAttrs{})
}

// JoinAttrs is the attribute struct for joinPointCloudSource.
type JoinAttrs struct {
	*camera.AttrConfig
	JoinFrom string `json:"join_from"`
	JoinTo   string `json:"join_to"`
}

// joinPointCloudSource takes two image sources that can produce point clouds and merges them together from
// the point of view of camTo. The model needs to have the entire robot available in order to build the correct offsets
// between robot components for the frame system transform.
type joinPointCloudSource struct {
	camTo, camFrom         camera.Camera
	camToName, camFromName string
	robot                  robot.Robot
}

// newJoinPointCloudSource creates a imageSource that combines two point cloud sources into one source from the
// reference frame of camTo.
func newJoinPointCloudSource(ctx context.Context, r robot.Robot, attrs *JoinAttrs) (camera.Camera, error) {
	joinSource := &joinPointCloudSource{}
	// frame to merge from
	joinSource.camFromName = attrs.JoinFrom
	camFrom, err := camera.FromRobot(r, joinSource.camFromName)
	if err != nil {
		return nil, fmt.Errorf("no camera to join from (%s): %w", joinSource.camFromName, err)
	}
	joinSource.camFrom = camFrom
	// frame to merge to
	joinSource.camToName = attrs.JoinTo
	camTo, err := camera.FromRobot(r, joinSource.camToName)
	if err != nil {
		return nil, fmt.Errorf("no camera to join to (%s): %w", joinSource.camToName, err)
	}
	joinSource.camTo = camTo
	// frame system
	fs, err := r.FrameSystem(ctx, "join_cameras", "")
	if err != nil {
		return nil, fmt.Errorf("cannot join cameras %q and %q: %w", joinSource.camFromName, joinSource.camToName, err)
	}
	frame := fs.GetFrame(joinSource.camFromName)
	if frame == nil {
		return nil, fmt.Errorf("frame %q does not exist in frame system", joinSource.camFromName)
	}
	frame = fs.GetFrame(joinSource.camToName)
	if frame == nil {
		return nil, fmt.Errorf("frame %q does not exist in frame system", joinSource.camToName)
	}
	joinSource.robot = r
	return camera.New(joinSource, attrs.AttrConfig, camTo)
}

// NextPointCloud gets both point clouds from each camera, and puts the points from camFrom in the frame of camTo.
func (jpcs *joinPointCloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	var err error
	pcFrom, err := jpcs.camFrom.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	pcTo, err := jpcs.camTo.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	fs, err := jpcs.robot.FrameSystem(ctx, "join_cameras", "")
	if err != nil {
		return nil, err
	}
	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}
	pcFrom.Iterate(func(p pointcloud.Point) bool {
		vec := r3.Vector(p.Position())
		var newVec r3.Vector
		newVec, err = fs.TransformPoint(inputs, vec, jpcs.camFromName, jpcs.camToName)
		if err != nil {
			return false
		}
		newPt := p.Clone(pointcloud.Vec3(newVec))
		err = pcTo.Set(newPt)
		return err == nil
	})
	if err != nil {
		return nil, err
	}
	return pcTo, nil
}

// get all the input positions for the robot components in order to calculate the frame system offsets
func (jpcs *joinPointCloudSource) initializeInputs(
	ctx context.Context,
	fs referenceframe.FrameSystem) (map[string][]referenceframe.Input, error) {
	inputs := referenceframe.StartPositions(fs)

	for k, original := range inputs {
		if strings.HasSuffix(k, "_offset") {
			continue
		}
		if len(original) == 0 {
			continue
		}

		all := robot.AllResourcesByName(jpcs.robot, k)
		if len(all) != 1 {
			return nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(all), k)
		}

		ii, ok := all[0].(referenceframe.InputEnabled)
		if !ok {
			return nil, fmt.Errorf("%v(%T) is not InputEnabled", k, all[0])
		}

		pos, err := ii.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		inputs[k] = pos
	}
	return inputs, nil
}

// Next gets the merged point cloud from both cameras, and then uses a parallel projection to turn it into a 2D image.
func (jpcs *joinPointCloudSource) Next(ctx context.Context) (image.Image, func(), error) {
	pp := rimage.ParallelProjection{}
	pc, err := jpcs.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	iwd, err := pp.PointCloudToImageWithDepth(pc)
	if err != nil {
		return nil, nil, err
	}

	return iwd, func() {}, nil
}
