package fake

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
)

const LidarDeviceType = ModelName

func init() {
	api.RegisterLidarDevice(LidarDeviceType, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (lidar.Device, error) {
		if config.Host == "" {
			config.Host = "0"
		}
		host := strings.TrimPrefix(config.Host, "fake-")
		seed, err := strconv.ParseInt(host, 10, 64)
		if err != nil {
			return nil, err
		}
		device := NewLidar(config.Name)
		device.SetSeed(seed)
		return device, nil
	})
}

// A Lidar outputs noisy scans based on its current position and seed.
type Lidar struct {
	Name       string
	mu         sync.Mutex
	posX, posY float64
	started    bool
	seed       int64
}

func NewLidar(name string) *Lidar {
	return &Lidar{Name: name}
}

func (l *Lidar) SetPosition(pos r2.Point) {
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

func (l *Lidar) Close() error {
	return l.Stop(context.Background())
}

func (l *Lidar) Range(ctx context.Context) (float64, error) {
	return 25, nil
}

func (l *Lidar) AngularResolution(ctx context.Context) (float64, error) {
	return 1, nil
}

func (l *Lidar) Bounds(ctx context.Context) (r2.Point, error) {
	r, err := l.Range(ctx)
	if err != nil {
		return r2.Point{}, err
	}
	x := r * 2
	return r2.Point{x, x}, nil
}

func (l *Lidar) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.started {
		return nil, nil
	}
	h := fnv.New64()
	if _, err := h.Write([]byte(fmt.Sprintf("%v,%v", l.posX, l.posY))); err != nil {
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
			getFloat64()*360, getFloat64()*rang))
	}
	return measurements, nil
}
