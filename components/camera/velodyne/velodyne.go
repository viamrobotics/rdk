// Package velodyne implements a general velodyne LIDAR as a camera.
package velodyne

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.einride.tech/vlp16"
	gutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type channelConfig struct {
	elevationAngle float64
	azimuthOffset  float64
	verticalOffset float64
}

type productConfig []channelConfig

var allProductData = map[vlp16.ProductID]productConfig{
	vlp16.ProductIDVLP32C: {
		channelConfig{-25, 1.4, 0},
		channelConfig{-1, -4.2, 0},
		channelConfig{-1.667, 1.4, 0},
		channelConfig{-15.639, -1.4, 0},
		channelConfig{-11.31, 1.4, 0},
		channelConfig{0, -1.4, 0},
		channelConfig{-0.667, 4.2, 0},
		channelConfig{-8.843, -1.4, 0},
		channelConfig{-7.254, 1.4, 0},
		channelConfig{0.333, -4.2, 0},
		channelConfig{-0.333, 1.4, 0},
		channelConfig{-6.148, -1.4, 0},
		channelConfig{-5.333, 4.2, 0},
		channelConfig{1.333, -1.4, 0},
		channelConfig{0.667, 4.2, 0},
		channelConfig{-4, -1.4, 0},
		channelConfig{-4.667, 1.4, 0},
		channelConfig{1.667, -4.2, 0},
		channelConfig{1, 1.4, 0},
		channelConfig{-3.667, -4.2, 0},
		channelConfig{-3.333, 4.2, 0},
		channelConfig{3.333, -1.4, 0},
		channelConfig{2.333, 1.4, 0},
		channelConfig{-2.667, -1.4, 0},
		channelConfig{-3, 1.4, 0},
		channelConfig{7, -1.4, 0},
		channelConfig{4.667, 1.4, 0},
		channelConfig{-2.333, -4.2, 0},
		channelConfig{-2, 4.2, 0},
		channelConfig{15, -1.4, 0},
		channelConfig{10.333, 1.4, 0},
		channelConfig{-1.333, -1.4, 0},
	},
	vlp16.ProductIDVLP16: { // This also covers VLP Puck LITE
		channelConfig{-15, 0, 11.2},
		channelConfig{1, 0, -0.7},
		channelConfig{-13, 0, 9.7},
		channelConfig{3, 0, -2.2},
		channelConfig{11, 0, 8.1},
		channelConfig{5, 0, -3.7},
		channelConfig{-9, 0, 6.6},
		channelConfig{7, 0, -5.1},
		channelConfig{-7, 0, 5.1},
		channelConfig{9, 0, -6.6},
		channelConfig{-5, 0, 3.7},
		channelConfig{11, 0, -8.1},
		channelConfig{-3, 0, 2.2},
		channelConfig{13, 0, -9.7},
		channelConfig{-1, 0, 0.7},
		channelConfig{15, 0, -11.2},
	},
	vlp16.ProductIDPuckHiRes: {
		channelConfig{-10, 0, 7.4},
		channelConfig{.67, 0, -0.9},
		channelConfig{-8.67, 0, 6.5},
		channelConfig{2, 0, -1.8},
		channelConfig{-7.33, 0, 5.5},
		channelConfig{3.33, 0, -2.7},
		channelConfig{-6, 0, 4.6},
		channelConfig{4.67, 0, -3.7},
		channelConfig{-4.67, 0, 3.7},
		channelConfig{6, 0, -4.6},
		channelConfig{-3.3, 0, 2.7},
		channelConfig{7.33, 0, -5.5},
		channelConfig{-2, 0, 1.8},
		channelConfig{8.67, 0, -6.5},
		channelConfig{-0.67, 0, 0.9},
		channelConfig{10, 0, -7.4},
	},
}

