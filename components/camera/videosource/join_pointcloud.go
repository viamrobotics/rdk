package videosource

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
)

const numThreadsVideoSource = 8 // This should be a param

var modelJoinPC = resource.DefaultModelFamily.WithModel("join_pointclouds")

func init() {
	resource.RegisterComponent(
		camera.API,
		modelJoinPC,
		resource.Registration[camera.Camera, *Config]{
			Constructor: newJoinPointCloudCamera,
		},
	)
}

// Config is the attribute struct for joinPointCloudSource.
type Config struct {
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
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if len(cfg.SourceCameras) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "source_cameras")
	}
	deps = append(deps, cfg.SourceCameras...)
	deps = append(deps, framesystem.InternalServiceName.String())
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
type joinPointCloudCamera struct {
	resource.Named
	resource.AlwaysRebuild
	sourceCameras []camera.Camera
	sourceNames   []string
	targetName    string
	fsService     framesystem.Service
	mergeMethod   MergeMethodType
	logger        logging.Logger
	debug         bool
	closeness     float64
	src           camera.VideoSource
}

// newJoinPointCloudSource creates a camera that combines point cloud sources into one point cloud in the
// reference frame of targetName.
func newJoinPointCloudCamera(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	joinCam := &joinPointCloudCamera{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := joinCam.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return camera.FromVideoSource(conf.ResourceName(), joinCam.src, logger), nil
}

func (jpcc *joinPointCloudCamera) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// frame to merge from
	jpcc.sourceCameras = make([]camera.Camera, len(cfg.SourceCameras))
	jpcc.sourceNames = make([]string, len(cfg.SourceCameras))
	for i, source := range cfg.SourceCameras {
		jpcc.sourceNames[i] = source
		camSource, err := camera.FromDependencies(deps, source)
		if err != nil {
			return fmt.Errorf("no camera source called (%s): %w", source, err)
		}
		jpcc.sourceCameras[i] = camSource
	}
	// frame to merge to
	jpcc.targetName = cfg.TargetFrame
	jpcc.fsService, err = framesystem.FromDependencies(deps)
	if err != nil {
		return err
	}
	jpcc.closeness = cfg.Closeness

	jpcc.debug = cfg.Debug

	jpcc.mergeMethod = MergeMethodType(cfg.MergeMethod)

	if idx, ok := contains(jpcc.sourceNames, jpcc.targetName); ok {
		parentCamera := jpcc.sourceCameras[idx]
		var intrinsicParams *transform.PinholeCameraIntrinsics
		if parentCamera != nil {
			props, err := parentCamera.Properties(ctx)
			if err != nil {
				return camera.NewPropertiesError(fmt.Sprintf("point cloud source at index %d for target %s", idx, jpcc.targetName))
			}
			intrinsicParams = props.IntrinsicParams
		}
		jpcc.src, err = camera.NewVideoSourceFromReader(
			ctx,
			jpcc,
			&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsicParams},
			camera.ColorStream,
		)
		return err
	}
	jpcc.src, err = camera.NewVideoSourceFromReader(ctx, jpcc, nil, camera.ColorStream)
	return err
}

// NextPointCloud gets all the point clouds from the source cameras,
// and puts the points in one point cloud in the frame of targetFrame.
func (jpcc *joinPointCloudCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	switch jpcc.mergeMethod {
	case Naive, Null:
		return jpcc.NextPointCloudNaive(ctx)
	case ICP:
		return jpcc.NextPointCloudICP(ctx)
	default:
		return nil, newMergeMethodUnsupportedError(string(jpcc.mergeMethod))
	}
}

