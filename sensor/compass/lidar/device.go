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
	mark    atomic.Value
	markMat atomic.Value
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

var parallelFactor = runtime.NumCPU()

func (d *Device) Heading() (float64, error) {
	rotatedMatsIfc := d.mark.Load()
	if rotatedMatsIfc == nil {
		return math.NaN(), nil
	}
	rotatedMatss := rotatedMatsIfc.([][]rotS)
	origMat := d.markMat.Load().(*utils.Vec2Matrix)
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return math.NaN(), err
	}

	angularRes := d.Device.AngularResolution()

	// fast path
	if false {
		const searchSize = 75 // always >= 2
		var findDistance func(from, to float64) float64
		findDistance = func(from, to float64) float64 {
			if to-from <= angularRes {
				return from
			}

			span := to - from
			spanSplit := span / (searchSize - 1)

			angs := make([]float64, 0, searchSize)
			dists := make([]float64, 0, searchSize)
			for i := 0; i < searchSize; i++ {
				ang := from + (float64(i) * spanSplit)
				rot := origMat.RotateMatrixAbout(0, 0, ang)
				angs = append(angs, ang)
				dists = append(dists, rot.DistanceMSETo(measureMat))
			}

			minIdx := 0
			minDist := dists[0]
			for i := 1; i < len(dists); i++ {
				if dists[i] < minDist {
					minIdx = i
					minDist = dists[i]
				}
			}
			if minIdx == 0 {
				return findDistance(angs[minIdx], angs[minIdx+1])
			}
			if minIdx == len(dists)-1 {
				return findDistance(angs[minIdx-1], angs[minIdx])
			}
			if math.Abs(dists[minIdx-1]-minDist) < math.Abs(dists[minIdx+1]-minDist) {
				return findDistance(angs[minIdx-1], angs[minIdx])
			}
			return findDistance(angs[minIdx], angs[minIdx+1])
		}
		return findDistance(0, 360), nil
	}

	maxTheta := 360
	if maxTheta%parallelFactor != 0 {
		return math.NaN(), fmt.Errorf("parallelFactor %d not evenly divisible", parallelFactor)
	}
	thetaParts := maxTheta / parallelFactor
	var wait sync.WaitGroup
	wait.Add(parallelFactor)
	results := make(utils.Vec2Fs, parallelFactor)
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
			rotatedMats := rotatedMatss[i]
			for theta := from; theta < to; theta += step {
				rotatedS := rotatedMats[angleNum]
				angleNum++
				dist := rotatedS.mat.DistanceMSETo(measureMat)
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

type rotS struct {
	mat   *utils.Vec2Matrix
	theta float64
}

func (d *Device) Mark() error {
	measureMat, err := d.scanToVec2Matrix()
	if err != nil {
		return err
	}

	maxTheta := 360
	if maxTheta%parallelFactor != 0 {
		return fmt.Errorf("parallelFactor %d not evenly divisible", parallelFactor)
	}
	angularRes := d.Device.AngularResolution()
	thetaParts := maxTheta / parallelFactor
	var wait sync.WaitGroup
	wait.Add(parallelFactor)
	numRotations := int(math.Ceil(float64(maxTheta) / angularRes))
	rotatedMatss := make([][]rotS, parallelFactor)
	groupSize := int(math.Ceil(float64(numRotations) / float64(parallelFactor)))
	for i := 0; i < parallelFactor; i++ {
		iCopy := i
		go func() {
			defer wait.Done()
			rotatedMats := make([]rotS, 0, groupSize)
			i := iCopy
			step := angularRes
			from := float64(thetaParts * i)
			to := float64(thetaParts * (i + 1))
			for theta := from; theta < to; theta += step {
				rotated := measureMat.RotateMatrixAbout(0, 0, theta)
				rotatedMats = append(rotatedMats, rotS{rotated, theta})
			}
			rotatedMatss[i] = rotatedMats
		}()
	}
	wait.Wait()

	d.mark.Store(rotatedMatss)
	d.markMat.Store(measureMat)
	return nil
}