// Config is the config for a veldoyne LIDAR.
type Config struct {
	Port  int `json:"port"`
	TTLMS int `json:"ttl_ms"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Port == 0 {
		return nil, gutils.NewConfigValidationFieldRequiredError(path, "port")
	}

	if conf.TTLMS == 0 {
		return nil, gutils.NewConfigValidationFieldRequiredError(path, "ttl_ms")
	}
	return nil, nil
}

var model = resource.DefaultModelFamily.WithModel("velodyne")

func init() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}

				port := newConf.Port
				if port == 0 {
					port = 2368
				}

				ttl := newConf.TTLMS
				if ttl == 0 {
					return nil, errors.New("need to specify a ttl")
				}

				return New(ctx, conf.ResourceName(), logger, port, ttl)
			},
		})
}

type client struct {
	resource.Named
	resource.AlwaysRebuild
	bindAddress     string
	ttlMilliseconds int

	logger golog.Logger

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	mu sync.Mutex

	lastError error
	product   vlp16.ProductID
	ip        string
	packets   []vlp16.Packet
}

// New creates a connection to a Velodyne lidar and generates pointclouds from it.
func New(ctx context.Context, name resource.Name, logger golog.Logger, port, ttlMilliseconds int) (camera.Camera, error) {
	bindAddress := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := vlp16.ListenUDP(ctx, bindAddress)
	if err != nil {
		return nil, err
	}
	// Listen for and print packets.

	c := &client{
		Named:           name.AsNamed(),
		bindAddress:     bindAddress,
		ttlMilliseconds: ttlMilliseconds,
		logger:          logger,
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	c.activeBackgroundWorkers.Add(1)
	gutils.PanicCapturingGo(func() {
		c.run(cancelCtx, listener)
	})

	src, err := camera.NewVideoSourceFromReader(ctx, c, nil, camera.DepthStream)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(name, src), nil
}

func (c *client) setLastError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

func (c *client) run(ctx context.Context, listener *vlp16.PacketListener) {
	defer gutils.UncheckedErrorFunc(listener.Close)
	defer c.activeBackgroundWorkers.Done()

	for {
		err := ctx.Err()
		if err != nil {
			// cancelled
			return
		}

		if listener == nil {
			listener, err = vlp16.ListenUDP(ctx, c.bindAddress)
			if err != nil {
				c.setLastError(err)
				c.logger.Infof("velodyne connect error: %w", err)
				if !gutils.SelectContextOrWait(ctx, time.Second) {
					return
				}
				continue
			}
		}

		err = c.runLoop(listener)
		c.setLastError(err)
		if err != nil {
			c.logger.Infof("velodyne client error: %w", err)
			err = listener.Close()
			if err != nil {
				c.logger.Warn("trying to close connection after error got", "error", err)
			}
			listener = nil
			if !gutils.SelectContextOrWait(ctx, time.Second) {
				return
			}
		}
	}
}

func (c *client) runLoop(listener *vlp16.PacketListener) error {
	if err := listener.ReadPacket(); err != nil {
		return err
	}

	p := listener.Packet()

	c.mu.Lock()
	defer c.mu.Unlock()

	ipString := listener.SourceIP().String()
	if c.ip == "" {
		c.ip = ipString
	} else if c.ip != ipString {
		c.packets = []vlp16.Packet{}
		c.product = 0
		err := fmt.Errorf("velodyne ip changed from %s -> %s", c.ip, ipString)
		c.ip = ipString
		return err
	}

	if c.product == 0 {
		c.product = p.ProductID
	} else if c.product != p.ProductID {
		c.packets = []vlp16.Packet{}
		err := fmt.Errorf("velodyne product changed from %s -> %s", c.product, p.ProductID)
		c.product = 0
		return err
	}

	// we remove the packets too old
	firstToRemove := -1
	for idx, old := range c.packets {
		age := int(p.Timestamp) - int(old.Timestamp)
		if age < c.ttlMilliseconds*1000 {
			break
		}
		firstToRemove = idx
	}

	if firstToRemove >= 0 {
		c.packets = c.packets[firstToRemove+1:]
	}

	c.packets = append(c.packets, *p)
	return nil
}

func pointFrom(yaw, pitch, distance float64) r3.Vector {
	ea := spatialmath.NewEulerAngles()
	ea.Yaw = yaw
	ea.Pitch = pitch

	pose1 := spatialmath.NewPoseFromOrientation(ea)
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{distance, 0, 0})
	p := spatialmath.Compose(pose1, pose2).Point()

	return pointcloud.NewVector(p.X*1000, p.Y*1000, p.Z*1000)
}

func (c *client) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastError != nil {
		return nil, c.lastError
	}

	config, ok := allProductData[c.product]
	if !ok {
		return nil, fmt.Errorf("no config for %s", c.product)
	}

	pc := pointcloud.New()
	for _, p := range c.packets {
		for _, b := range p.Blocks {
			yaw := float64(b.Azimuth) / 100
			for channelID, c := range b.Channels {
				if channelID >= len(config) {
					return nil, fmt.Errorf("channel (%d)out of range %d", channelID, len(config))
				}
				pitch := config[channelID].elevationAngle
				yaw += config[channelID].azimuthOffset

				p := pointFrom(utils.DegToRad(yaw), utils.DegToRad(pitch), float64(c.Distance)/1000)
				p.Z = p.Z + config[channelID].verticalOffset

				err := pc.Set(p, pointcloud.NewBasicData().SetIntensity(uint16(c.Reflectivity)*255))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return pc, nil
}

func (c *client) Read(ctx context.Context) (image.Image, func(), error) {
	pc, err := c.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}

	meta := pc.MetaData()

	width := 800
	height := 800

	scale := func(x, y float64) (int, int) {
		return int(float64(width) * ((x - meta.MinX) / (meta.MaxX - meta.MinX))),
			int(float64(height) * ((y - meta.MinY) / (meta.MaxY - meta.MinY)))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	set := func(xpc, ypc float64, clr color.NRGBA) {
		x, y := scale(xpc, ypc)
		img.SetNRGBA(x, y, clr)
	}

	pc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		set(p.X, p.Y, color.NRGBA{255, 0, 0, 255})
		return true
	})

	centerSize := .1
	for x := -1 * centerSize; x < centerSize; x += .01 {
		for y := -1 * centerSize; y < centerSize; y += .01 {
			set(x, y, color.NRGBA{0, 255, 0, 255})
		}
	}

	return img, nil, nil
}

func (c *client) Close(ctx context.Context) error {
	c.cancelFunc()
	c.activeBackgroundWorkers.Wait()
	return nil
}
