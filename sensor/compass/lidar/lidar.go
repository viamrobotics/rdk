// Package lidar defines a slow, inefficient LiDAR based relative compass.
//
// It is useful in scenarios where an IMU is not present or another source of yaw measurement
// is desired.
package lidar

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/mat"
)

// ModelName is used to register the sensor to a model name.
const ModelName = "lidar"

// init registers the lidar compass type.
func init() {
	registry.RegisterSensor(compass.CompassType, ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return New(ctx, config, logger)
	})
}

// Lidar is a LiDAR based compass that uses MSE calculations to determine yaw.
type Lidar struct {
	lidar.Lidar
	markedRotatedMats atomic.Value // [][]rotatedMat
}

// From creates a compass from a lidar.
func From(lidar lidar.Lidar) compass.RelativeCompass {
	return &Lidar{Lidar: lidar}
}

// New returns a newly constructed lidar and turns it into a compass.
func New(ctx context.Context, config config.Component, logger golog.Logger) (compass.RelativeCompass, error) {
	lidarType := config.Attributes.String("type")
	f := registry.LidarLookup(lidarType)
	if f == nil {
		return nil, fmt.Errorf("unknown lidar model: %s", lidarType)
	}
	lidar, err := f(ctx, nil, config, logger)
	if err != nil {
		return nil, err
	}
	if err := lidar.Start(ctx); err != nil {
		return nil, err
	}
	return From(lidar), nil
}

// Desc returns a description of the compass.
func (li *Lidar) Desc() sensor.Description {
	return sensor.Description{compass.RelativeCompassType, ""}
}

// Desc stops and closes the underlying lidar.
func (li *Lidar) Close() (err error) {
	defer func() {
		err = multierr.Combine(err, utils.TryClose(li.Lidar))
	}()
	return li.Lidar.Stop(context.Background()) // because we started it
}

func (li *Lidar) clone() *Lidar {
	cloned := *li
	marked := li.markedRotatedMats.Load()
	if marked == nil {
		return &cloned
	}
	cloned.markedRotatedMats.Store(marked)
	return &cloned
}

func (li *Lidar) setLidar(lidar lidar.Lidar) {
	li.Lidar = lidar
}

func (li *Lidar) StartCalibration(ctx context.Context) error {
	return nil
}

func (li *Lidar) StopCalibration(ctx context.Context) error {
	return nil
}

func (li *Lidar) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := li.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (li *Lidar) rotationResolution(ctx context.Context) (float64, error) {
	angularRes, err := li.Lidar.AngularResolution(ctx)
	if err != nil {
		return math.NaN(), err
	}
	if angularRes <= .5 {
		return .5, nil
	}
	return 1, nil
}

// Heading returns the best matching heading from a series of predictions
// based on mean squared errors originating from a marked scan.
func (li *Lidar) Heading(ctx context.Context) (float64, error) {
	markedRotatedMatsIfc := li.markedRotatedMats.Load()
	if markedRotatedMatsIfc == nil {
		return math.NaN(), nil
	}
	markedRotatedMats := markedRotatedMatsIfc.([][]rotatedMat)
	measureMat, err := li.scanToVec2Matrix(ctx)
	if err != nil {
		return math.NaN(), err
	}

	var results utils.Vec2Fs
	if err := li.groupWorkParallel(
		ctx,
		func(numGroups int) {
			results = make(utils.Vec2Fs, numGroups)
		},
		func(groupNum, size int) (memberWorkFunc, groupWorkDoneFunc) {
			minDist := math.MaxFloat64
			var minTheta float64
			rotatedMats := markedRotatedMats[groupNum]
			return func(memberNum int, theta float64) {
					rotatedS := rotatedMats[memberNum]
					dist := rotatedS.mat.DistanceMSETo(measureMat)
					if dist < minDist {
						minDist = dist
						minTheta = theta
					}
				}, func() {
					results[groupNum] = []float64{minDist, minTheta}
				}
		},
	); err != nil {
		return math.NaN(), err
	}
	sort.Sort(results)
	return results[0][1], nil
}