func (jpcc *joinPointCloudCamera) NextPointCloudNaive(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloudNaive")
	defer span.End()

	fs, err := jpcc.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	inputs, _, err := jpcc.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	cloudFuncs := make([]pointcloud.CloudAndOffsetFunc, len(jpcc.sourceCameras))
	for i, cam := range jpcc.sourceCameras {
		iCopy := i
		camCopy := cam
		pcSrc := func(ctx context.Context) (pointcloud.PointCloud, spatialmath.Pose, error) {
			ctx, span := trace.StartSpan(ctx, "camera::joinPointCloudSource::NextPointCloud::"+jpcc.sourceNames[iCopy]+"-NextPointCloud")
			defer span.End()
			var framePose spatialmath.Pose
			if jpcc.sourceNames[iCopy] != jpcc.targetName {
				sourceFrame := referenceframe.NewPoseInFrame(jpcc.sourceNames[iCopy], spatialmath.NewZeroPose())
				theTransform, err := fs.Transform(inputs, sourceFrame, jpcc.targetName)
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
				return nil, nil, errors.Errorf("camera %q returned a nil point cloud", jpcc.sourceNames[iCopy])
			}
			return pc, framePose, nil
		}
		cloudFuncs[iCopy] = pcSrc
	}

	return pointcloud.MergePointClouds(ctx, cloudFuncs, jpcc.logger)
}

func (jpcc *joinPointCloudCamera) NextPointCloudICP(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloudICP")
	defer span.End()

	fs, err := jpcc.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	inputs, _, err := jpcc.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}

	targetIndex := 0

	for i, camName := range jpcc.sourceNames {
		if camName == jpcc.targetName {
			targetIndex = i
			break
		}
	}

	targetPointCloud, err := jpcc.sourceCameras[targetIndex].NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	finalPointCloud := pointcloud.ToKDTree(targetPointCloud)
	for i := range jpcc.sourceCameras {
		if i == targetIndex {
			continue
		}

		pcSrc, err := jpcc.sourceCameras[i].NextPointCloud(ctx)
		if err != nil {
			return nil, err
		}

		sourceFrame := referenceframe.NewPoseInFrame(jpcc.sourceNames[i], spatialmath.NewZeroPose())
		theTransform, err := fs.Transform(inputs, sourceFrame, jpcc.targetName)
		if err != nil {
			return nil, err
		}

		registeredPointCloud, info, err := pointcloud.RegisterPointCloudICP(pcSrc, finalPointCloud,
			theTransform.(*referenceframe.PoseInFrame).Pose(), jpcc.debug, numThreadsVideoSource)
		if err != nil {
			return nil, err
		}
		if jpcc.debug {
			jpcc.logger.Debugf("Learned Transform = %v", info.OptResult.Location.X)
		}
		transformDist := math.Sqrt(math.Pow(info.OptResult.Location.X[0]-info.X0[0], 2) +
			math.Pow(info.OptResult.Location.X[1]-info.X0[1], 2) +
			math.Pow(info.OptResult.Location.X[2]-info.X0[2], 2))
		if transformDist > 100 {
			jpcc.logger.Warnf(`Transform is %f away from transform defined in frame system. 
			This may indicate an incorrect frame system.`, transformDist)
		}
		registeredPointCloud.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			nearest, _, _, _ := finalPointCloud.NearestNeighbor(p)
			distance := math.Sqrt(math.Pow(p.X-nearest.X, 2) + math.Pow(p.Y-nearest.Y, 2) + math.Pow(p.Z-nearest.Z, 2))
			if distance > jpcc.closeness {
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

// Read gets the merged point cloud from all sources, and then uses a projection to turn it into a 2D image.
func (jpcc *joinPointCloudCamera) Read(ctx context.Context) (image.Image, func(), error) {
	var proj transform.Projector
	var err error
	if idx, ok := contains(jpcc.sourceNames, jpcc.targetName); ok {
		proj, err = jpcc.sourceCameras[idx].Projector(ctx)
		if err != nil && !errors.Is(err, transform.ErrNoIntrinsics) {
			return nil, nil, err
		}
	}
	if proj == nil { // use a default projector if target frame doesn't have one
		proj = &transform.ParallelProjection{}
	}

	pc, err := jpcc.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	if jpcc.debug && pc != nil {
		jpcc.logger.Debugf("joinPointCloudSource Read: number of points in pointcloud: %d", pc.Size())
	}
	img, _, err := proj.PointCloudToRGBD(pc)
	return img, func() {}, err // return color image
}

func (jpcc *joinPointCloudCamera) Close(ctx context.Context) error {
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
