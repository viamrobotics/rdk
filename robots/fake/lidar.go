package fake

import (
	"context"
	"fmt"
	"hash/fnv"
	"image"
	"math/rand"
	"strconv"
	"sync"

	"go.viam.com/robotcore/lidar"
)

const LidarDeviceType = "fake"

func init() {
	lidar.RegisterDeviceType(LidarDeviceType, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
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

func (l *Lidar) Info(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

func (l *Lidar) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = true
	return nil
}

func (l *Lidar) Stop(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = false
	return nil
}

func (l *Lidar) Close(ctx context.Context) error {
	return l.Stop(ctx)
}

func (l *Lidar) Range(ctx context.Context) (int, error) {
	return 25, nil
}

func (l *Lidar) AngularResolution(ctx context.Context) (float64, error) {
	return 1, nil
}

func (l *Lidar) Bounds(ctx context.Context) (image.Point, error) {
	r, err := l.Range(ctx)
	if err != nil {
		return image.Point{}, err
	}
	x := r * 2
	return image.Point{x, x}, nil
}

func (l *Lidar) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
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
	rang, err := l.Range(ctx)
	if err != nil {
		return nil, err
	}
	for i := 0; i < cap(measurements); i++ {
		measurements = append(measurements, lidar.NewMeasurement(
			getFloat64()*360, getFloat64()*float64(rang)))
	}
	return measurements, nil
}
