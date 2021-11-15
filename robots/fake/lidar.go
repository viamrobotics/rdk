package fake

import (
	"context"
	_ "embed" // used to import model frame
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
)

//go:embed lidar_model.json
var lidarmodel []byte

// LidarType uses the fake model name.
const LidarType = ModelName

func init() {
	registry.RegisterLidar(LidarType, registry.Lidar{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
			if config.Host == "" {
				config.Host = "0"
			}
			host := strings.TrimPrefix(config.Host, "fake-")
			seed, err := strconv.ParseInt(host, 10, 64)
			if err != nil {
				return nil, err
			}
			device, err := NewLidar(config)
			if err != nil {
				return nil, err
			}
			device.SetSeed(seed)
			return device, nil
		},
	})
}

// A Lidar outputs noisy scans based on its current position and seed.
type Lidar struct {
	Name       string
	mu         *sync.Mutex
	posX, posY float64
	started    bool
	seed       int64
	model      *referenceframe.Model
}

// NewLidar returns a new fake lidar.
func NewLidar(cfg config.Component) (*Lidar, error) {
	model, err := referenceframe.ParseJSON(lidarmodel, "")
	if err != nil {
		return nil, err
	}
	name := cfg.Name
	return &Lidar{Name: name, mu: &sync.Mutex{}, model: model}, nil
}

// ModelFrame returns the dynamic frame of the model
func (l *Lidar) ModelFrame() *referenceframe.Model {
	return l.model
}

// SetPosition sets the given position.
func (l *Lidar) SetPosition(pos r2.Point) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.posX = pos.X
	l.posY = pos.Y
}

// Seed returns the seed being used for random number generation.
func (l *Lidar) Seed() int64 {
	return l.seed
}

// SetSeed sets the seed to be used for random number generation.
func (l *Lidar) SetSeed(seed int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seed = seed
}

// Info returns nothing.
func (l *Lidar) Info(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

// Start marks the lidar as started.
func (l *Lidar) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = true
	return nil
}

// Stop marks the lidar as stopped.
func (l *Lidar) Stop(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = false
	return nil
}

// Close stops the lidar.
func (l *Lidar) Close() error {
	return l.Stop(context.Background())
}

// Range always returns the same value.
func (l *Lidar) Range(ctx context.Context) (float64, error) {
	return 25, nil
}

// AngularResolution always returns the same value.
func (l *Lidar) AngularResolution(ctx context.Context) (float64, error) {
	return 1, nil
}

// Bounds always returns the same value.
func (l *Lidar) Bounds(ctx context.Context) (r2.Point, error) {
	r, err := l.Range(ctx)
	if err != nil {
		return r2.Point{}, err
	}
	x := r * 2
	return r2.Point{x, x}, nil
}

// Scan returns random measurements based off the currently set seed.
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
