package imagesource

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
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
			return newJoinPointCloudSource(r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "join_pointclouds",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf JoinAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*JoinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&JoinAttrs{})
}

// JoinAttrs is the attribute struct for joinPointCloudSource.
type JoinAttrs struct {
	TargetFrame   string   `json:"target_frame"`
	SourceCameras []string `json:"source_cameras"`
}

// joinPointCloudSource takes image sources that can produce point clouds and merges them together from
// the point of view of targetName. The model needs to have the entire robot available in order to build the correct offsets
// between robot components for the frame system transform.
type joinPointCloudSource struct {
	sourceCameras []camera.Camera
	sourceNames   []string
	targetName    string
	robot         robot.Robot
}

// newJoinPointCloudSource creates a camera that combines point cloud sources into one point cloud in the
// reference frame of targetName.
func newJoinPointCloudSource(r robot.Robot, attrs *JoinAttrs) (camera.Camera, error) {
	joinSource := &joinPointCloudSource{}
	// frame to merge from
	joinSource.sourceCameras = make([]camera.Camera, len(attrs.SourceCameras))
	joinSource.sourceNames = make([]string, len(attrs.SourceCameras))
	for i, source := range attrs.SourceCameras {
		joinSource.sourceNames[i] = source
		camSource, err := camera.FromRobot(r, source)
		if err != nil {
			return nil, fmt.Errorf("no camera source called (%s): %w", source, err)
		}
		joinSource.sourceCameras[i] = camSource
	}
	// frame to merge to
	joinSource.targetName = attrs.TargetFrame
	joinSource.robot = r
	if idx, ok := contains(joinSource.sourceNames, joinSource.targetName); ok {
		return camera.New(joinSource, nil, joinSource.sourceCameras[idx])
	}
	return camera.New(joinSource, nil, nil)
}

// NextPointCloud gets all the point clouds from the source cameras,
// and puts the points in one point cloud in the frame of targetFrame.
func (jpcs *joinPointCloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloud")
	defer span.End()

	pcTo := pointcloud.New()
	fs, err := jpcs.robot.FrameSystem(ctx, "join_cameras", "")
	if err != nil {
		return nil, err
	}
	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}
	for i, cam := range jpcs.sourceCameras {
		pcSrc, err := func() (pointcloud.PointCloud, error) {
			ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloud::"+jpcs.sourceNames[i]+"-NextPointCloud")
			defer span.End()
			return cam.NextPointCloud(ctx)
		}()
		if err != nil {
			return nil, err
		}
		if jpcs.sourceNames[i] == jpcs.targetName {
			pcSrc.Iterate(func(p pointcloud.Point) bool {
				err = pcTo.Set(p)
				return err == nil
			})
		} else {
			theTransform, err := fs.TransformFrame(inputs, jpcs.sourceNames[i], jpcs.targetName)
			if err != nil {
				return nil, err
			}

			pcSrc.Iterate(func(p pointcloud.Point) bool {
				vec := r3.Vector(p.Position())

				newPose := spatialmath.Compose(theTransform.Pose(), spatialmath.NewPoseFromPoint(vec))

				newPt := p.Clone(pointcloud.Vec3(newPose.Point()))
				err = pcTo.Set(newPt)
				return err == nil
			})
		}
		if err != nil {
			return nil, err
		}
	}
	return pcTo, nil
}

// initalizeInputs gets all the input positions for the robot components in order to calculate the frame system offsets.
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

// Next gets the merged point cloud from all sources, and then uses a projection to turn it into a 2D image.
func (jpcs *joinPointCloudSource) Next(ctx context.Context) (image.Image, func(), error) {
	var proj rimage.Projector
	if idx, ok := contains(jpcs.sourceNames, jpcs.targetName); ok {
		proj = camera.Projector(jpcs.sourceCameras[idx])
	}
	if proj == nil { // use a default projector if target frame doesn't have one
		proj = &rimage.ParallelProjection{}
	}
	pc, err := jpcs.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	iwd, err := proj.PointCloudToImageWithDepth(pc)
	if err != nil {
		return nil, nil, err
	}

	return iwd, func() {}, nil
}

func contains(s []string, str string) (int, bool) {
	for i, v := range s {
		if v == str {
			return i, true
		}
	}
	return -1, false
}
