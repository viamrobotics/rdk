package lidar

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/sensor/compass"
	"github.com/viamrobotics/robotcore/utils"

	"gonum.org/v1/gonum/mat"
)

type Device struct {
	lidar.Device
	markedRotatedMats atomic.Value
}

func From(lidarDevice lidar.Device) compass.RelativeDevice {
	return &Device{Device: lidarDevice}
}

func New(deviceDesc lidar.DeviceDescription) (compass.RelativeDevice, error) {
	lidarDevice, err := lidar.CreateDevice(deviceDesc)
	if err != nil {
		return nil, err
	}

	return &Device{Device: lidarDevice}, nil
}

func (d *Device) clone() *Device {
	cloned := *d
	cloned.markedRotatedMats.Store(d.markedRotatedMats.Load())
	return &cloned
}

func (d *Device) setDevice(lidarDevice lidar.Device) {
	d.Device = lidarDevice
}

func (d *Device) StartCalibration() error {
	return nil
}

func (d *Device) StopCalibration() error {
	return nil
}

func (d *Device) Readings() ([]interface{}, error) {
	heading, err := d.Heading()
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (d *Device) rotationResolution() float64 {
	angularRes := d.Device.AngularResolution()
	if angularRes <= .5 {
		return .5
	}
	return 1
}

func (d *Device) Heading() (float64, error) {
	markedRotatedMatsIfc := d.markedRotatedMats.Load()
	if markedRotatedMatsIfc == nil {
		return math.NaN(), nil
	}
	markedRotatedMats := markedRotatedMatsIfc.([][]rotatedMat)
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return math.NaN(), err
	}

	var results utils.Vec2Fs
	d.groupWorkParallel(
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
	)
	sort.Sort(results)
	return results[0][1], nil
}

func (d *Device) scanToVec2Matrix() (*utils.Vec2Matrix, error) {
	var measurements lidar.Measurements
	attempts := 5
	for i := 0; i < attempts; i++ {
		var err error
		measurements, err = d.Device.Scan(lidar.ScanOptions{Count: 5, NoFilter: true})
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
	var angSum float64
	var distSum float64
	for i, next := range measurements {
		angSum += next.Angle()
		distSum += next.Distance()
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

func (d *Device) Mark() error {
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return err
	}
	var markedRotatedMats [][]rotatedMat
	d.groupWorkParallel(
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
	)
	d.markedRotatedMats.Store(markedRotatedMats)
	return nil
}

var parallelFactor = runtime.NumCPU()

type beforeParallelGroupWorkFunc func(groupSize int)
type memberWorkFunc func(memberNum int, theta float64)
type groupWorkDoneFunc func()
type groupWorkFunc func(groupNum, size int) (memberWorkFunc, groupWorkDoneFunc)

func (d *Device) groupWorkParallel(before beforeParallelGroupWorkFunc, groupWork groupWorkFunc) {
	maxTheta := 360
	if maxTheta%parallelFactor != 0 {
		panic(fmt.Errorf("parallelFactor %d not evenly divisible", parallelFactor))
	}
	thetaParts := maxTheta / parallelFactor
	rotRes := d.rotationResolution()
	numRotations := int(math.Ceil(float64(maxTheta) / rotRes))
	groupSize := int(math.Ceil(float64(numRotations) / float64(parallelFactor)))

	numGroups := parallelFactor
	before(numGroups)

	var wait sync.WaitGroup
	wait.Add(numGroups)
	for groupNum := 0; groupNum < numGroups; groupNum++ {
		groupNumCopy := groupNum
		go func() {
			defer wait.Done()
			groupNum := groupNumCopy

			memberWork, groupWorkDone := groupWork(groupNum, groupSize)
			from := float64(thetaParts * groupNum)
			to := float64(thetaParts * (groupNum + 1))
			memberNum := 0
			for theta := from; theta < to; theta += rotRes {
				memberWork(memberNum, theta)
				memberNum++
			}
			groupWorkDone()
		}()
	}
	wait.Wait()
}
