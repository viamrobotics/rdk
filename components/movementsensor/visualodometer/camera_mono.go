package visualodometer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/odometry"
)

const model = "camera_mono"

func init() {
	registry.RegisterComponent(movementsensor.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*Attributes)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			camName := attrs.Camera
			cam, err := camera.FromDependencies(deps, camName)
			if err != nil {
				return nil, fmt.Errorf("no camera (%s): %w", camName, err)
			}
			return newCameraMono(ctx, cam, attrs, logger)
		},
	})
	config.RegisterComponentAttributeMapConverter(movementsensor.SubtypeName, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Attributes
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*Attributes)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&Attributes{})
}

type Attributes struct {
	Camera string `json:"camera"`
}

type monoCamera struct {
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancelFunc              func()
	output                  chan *vision.Motion3D
}

func newMonoCamera(
	ctx context.Context,
	cam camera.Camera,
	attrs *Attributes,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	vo := &monoCamera{
		cancelCtx:  cancelCtx,
		cancelFunc: cancel,
	}

	vo.backgroundWorker(gostream.NewEmbeddedVideoStream(cam), logger)
	return vo, nil
}

func (vo *monoCamera) backgroundWorker(stream gostream.VideoStream, logger golog.Logger) {
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
	defer release()
	vo.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			endTime := time.Now()
			end, release, err := stream.Next(vo.cancelCtx)
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			duration := endTime.Sub(startTime)
			motion, err := odometry.EstimateMotionFrom2Frames(start, end, nil, logger, false)
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
func (vo *monoCamera) Close() {
	vo.cancelFunc()
	vo.activeBackgroundWorkers.Wait()
}

func (vo *cameraMono) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
}

func (vo *cameraMono) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
}

func (vo *cameraMono) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported: true,
		LinearVelocitySupported:  true,
	}, nil
}

func (vo *cameraMono) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.GetReadings(ctx, vo)
}