func (li *Lidar) scanToVec2Matrix(ctx context.Context) (*utils.Vec2Matrix, error) {
	var measurements lidar.Measurements
	attempts := 5
	for i := 0; i < attempts; i++ {
		var err error
		measurements, err = li.Lidar.Scan(ctx, lidar.ScanOptions{Count: 5, NoFilter: true})
		if err != nil && i+1 >= attempts {
			return nil, err
		}
		if err == nil {
			break
		}
	}
	if len(measurements) == 0 {
		return &utils.Vec2Matrix{}, nil
	}
	measureMat := mat.NewDense(3, len(measurements), nil)
	for i, next := range measurements {
		x, y := next.Coords()
		measureMat.Set(0, i, x)
		measureMat.Set(1, i, y)
		measureMat.Set(2, i, 1)
	}
	return (*utils.Vec2Matrix)(measureMat), nil
}

type rotatedMat struct {
	mat   *utils.Vec2Matrix
	theta float64
}

// Mark records a scan into a matrix as well as produces perfect
// rotations from that scan in order to use on future MSE calculations
// when predicting a heading.
func (li *Lidar) Mark(ctx context.Context) error {
	measureMat, err := li.scanToVec2Matrix(ctx)
	if err != nil {
		return err
	}
	var markedRotatedMats [][]rotatedMat
	if err := li.groupWorkParallel(
		ctx,
		func(numGroups int) {
			markedRotatedMats = make([][]rotatedMat, numGroups)
		},
		func(groupNum, size int) (memberWorkFunc, groupWorkDoneFunc) {
			rotatedMats := make([]rotatedMat, 0, size)
			return func(memberNum int, theta float64) {
					rotated := measureMat.RotateMatrixAbout(0, 0, theta)
					rotatedMats = append(rotatedMats, rotatedMat{rotated, theta})
				}, func() {
					markedRotatedMats[groupNum] = rotatedMats
				}
		},
	); err != nil {
		return err
	}
	li.markedRotatedMats.Store(markedRotatedMats)
	return nil
}

const maxTheta = 360

var parallelFactor = runtime.GOMAXPROCS(0)

func init() {
	if parallelFactor <= 0 {
		parallelFactor = 1
	}
	for parallelFactor != 1 {
		if maxTheta%parallelFactor == 0 {
			break
		}
		parallelFactor--
	}
}

type beforeParallelGroupWorkFunc func(groupSize int)
type memberWorkFunc func(memberNum int, theta float64)
type groupWorkDoneFunc func()
type groupWorkFunc func(groupNum, size int) (memberWorkFunc, groupWorkDoneFunc)

func (li *Lidar) groupWorkParallel(ctx context.Context, before beforeParallelGroupWorkFunc, groupWork groupWorkFunc) error {
	thetaParts := maxTheta / float64(parallelFactor)
	rotRes, err := li.rotationResolution(ctx)
	if err != nil {
		return err
	}
	numRotations := int(math.Ceil(maxTheta / rotRes))
	groupSize := int(math.Ceil(float64(numRotations) / float64(parallelFactor)))

	numGroups := parallelFactor
	before(numGroups)

	var wait sync.WaitGroup
	wait.Add(numGroups)
	for groupNum := 0; groupNum < numGroups; groupNum++ {
		groupNumCopy := groupNum
		utils.PanicCapturingGo(func() {
			defer wait.Done()
			groupNum := groupNumCopy

			memberWork, groupWorkDone := groupWork(groupNum, groupSize)
			from := thetaParts * float64(groupNum)
			to := thetaParts * float64(groupNum+1)
			memberNum := 0
			for theta := from; theta < to; theta += rotRes {
				memberWork(memberNum, theta)
				memberNum++
			}
			groupWorkDone()
		})
	}
	wait.Wait()
	return nil
}
