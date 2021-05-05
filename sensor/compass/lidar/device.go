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

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/mat"
)

const ModelName = "lidar"

func init() {
	api.RegisterSensor(compass.DeviceType, ModelName, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (sensor.Device, error) {
		return New(ctx, config, logger)
	})
}

type Device struct {
	lidar.Device
	markedRotatedMats atomic.Value // [][]rotatedMat
}

func From(lidarDevice lidar.Device) compass.RelativeDevice {
	return &Device{Device: lidarDevice}
}

func New(ctx context.Context, config api.ComponentConfig, logger golog.Logger) (compass.RelativeDevice, error) {
	lidarType := config.Attributes.GetString("type")
	f := api.LidarDeviceLookup(lidarType)
	if f == nil {
		return nil, fmt.Errorf("unknown lidar model: %s", lidarType)
	}
	lidarDevice, err := f(ctx, nil, config, logger)
	if err != nil {
		return nil, err
	}
	if err := lidarDevice.Start(ctx); err != nil {
		return nil, err
	}
	return &Device{Device: lidarDevice}, nil
}

func (d *Device) Desc() sensor.DeviceDescription {
	return sensor.DeviceDescription{compass.RelativeDeviceType, ""}
}

func (d *Device) Close() (err error) {
	defer func() {
		err = multierr.Combine(err, utils.TryClose(d.Device))
	}()
	return d.Device.Stop(context.Background()) // because we started it
}

func (d *Device) clone() *Device {
	cloned := *d
	marked := d.markedRotatedMats.Load()
	if marked == nil {
		return &cloned
	}
	cloned.markedRotatedMats.Store(marked)
	return &cloned
}

func (d *Device) setDevice(lidarDevice lidar.Device) {
	d.Device = lidarDevice
}

func (d *Device) StartCalibration(ctx context.Context) error {
	return nil
}

func (d *Device) StopCalibration(ctx context.Context) error {
	return nil
}

func (d *Device) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := d.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (d *Device) rotationResolution(ctx context.Context) (float64, error) {
	angularRes, err := d.Device.AngularResolution(ctx)
	if err != nil {
		return math.NaN(), err
	}
	if angularRes <= .5 {
		return .5, nil
	}
	return 1, nil
}

func (d *Device) Heading(ctx context.Context) (float64, error) {
	markedRotatedMatsIfc := d.markedRotatedMats.Load()
	if markedRotatedMatsIfc == nil {
		return math.NaN(), nil
	}
	markedRotatedMats := markedRotatedMatsIfc.([][]rotatedMat)
	measureMat, err := d.scanToVec2Matrix(ctx)
	if err != nil {
		return math.NaN(), err
	}

	var results utils.Vec2Fs
	if err := d.groupWorkParallel(
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

func (d *Device) scanToVec2Matrix(ctx context.Context) (*utils.Vec2Matrix, error) {
	var measurements lidar.Measurements
	attempts := 5
	for i := 0; i < attempts; i++ {
		var err error
		measurements, err = d.Device.Scan(ctx, lidar.ScanOptions{Count: 5, NoFilter: true})
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

func (d *Device) Mark(ctx context.Context) error {
	measureMat, err := d.scanToVec2Matrix(ctx)
	if err != nil {
		return err
	}
	var markedRotatedMats [][]rotatedMat
	if err := d.groupWorkParallel(
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
	d.markedRotatedMats.Store(markedRotatedMats)
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

func (d *Device) groupWorkParallel(ctx context.Context, before beforeParallelGroupWorkFunc, groupWork groupWorkFunc) error {
	thetaParts := maxTheta / float64(parallelFactor)
	rotRes, err := d.rotationResolution(ctx)
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
