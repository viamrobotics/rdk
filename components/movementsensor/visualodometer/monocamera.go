package visualodometer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/odometry"
	"go.viam.com/utils"
)

const modelname = "vo_monocular"

type AttrConfig struct {
	Camera       string                          `json"camera"`
	MotionConfig odometry.MotionEstimationConfig `json:"motion_estimation_config"`
}

func (config *AttrConfig) Validate() error {
	if config.Camera == "" {
		return errors.New("single camera missign for visual odometry")
	}
	return nil
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newMonoCamVizOdo(ctx, deps, config, logger)
		},
		})
	config.RegisterComponentAttributeMapConverter(
		movementsensor.SubtypeName,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

type vizodometer struct {
	generic.Unimplemented
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancelFunc              func()
	output                  chan *odometry.Motion3D
}

func newMonoCamVizOdo(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	logger.Info("visual odometry with a single camera allows GetLinearVelocity and GetAngularVelocity")
	cancelCtx, cancel := context.WithCancel(ctx)

	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
	}
	cam, err := camera.FromDependencies(deps, conf.Camera)
	if err != nil {
		return nil, err
	}

	vo := &vizodometer{
		cancelCtx:  cancelCtx,
		cancelFunc: cancel,
	}

	vo.backgroundWorker(gostream.NewEmbeddedVideoStream(cam), logger)
	return vo, nil
}

func (vo *vizodometer) backgroundWorker(stream gostream.VideoStream, logger golog.Logger) {
	defer func() {
		utils.UncheckedError(stream.Close(context.Background()))
	}()

	// define the visual odometry loop
	// get time differences for velocity calculation
	startTime := time.Now()
	start, release, err := stream.Next(vo.cancelCtx)
	if err != nil && errors.Is(err, context.Canceled) {
		return
	}
	startImage, ok := start.(*rimage.Image)
	if !ok {
		return
	}

	defer release()
	vo.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			endTime := time.Now()
			end, release, err := stream.Next(vo.cancelCtx)
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}

			endImage, ok := end.(*rimage.Image)
			if !ok && errors.Is(err, context.Canceled) {
				return
			}

			duration := endTime.Sub(startTime)
			motion, err := odometry.EstimateMotionFrom2Frames(startImage, endImage, nil, logger, false) // vision packages
			if err != nil {
				// if can't find the motion, do something here to not update the result
				// maybe just set "motion" to what it was before?
			}
			start = end
			startTime = endTime
			select {
			case <-vo.cancelCtx.Done():
				return
			case vo.output <- motion:
			}
		}
	}, vo.activeBackgroundWorkers.Done)
}

// Close closes all the channels and threads.
func (vo *vizodometer) Close() {
	vo.cancelFunc()
	vo.activeBackgroundWorkers.Wait()
}

func (vo *vizodometer) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	return convertTranslatioToVector(<-vo.output) // check for
}

func (vo *vizodometer) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return converRotationToAngularVelocity(<-vo.output)
}

func (vo *vizodometer) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported: true,
		LinearVelocitySupported:  true,
	}, nil
}

// helpers

func converRotationToAngularVelocity(motion *odometry.Motion3D) (spatialmath.AngularVelocity, error) {
	rotMat, err := spatialmath.NewRotationMatrix(motion.Rotation.RawMatrix().Data)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity(rotMat.AxisAngles().ToR3()), nil
}

func convertTranslatioToVector(motion *odometry.Motion3D) (r3.Vector, error) {
	return r3.Vector{
		X: motion.Translation.At(1, 1),
		Y: motion.Translation.At(2, 1),
		Z: motion.Translation.At(3, 1),
	}, nil
}

// unimplemented methods
func (vo *vizodometer) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, nil
}

func (vo *vizodometer) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return spatialmath.NewOrientationVector(), nil
}

func (vo *vizodometer) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.GetReadings(ctx, vo)
}

func (vo *vizodometer) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	return &geo.Point{}, 0, nil
}

func (vo *vizodometer) GetCompassHeading(ctx context.Context) (float64, error) {
	return 0, nil
}
