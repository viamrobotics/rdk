package fake

import (
	"fmt"
	"hash/fnv"
	"image"
	"math/rand"
	"strconv"
	"sync"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/utils"
)

const LidarDeviceType = "fake"

func init() {
	lidar.RegisterDeviceType(LidarDeviceType, lidar.DeviceTypeRegistration{
		New: func(desc lidar.DeviceDescription) (lidar.Device, error) {
			seed, err := strconv.ParseInt(desc.Path, 10, 64)
			if err != nil {
				return nil, err
			}
			device := NewLidar()
			device.SetSeed(seed)
			return device, nil
		},
	})
}

// A Lidar outputs noisy scans based on its current position and seed.
type Lidar struct {
	mu         sync.Mutex
	posX, posY int
	started    bool
	seed       int64
}

func NewLidar() *Lidar {
	return &Lidar{}
}

func (l *Lidar) SetPosition(pos image.Point) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.posX = pos.X
	l.posY = pos.Y
}

func (l *Lidar) Seed() int64 {
	return l.seed
}

func (l *Lidar) SetSeed(seed int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seed = seed
}

func (l *Lidar) Start() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = true
}

func (l *Lidar) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = false
}

func (l *Lidar) Close() error {
	l.Stop()
	return nil
}

func (l *Lidar) Range() int {
	return 25
}

func (l *Lidar) AngularResolution() float64 {
	return 1
}

func (l *Lidar) Bounds() (image.Point, error) {
	x := l.Range() * 2
	return image.Point{x, x}, nil
}

func (l *Lidar) Scan(options lidar.ScanOptions) (lidar.Measurements, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.started {
		return nil, nil
	}
	h := fnv.New64()
	if _, err := h.Write([]byte(fmt.Sprintf("%d,%d", l.posX, l.posY))); err != nil {
		return nil, err
	}
	r := rand.NewSource(int64(h.Sum64()) + l.seed)
	measurements := make(lidar.Measurements, 0, 360)
	getFloat64 := func() float64 {
	again:
		f := float64(r.Int63()) / (1 << 63)
		if f == 1 {
			goto again // resample
		}
		return f
	}
	for i := 0; i < cap(measurements); i++ {
		measurements = append(measurements, lidar.NewMeasurement(
			utils.DegToRad(getFloat64()*360), getFloat64()*float64(l.Range())))
	}
	return measurements, nil
}
