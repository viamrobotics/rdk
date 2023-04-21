package videosource

import (
	"context"
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const numThreadsVideoSource = 8 // This should be a param

var modelJoinPC = resource.NewDefaultModel("join_pointclouds")

func init() {
	resource.RegisterComponent(
		camera.Subtype,
		modelJoinPC,
		resource.Registration[camera.Camera, *JoinConfig]{
			DeprecatedRobotConstructor: func(
				ctx context.Context,
				r any,
				conf resource.Config,
				logger golog.Logger,
			) (camera.Camera, error) {
				actualR, err := rdkutils.AssertType[robot.Robot](r)
				if err != nil {
					return nil, err
				}
				newConf, err := resource.NativeConfig[*JoinConfig](conf)
				if err != nil {
					return nil, err
				}
				src, err := newJoinPointCloudSource(ctx, actualR, logger, conf.ResourceName(), newConf)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src), nil
			},
		})
}

// JoinConfig is the attribute struct for joinPointCloudSource.
type JoinConfig struct {
	TargetFrame   string   `json:"target_frame"`
	SourceCameras []string `json:"source_cameras"`
	// Closeness defines how close 2 points should be together to be considered the same point when merged.
	Closeness            float64                            `json:"proximity_threshold_mm,omitempty"`
	MergeMethod          string                             `json:"merge_method,omitempty"`
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Debug                bool                               `json:"debug,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *JoinConfig) Validate(path string) ([]string, error) {
	var deps []string
	if len(cfg.SourceCameras) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "source_cameras")
	}
	deps = append(deps, cfg.SourceCameras...)
	return deps, nil
}

type (
	// MergeMethodType Defines which strategy is used for merging.
	MergeMethodType string
	// MergeMethodUnsupportedError is returned when the merge method is not supported.
	MergeMethodUnsupportedError error
)

const (
	// Null is a default value for the merge method.
	Null = MergeMethodType("")
	// Naive is the naive merge method.
	Naive = MergeMethodType("naive")
	// ICP is the ICP merge method.
	ICP = MergeMethodType("icp")
)

func newMergeMethodUnsupportedError(method string) MergeMethodUnsupportedError {
	return errors.Errorf("merge method %s not supported", method)
}

// joinPointCloudSource takes image sources that can produce point clouds and merges them together from
// the point of view of targetName. The model needs to have the entire robot available in order to build the correct offsets
// between robot components for the frame system transform.
type joinPointCloudSource struct {
	resource.Named
	resource.AlwaysRebuild
	sourceCameras []camera.Camera
	sourceNames   []string
	targetName    string
	robot         robot.Robot
	mergeMethod   MergeMethodType
	logger        golog.Logger
	debug         bool
	closeness     float64
}

// newJoinPointCloudSource creates a camera that combines point cloud sources into one point cloud in the
// reference frame of targetName.
func newJoinPointCloudSource(
	ctx context.Context,
	r robot.Robot,
	l golog.Logger,
	name resource.Name,
	conf *JoinConfig,
) (camera.VideoSource, error) {
	joinSource := &joinPointCloudSource{
		Named: name.AsNamed(),
	}
	// frame to merge from
	joinSource.sourceCameras = make([]camera.Camera, len(conf.SourceCameras))
	joinSource.sourceNames = make([]string, len(conf.SourceCameras))
	for i, source := range conf.SourceCameras {
		joinSource.sourceNames[i] = source
		camSource, err := camera.FromRobot(r, source)
		if err != nil {
			return nil, fmt.Errorf("no camera source called (%s): %w", source, err)
		}
		joinSource.sourceCameras[i] = camSource
	}
	// frame to merge to
	joinSource.targetName = conf.TargetFrame
	joinSource.robot = r
	joinSource.closeness = conf.Closeness

	joinSource.logger = l
	joinSource.debug = conf.Debug

	joinSource.mergeMethod = MergeMethodType(conf.MergeMethod)

	if idx, ok := contains(joinSource.sourceNames, joinSource.targetName); ok {
		parentCamera := joinSource.sourceCameras[idx]
		var intrinsicParams *transform.PinholeCameraIntrinsics
		if parentCamera != nil {
			props, err := parentCamera.Properties(ctx)
			if err != nil {
				return nil, camera.NewPropertiesError(
					fmt.Sprintf("point cloud source at index %d for target %s", idx, conf.TargetFrame))
			}
			intrinsicParams = props.IntrinsicParams
		}
		return camera.NewVideoSourceFromReader(
			ctx,
			joinSource,
			&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsicParams},
			camera.ColorStream,
		)
	}
	return camera.NewVideoSourceFromReader(ctx, joinSource, nil, camera.ColorStream)
}

// NextPointCloud gets all the point clouds from the source cameras,
// and puts the points in one point cloud in the frame of targetFrame.
func (jpcs *joinPointCloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	switch jpcs.mergeMethod {
	case Naive, Null:
		return jpcs.NextPointCloudNaive(ctx)
	case ICP:
		return jpcs.NextPointCloudICP(ctx)
	default:
		return nil, newMergeMethodUnsupportedError(string(jpcs.mergeMethod))
	}
}

func (jpcs *joinPointCloudSource) NextPointCloudNaive(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloudNaive")
	defer span.End()

	fs, err := framesystem.RobotFrameSystem(ctx, jpcs.robot, nil)
	if err != nil {
		return nil, err
	}

	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}
	cloudFuncs := make([]pointcloud.CloudAndOffsetFunc, len(jpcs.sourceCameras))
	for i, cam := range jpcs.sourceCameras {
		iCopy := i
		camCopy := cam
		pcSrc := func(ctx context.Context) (pointcloud.PointCloud, spatialmath.Pose, error) {
			ctx, span := trace.StartSpan(ctx, "camera::joinPointCloudSource::NextPointCloud::"+jpcs.sourceNames[iCopy]+"-NextPointCloud")
			defer span.End()
			var framePose spatialmath.Pose
			if jpcs.sourceNames[iCopy] != jpcs.targetName {
				sourceFrame := referenceframe.NewPoseInFrame(jpcs.sourceNames[iCopy], spatialmath.NewZeroPose())
				theTransform, err := fs.Transform(inputs, sourceFrame, jpcs.targetName)
				if err != nil {
					return nil, nil, err
				}
				framePose = theTransform.(*referenceframe.PoseInFrame).Pose()
			}
			pc, err := camCopy.NextPointCloud(ctx)
			if err != nil {
				return nil, nil, err
			}
			if pc == nil {
				return nil, nil, errors.Errorf("camera %q returned a nil point cloud", jpcs.sourceNames[iCopy])
			}
			return pc, framePose, nil
		}
		cloudFuncs[iCopy] = pcSrc
	}

	return pointcloud.MergePointClouds(ctx, cloudFuncs, jpcs.logger)
}

func (jpcs *joinPointCloudSource) NextPointCloudICP(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloudICP")
	defer span.End()

	fs, err := framesystem.RobotFrameSystem(ctx, jpcs.robot, nil)
	if err != nil {
		return nil, err
	}

	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}

	targetIndex := 0

	for i, camName := range jpcs.sourceNames {
		if camName == jpcs.targetName {
			targetIndex = i
			break
		}
	}

	targetPointCloud, err := jpcs.sourceCameras[targetIndex].NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	finalPointCloud := pointcloud.ToKDTree(targetPointCloud)
	for i := range jpcs.sourceCameras {
		if i == targetIndex {
			continue
		}

		pcSrc, err := jpcs.sourceCameras[i].NextPointCloud(ctx)
		if err != nil {
			return nil, err
		}

		sourceFrame := referenceframe.NewPoseInFrame(jpcs.sourceNames[i], spatialmath.NewZeroPose())
		theTransform, err := fs.Transform(inputs, sourceFrame, jpcs.targetName)
		if err != nil {
			return nil, err
		}

		registeredPointCloud, info, err := pointcloud.RegisterPointCloudICP(pcSrc, finalPointCloud,
			theTransform.(*referenceframe.PoseInFrame).Pose(), jpcs.debug, numThreadsVideoSource)
		if err != nil {
			return nil, err
		}
		if jpcs.debug {
			jpcs.logger.Debugf("Learned Transform = %v", info.OptResult.Location.X)
		}
		transformDist := math.Sqrt(math.Pow(info.OptResult.Location.X[0]-info.X0[0], 2) +
			math.Pow(info.OptResult.Location.X[1]-info.X0[1], 2) +
			math.Pow(info.OptResult.Location.X[2]-info.X0[2], 2))
		if transformDist > 100 {
			jpcs.logger.Warnf(`Transform is %f away from transform defined in frame system. 
			This may indicate an incorrect frame system.`, transformDist)
		}
		registeredPointCloud.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			nearest, _, _, _ := finalPointCloud.NearestNeighbor(p)
			distance := math.Sqrt(math.Pow(p.X-nearest.X, 2) + math.Pow(p.Y-nearest.Y, 2) + math.Pow(p.Z-nearest.Z, 2))
			if distance > jpcs.closeness {
				err = finalPointCloud.Set(p, d)
				if err != nil {
					return false
				}
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}

	return finalPointCloud, nil
}

// initalizeInputs gets all the input positions for the robot components in order to calculate the frame system offsets.
func (jpcs *joinPointCloudSource) initializeInputs(
	ctx context.Context,
	fs referenceframe.FrameSystem,
) (map[string][]referenceframe.Input, error) {
	inputs := referenceframe.StartPositions(fs)

	for k, original := range inputs {
		if strings.HasSuffix(k, "_origin") {
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

// Read gets the merged point cloud from all sources, and then uses a projection to turn it into a 2D image.
func (jpcs *joinPointCloudSource) Read(ctx context.Context) (image.Image, func(), error) {
	var proj transform.Projector
	var err error
	if idx, ok := contains(jpcs.sourceNames, jpcs.targetName); ok {
		proj, err = jpcs.sourceCameras[idx].Projector(ctx)
		if err != nil && !errors.Is(err, transform.ErrNoIntrinsics) {
			return nil, nil, err
		}
	}
	if proj == nil { // use a default projector if target frame doesn't have one
		proj = &transform.ParallelProjection{}
	}

	pc, err := jpcs.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	if jpcs.debug && pc != nil {
		jpcs.logger.Debugf("joinPointCloudSource Read: number of points in pointcloud: %d", pc.Size())
	}
	img, _, err := proj.PointCloudToRGBD(pc)
	return img, func() {}, err // return color image
}

func (jpcs *joinPointCloudSource) Close(ctx context.Context) error {
	return nil
}

func contains(s []string, str string) (int, bool) {
	for i, v := range s {
		if v == str {
			return i, true
		}
	}
	return -1, false
}
