package lidar

import (
	"fmt"
	"hash/fnv"
	"image"
	"math/rand"
	"strconv"
	"sync"

	"github.com/echolabsinc/robotcore/utils"
)

func init() {
	RegisterDeviceType(DeviceTypeFake, DeviceTypeRegistration{
		New: func(desc DeviceDescription) (Device, error) {
			seed, err := strconv.ParseInt(desc.Path, 10, 64)
			if err != nil {
				return nil, err
			}
			device := NewFakedNoiseDevice()
			device.SetSeed(seed)
			return device, nil
		},
	})
}

// A FakedNoiseDevice outputs noisy scans based on
// its current position and seed.
type FakedNoiseDevice struct {
	mu         sync.Mutex
	posX, posY int
	started    bool
	seed       int64
}

func NewFakedNoiseDevice() *FakedNoiseDevice {
	return &FakedNoiseDevice{}
}

func (fnd *FakedNoiseDevice) SetPosition(pos image.Point) {
	fnd.mu.Lock()
	defer fnd.mu.Unlock()
	fnd.posX = pos.X
	fnd.posY = pos.Y
}

func (fnd *FakedNoiseDevice) SetSeed(seed int64) {
	fnd.mu.Lock()
	defer fnd.mu.Unlock()
	fnd.seed = seed
}

func (fnd *FakedNoiseDevice) Start() {
	fnd.mu.Lock()
	defer fnd.mu.Unlock()
	fnd.started = true
}

func (fnd *FakedNoiseDevice) Stop() {
	fnd.mu.Lock()
	defer fnd.mu.Unlock()
	fnd.started = false
}

func (fnd *FakedNoiseDevice) Close() {
	fnd.Stop()
}

func (fnd *FakedNoiseDevice) Range() int {
	return 25
}

func (fnd *FakedNoiseDevice) Bounds() (image.Point, error) {
	x := fnd.Range() * 2
	return image.Point{x, x}, nil
}

func (fnd *FakedNoiseDevice) Scan() (Measurements, error) {
	fnd.mu.Lock()
	defer fnd.mu.Unlock()
	if !fnd.started {
		return nil, nil
	}
	h := fnv.New64()
	if _, err := h.Write([]byte(fmt.Sprintf("%d,%d", fnd.posX, fnd.posY))); err != nil {
		return nil, err
	}
	r := rand.NewSource(int64(h.Sum64()) + fnd.seed)
	measurements := make(Measurements, 0, 360)
	getFloat64 := func() float64 {
	again:
		f := float64(r.Int63()) / (1 << 63)
		if f == 1 {
			goto again // resample
		}
		return f
	}
	for i := 0; i < cap(measurements); i++ {
		measurements = append(measurements, NewMeasurement(
			utils.DegToRad(getFloat64()*360), getFloat64()*float64(fnd.Range())))
	}
	return measurements, nil
}
