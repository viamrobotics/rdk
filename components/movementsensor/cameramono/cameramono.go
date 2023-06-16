// Package cameramono implements a visual odemetry movement sensor based ona  single camera stream
// This is an Experimental package
package cameramono

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/viamrobotics/gostream"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/vision/odometry"
)

var model = resource.DefaultModelFamily.WithModel("camera_mono")

// Config is used for converting config attributes of a cameramono movement sensor.
type Config struct {
	Camera       string                           `json:"camera"`
	MotionConfig *odometry.MotionEstimationConfig `json:"motion_estimation_config"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Camera == "" {
		return nil, utils.NewConfigValidationError(path,
			errors.New("single camera missing for visual odometry"))
	}
	deps = append(deps, cfg.Camera)
	if cfg.MotionConfig == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no motion_estimation_config for visual odometry algorithm"))
	}

	if cfg.MotionConfig.KeyPointCfg == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no kps config found in motion_estimation_config"))
	}

	if cfg.MotionConfig.MatchingCfg == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no matching config found in motion_estimation_config"))
	}

	if cfg.MotionConfig.CamIntrinsics == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no camera_instrinsics config found in motion_estimation_config"))
	}

	if cfg.MotionConfig.ScaleEstimatorCfg == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no scale_estimator config found in motion_estimation_config"))
	}

	if cfg.MotionConfig.CamHeightGround == 0 {
		return nil, utils.NewConfigValidationError(path,
			errors.New("set camera_height from ground to 0 by default"))
	}
	if cfg.MotionConfig.KeyPointCfg.BRIEFConf == nil {
		return nil, utils.NewConfigValidationError(path,
			errors.New("no BRIEF Config found in motion_estimation_config"))
	}

	if cfg.MotionConfig.KeyPointCfg.DownscaleFactor <= 1 {
		return nil, utils.NewConfigValidationError(path,
			errors.New("downscale_factor in motion_estimation_config should be greater than 1"))
	}

	return deps, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (movementsensor.MovementSensor, error) {
				return newCameraMono(deps, conf, logger)
			},
		})
}

type cameramono struct {
	resource.Named
	resource.AlwaysRebuild
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancelFunc              func()
	motion                  *odometry.Motion3D
	logger                  golog.Logger
	result                  result
	stream                  gostream.VideoStream
	mu                      sync.RWMutex
	err                     movementsensor.LastError
}

type result struct {
	dt            float64
	trackedPos    r3.Vector
	trackedOrient spatialmath.Orientation
	angVel        spatialmath.AngularVelocity
	linVel        r3.Vector
}

func newCameraMono(
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	logger.Info(
		"visual odometry using one camera implements Position, Orientation, LinearVelocity and AngularVelocity",
	)

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	cam, err := camera.FromDependencies(deps, newConf.Camera)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	co := &cameramono{
		Named:      conf.ResourceName().AsNamed(),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		motion: &odometry.Motion3D{
			Rotation:    mat.NewDense(3, 3, []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}),
			Translation: mat.NewDense(3, 1, []float64{0, 0, 0}),
		},
		result: result{
			trackedPos:    r3.Vector{X: 0, Y: 0, Z: 0},
			trackedOrient: spatialmath.NewOrientationVector(),
		},
		err: movementsensor.NewLastError(1, 1),
	}

	co.stream = gostream.NewEmbeddedVideoStream(cam)

	err = co.backgroundWorker(co.stream, newConf.MotionConfig)
	co.err.Set(err)

	return co, co.err.Get()
}

func (co *cameramono) backgroundWorker(stream gostream.VideoStream, cfg *odometry.MotionEstimationConfig) error {
	defer func() { utils.UncheckedError(stream.Close(context.Background())) }()
	co.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		sT := time.Now()
		sImg, _, err := stream.Next(co.cancelCtx)
		if err != nil && errors.Is(err, context.Canceled) {
			co.err.Set(err)
			return
		}
		for {
			eT := time.Now()
			eImg, _, err := stream.Next(co.cancelCtx)
			if err != nil {
				co.err.Set(err)
				return
			}

			dt, moreThanZero := co.getDt(sT, eT)
			co.result.dt = dt
			if moreThanZero {
				co.motion, err = co.extractMovementFromOdometer(rimage.ConvertImage(sImg), rimage.ConvertImage(eImg), dt, cfg)
				if err != nil {
					co.err.Set(err)
					co.logger.Error(err)
					continue
				}
				select {
				case <-co.cancelCtx.Done():
					return
				default:
				}
			}

			sImg = eImg
			sT = eT
		}
	}, co.activeBackgroundWorkers.Done)
	return co.err.Get()
}

func (co *cameramono) extractMovementFromOdometer(
	start, end *rimage.Image,
	dt float64,
	cfg *odometry.MotionEstimationConfig,
) (*odometry.Motion3D, error) {
	motion, _, err := odometry.EstimateMotionFrom2Frames(start, end, cfg, co.logger)
	if err != nil {
		motion = co.motion
		return motion, err
	}

	rAng, cAng := motion.Rotation.Dims()
	if rAng != 3 || cAng != 3 {
		return nil, errors.New("rotation dims are not 3,3")
	}

	rLin, cLin := motion.Translation.Dims()
	if rLin != 3 || cLin != 1 {
		return nil, errors.New("lin dims are not 3,1")
	}

	rotMat, err := spatialmath.NewRotationMatrix(co.motion.Rotation.RawMatrix().Data)
	if err != nil {
		return nil, err
	}

	co.mu.RLock()
	defer co.mu.RUnlock()
	co.result.trackedOrient = co.result.trackedOrient.RotationMatrix().LeftMatMul(*rotMat)
	co.result.trackedPos = co.result.trackedPos.Add(translationToR3(co.motion))
	co.result.linVel = calculateLinVel(motion, dt)
	co.result.angVel = spatialmath.OrientationToAngularVel(rotMat.EulerAngles(), dt)

	return motion, err
}

func (co *cameramono) getDt(startTime, endTime time.Time) (float64, bool) {
	duration := endTime.Sub(startTime)
	dt := float64(duration/time.Millisecond) / 1000
	moreThanZero := dt > 0
	return dt, moreThanZero
}

// Close closes all the channels and threads.
func (co *cameramono) Close(ctx context.Context) error {
	co.cancelFunc()
	co.activeBackgroundWorkers.Wait()
	err := co.stream.Close(co.cancelCtx)
	co.err.Set(err)
	return nil
}

// Position gets the position of the moving object calculated by visual odometry.
func (co *cameramono) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return geo.NewPoint(co.result.trackedPos.X, co.result.trackedPos.Y), co.result.trackedPos.Z, nil
}

// Oritentation gets the position of the moving object calculated by visual odometry.
func (co *cameramono) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.result.trackedOrient, nil
}

// Readings gets the position of the moving object calculated by visual odometry.
func (co *cameramono) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, co, extra)
}

// LinearVelocity gets the position of the moving object calculated by visual odometry.
func (co *cameramono) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.result.linVel, nil
}

func (co *cameramono) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

// AngularVelocity gets the position of the moving object calculated by visual odometry.
func (co *cameramono) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.result.angVel, nil
}

// Properties gets the position of the moving object calculated by visual odometry.
func (co *cameramono) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
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
	tVec := translationToR3(motion)
	return r3.Vector{
		X: tVec.X / dt,
		Y: tVec.Y / dt,
		Z: tVec.Z / dt,
	}
}

// unimplemented methods.

// Accuracy gets the position of the moving object calculated by visual odometry.
func (co *cameramono) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

// CompassHeadings gets the position of the moving object calculated by visual odometry.
func (co *cameramono) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}
