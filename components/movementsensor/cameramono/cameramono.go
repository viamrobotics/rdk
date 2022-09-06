// Package cameramono implements a visual odemetry movement sensor based ona  single camera stream
package cameramono

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/odometry"
)

const modelname = "camera_mono"

// AttrConfig is used for converting config attributes of a cameramono movement sensor.
type AttrConfig struct {
	Camera       string                          `json:"camera"`
	MotionConfig odometry.MotionEstimationConfig `json:"motion_estimation_config"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate() error {
	if config.Camera == "" {
		return errors.New("single camera missing for visual odometry")
	}
	return nil
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		modelname,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				deps registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return newCameraMono(ctx, deps, config, logger)
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

type cameramono struct {
	generic.Unimplemented
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancelFunc              func()
	output                  chan *odometry.Motion3D
	logger                  golog.Logger
	trackedPos              r3.Vector
	trackedOrient           spatialmath.Orientation
	angVel                  spatialmath.AngularVelocity
	linVel                  r3.Vector
}

func newCameraMono(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	logger.Info(
		"visual odometry with a single camera implements GetPosition, Get Orientation,GetLinearVelocity and GetAngularVelocity",
	)

	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
	}

	cam, err := camera.FromDependencies(deps, conf.Camera)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	co := &cameramono{
		cancelCtx:     cancelCtx,
		cancelFunc:    cancel,
		logger:        logger,
		trackedPos:    r3.Vector{X: 0, Y: 0, Z: 0},
		trackedOrient: spatialmath.NewOrientationVector(),
	}

	co.backgroundWorker(gostream.NewEmbeddedVideoStream(cam), &conf.MotionConfig, logger)
	return co, nil
}

func (co *cameramono) backgroundWorker(stream gostream.VideoStream, cfg *odometry.MotionEstimationConfig, logger golog.Logger) {
	defer func() {
		utils.UncheckedError(stream.Close(context.Background()))
	}()

	// define and get times for calculations
	startTime := time.Now()
	logger.Infof("start time : %s", startTime)
	start, release, err := stream.Next(co.cancelCtx)
	if err != nil && errors.Is(err, context.Canceled) {
		return
	}
	startImage := rimage.ConvertImage(start)

	defer release()
	co.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			endTime := time.Now()
			logger.Infof("end time : %s", endTime)
			end, _, err := stream.Next(co.cancelCtx) // check release condition, necessary
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			endImage := rimage.ConvertImage(end)

			duration := endTime.Sub(startTime)

			dt := float64(duration / time.Millisecond)
			if dt <= 0.0 {
				return
			}
			motion, _, err := odometry.EstimateMotionFrom2Frames(startImage, endImage, cfg, logger)
			rAng, cAng := motion.Rotation.Dims() // maybe not needed because of rotation matrix check?
			if rAng != 3 || cAng != 3 {
				return
			}

			rLin, cLin := motion.Translation.Dims()
			if rLin != 3 || cLin != 1 {
				return
			}

			if err != nil {
				startTime = time.Now()
				return
			}
			rotMat, err := spatialmath.NewRotationMatrix(motion.Rotation.RawMatrix().Data)
			if err != nil {
				return
			}

			start = end
			startTime = endTime
			select {
			case <-co.cancelCtx.Done():
				return
			case co.output <- motion:
				co.trackedPos = co.trackedPos.Add(translationToR3(motion)) // most drift occurs here due to deadreckoning
				co.trackedOrient = co.trackedOrient.RotationMatrix().MatMul(*rotMat)

				// Future improvements: output linear and angular velocity from the odometry algorithm itself?
				co.linVel = calculateLinVel(motion, dt)
				co.angVel = *co.angVel.OrientationToAngularVel(rotMat, dt)
			}
		}
	}, co.activeBackgroundWorkers.Done)
}

// Close closes all the channels and threads.
func (co *cameramono) Close() {
	co.cancelFunc()
	co.activeBackgroundWorkers.Wait()
}

func (co *cameramono) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	return geo.NewPoint(co.trackedPos.X, co.trackedPos.Y), co.trackedPos.Z, nil
}

func (co *cameramono) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return co.trackedOrient, nil
}

func (co *cameramono) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.GetReadings(ctx, co)
}

func (co *cameramono) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	return co.linVel, nil
}

func (co *cameramono) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return co.angVel, nil
}

func (co *cameramono) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		PositionSupported:        true,
		OrientationSupported:     true,
		AngularVelocitySupported: true,
		LinearVelocitySupported:  true,
	}, nil
}

// helpers.
func translationToR3(motion *odometry.Motion3D) r3.Vector {
	return r3.Vector{
		X: motion.Translation.At(0, 0),
		Y: motion.Translation.At(1, 0),
		Z: motion.Translation.At(2, 0),
	}
}

func calculateLinVel(motion *odometry.Motion3D, dt float64) r3.Vector {
	/// add dt check here as well? It will never be zero and passe din in this package
	tVec := translationToR3(motion)
	return r3.Vector{
		X: tVec.X / dt,
		Y: tVec.Y / dt,
		Z: tVec.Z / dt,
	}
}

// unimplemented methods.
func (co *cameramono) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, nil
}

func (co *cameramono) GetCompassHeading(ctx context.Context) (float64, error) {
	return 0, nil
}
