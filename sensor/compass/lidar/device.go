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
	mark atomic.Value
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

func (d *Device) Heading() (float64, error) {
	rotatedMatsIfc := d.mark.Load()
	if rotatedMatsIfc == nil {
		return math.NaN(), nil
	}
	rotatedMats := rotatedMatsIfc.([]*utils.Vec2Matrix)
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return math.NaN(), err
	}
	parallelFactor := runtime.NumCPU()
	maxTheta := 360
	if maxTheta%parallelFactor != 0 {
		return math.NaN(), fmt.Errorf("parallelFactor %d not evenly divisible", parallelFactor)
	}
	thetaParts := maxTheta / parallelFactor
	var wait sync.WaitGroup
	wait.Add(parallelFactor)
	results := make(utils.Vec2Fs, parallelFactor)
	groupSize := maxTheta / parallelFactor
	for i := 0; i < parallelFactor; i++ {
		iCopy := i
		go func() {
			defer wait.Done()
			i := iCopy
			step := d.Device.AngularResolution()
			minDist := math.MaxFloat64
			var minTheta float64
			from := float64(thetaParts * i)
			to := float64(thetaParts * (i + 1))
			angleNum := 0
			for theta := from; theta < to; theta += step {
				rotated := rotatedMats[(i*groupSize)+angleNum]
				angleNum++
				dist := rotated.DistanceMSETo(measureMat)
				if dist < minDist {
					minDist = dist
					minTheta = theta
				}
			}
			results[i] = []float64{minDist, minTheta}
		}()
	}
	wait.Wait()
	sort.Sort(results)
	return math.Mod(float64(360)-results[0][1], 360), nil
}

func (d *Device) scanToVec2Matrix() (*utils.Vec2Matrix, error) {
	var measurements lidar.Measurements
	attempts := 5
	for i := 0; i < attempts; i++ {
		var err error
		measurements, err = d.Device.Scan()
		if err != nil && i+1 >= attempts {
			return nil, err
		}
		if err == nil {
			break
		}
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

func (d *Device) Mark() error {
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return err
	}

	parallelFactor := runtime.NumCPU()
	maxTheta := 360
	if maxTheta%parallelFactor != 0 {
		return fmt.Errorf("parallelFactor %d not evenly divisible", parallelFactor)
	}
	groupSize := maxTheta / parallelFactor
	angularRes := d.Device.AngularResolution()
	thetaParts := maxTheta / parallelFactor
	var wait sync.WaitGroup
	wait.Add(parallelFactor)
	numRotations := int(math.Ceil(float64(maxTheta) / angularRes))
	rotatedMats := make([]*utils.Vec2Matrix, numRotations)
	for i := 0; i < parallelFactor; i++ {
		iCopy := i
		go func() {
			defer wait.Done()
			i := iCopy
			step := angularRes
			from := float64(thetaParts * i)
			to := float64(thetaParts * (i + 1))
			angleNum := 0
			for theta := from; theta < to; theta += step {
				rotated := measureMat.RotateMatrixAbout(0, 0, theta)
				rotatedMats[(i*groupSize)+angleNum] = rotated
				angleNum++
			}
		}()
	}
	wait.Wait()

	d.mark.Store(rotatedMats)
	return nil
}
