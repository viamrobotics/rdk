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
	"gonum.org/v1/gonum/stat"
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
	rotatedMats := rotatedMatsIfc.([]*mat.Dense)
	measureMat, err := d.scanToMat()
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
	results := make(vec2s, parallelFactor)
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
				dist := distanceMSE(rotated, measureMat)
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

func (d *Device) scanToMat() (*mat.Dense, error) {
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
	return measureMat, nil
}

func (d *Device) Mark() error {
	measureMat, err := d.scanToMat()
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
	rotatedMats := make([]*mat.Dense, numRotations)
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
				thetaRad := utils.DegToRad(theta)
				rotated := rotateMatrixAbout(measureMat, 0, 0, thetaRad)
				rotatedMats[(i*groupSize)+angleNum] = rotated
				angleNum++
			}
		}()
	}
	wait.Wait()

	d.mark.Store(rotatedMats)
	return nil
}

// TODO(erd): move to math
// ccw
func rotationMatrixAbout(x, y, theta float64) mat.Matrix {
	tNeg1 := mat.NewDense(3, 3, []float64{
		1, 0, x,
		0, 1, y,
		0, 0, 1,
	})
	rot := mat.NewDense(3, 3, []float64{
		math.Cos(theta), -math.Sin(theta), 0,
		math.Sin(theta), math.Cos(theta), 0,
		0, 0, 1,
	})
	t := mat.NewDense(3, 3, []float64{
		1, 0, -x,
		0, 1, -y,
		0, 0, 1,
	})
	var rotFinal mat.Dense
	rotFinal.Product(tNeg1, rot, t)
	return &rotFinal
}

func rotateMatrixAbout(src mat.Matrix, x, y, theta float64) *mat.Dense {
	rot := rotationMatrixAbout(x, y, theta)
	var rotated mat.Dense
	rotated.Mul(rot, src)
	return &rotated
}

func sortMat(target *mat.Dense) *mat.Dense {
	numCols := target.RowView(0).Len()
	cols := make([][]float64, 0, target.RowView(0).Len())
	targetT := mat.DenseCopyOf(target.T())
	for i := 0; i < numCols; i++ {
		cols = append(cols, targetT.RawRowView(i))
	}
	sort.Sort(vec2s(cols))
	r, c := target.Dims()
	sortedMat := mat.NewDense(r, c, nil)
	for i := 0; i < numCols; i++ {
		sortedMat.SetCol(i, cols[i])
	}
	return sortedMat
}

type vec2s [][]float64

func (vs vec2s) Len() int {
	return len(vs)
}

func (vs vec2s) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs vec2s) Less(i, j int) bool {
	if vs[i][0] < vs[j][0] {
		return true
	}
	if vs[i][0] > vs[j][0] {
		return false
	}
	return vs[i][1] < vs[j][1]
}

func distanceMSE(from *mat.Dense, to *mat.Dense) float64 {
	from = sortMat(from)
	to = sortMat(to)
	_, fromLen := from.Dims()
	_, toLen := to.Dims()
	numRows := from.ColView(0).Len()
	compareFrom := from
	compareTo := to
	if fromLen < toLen {
		compareTo = mat.DenseCopyOf(to.Slice(0, numRows, 0, fromLen))
	} else if fromLen > toLen {
		compareFrom = mat.DenseCopyOf(from.Slice(0, numRows, 0, toLen))
	}

	var subbed mat.Dense
	subbed.Sub(compareFrom, compareTo)

	var powwed mat.Dense
	powwed.MulElem(&subbed, &subbed)

	var plussed mat.Dense
	plussed.Add(powwed.RowView(0), powwed.RowView(1))

	var rooted mat.Dense
	rooted.Apply(func(i, j int, v float64) float64 { return math.Sqrt(v) }, &plussed)

	return stat.Mean(rooted.RawRowView(0), nil)
}
