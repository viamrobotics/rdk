// Package iphonelidar provides a command for viewing the output of iPhone's LiDAR camera
package iphonelidar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/pointcloud"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
)

// A Measurement is a struct representing the data collected by the iPhone using
// the point clouds iPhone app.
type Measurement struct {
	// e.g of PointCloud:
	// [(0.1, 0.2, 0.3), (0.4, 0.5, 0.6), ... , (0.7, 0.8, 0.9)]
	PointCloud []float64 `json:"poclo"`
	// hasColor   bool
	// hasValue   bool
	// minX, maxX float64
	// minY, maxY float64
	// minZ, maxZ float64
	// rbg        []float64 `json:"rbg"`
}

// IPhone is an iPhone based LiDAR camera.
type IPhone struct {
	Config      *Config       // The config struct containing the info necessary to determine what iPhone to connect to.
	readCloser  io.ReadCloser // The underlying response stream from the iPhone.
	reader      *bufio.Reader // Read connection to iPhone to pull lidar data from.
	log         golog.Logger
	mut         sync.Mutex   // Mutex to ensure only one goroutine or thread is reading from reader at a time.
	measurement atomic.Value // The latest measurement value read from reader.

	cancelCtx               context.Context
	cancelFn                func()
	activeBackgroundWorkers sync.WaitGroup
}

// Config is a struct used to configure and construct an IPhone using IPhone.New().
type Config struct {
	Host      string // The host name of the iPhone being connected to.
	Port      int    // The port to connect to.
	isAligned bool   // are color and depth image already aligned
}

const (
	DefaultPath      = "/hello" //"/measurementStream"
	defaultTimeoutMs = 1000
	model            = "iphone"
)

// init registers the iphone lidar camera.
func init() {
	registry.RegisterCamera("iPhoneLiDAR", registry.Camera{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (camera.Camera, error) {
			// add conditionals to  make sure that json file was properly formatted?

			iCam, err := New(ctx, Config{Host: c.Host, Port: c.Port}, logger)
			if err != nil {
				return nil, err
			}
			// the velodyne implementation does not use the line below
			// why is the line below used in some implementations
			return &camera.ImageSource{iCam}, nil
			// Note can also use:
			// RegisterComponentAttributeMapConverter
			// 'to convert the whole thing and set defaults in the validate function'
			// what is the validate function?
		}})
}

// New returns a new IPhone that that pulls data from the iPhone defined in config.
// New creates a connection to a iPhone lidar and generates pointclouds from it.
func New(ctx context.Context, config Config, logger golog.Logger) (*IPhone, error) {
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	ip := IPhone{
		Config:                  &config,
		log:                     logger,
		mut:                     sync.Mutex{},
		cancelCtx:               cancelCtx,
		cancelFn:                cancelFn,
		activeBackgroundWorkers: sync.WaitGroup{},
	}
	r, rc, err := ip.Config.getNewReader()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to iphone %s on port %d: %v", config.Host, config.Port, err)
	}
	ip.readCloser = rc
	ip.reader = r
	ip.measurement.Store(Measurement{})

	// Have a thread in the background constantly reading the latest camera readings from the iPhone and saving
	// them to ip.measurement. This avoids the problem of our read accesses constantly being behind by bufSize bytes.
	ip.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer ip.activeBackgroundWorkers.Done()
		for {
			select {
			case <-ip.cancelCtx.Done():
				return
			default:
			}
			CamReading, err := ip.readNextMeasurement(ip.cancelCtx)
			if err != nil {
				logger.Debugw("error reading iphone data", "error", err)
			} else {
				ip.measurement.Store(*CamReading)
			}
		}
	})

	return &ip, nil
}

// StartCalibration does nothing.
func (ip *IPhone) StartCalibration(ctx context.Context) error {
	return nil
}

// StopCalibration does nothing.
func (ip *IPhone) StopCalibration(ctx context.Context) error {
	return nil
}

func (c *Config) getNewReader() (*bufio.Reader, io.ReadCloser, error) {
	portString := strconv.Itoa(c.Port)
	url := path.Join(c.Host+":"+portString, DefaultPath)
	resp, err := http.Get("http://" + url)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("received non-200 status code when connecting: %d", resp.StatusCode)
	}
	return bufio.NewReader(resp.Body), resp.Body, nil
}

// readNextMeasurement attempts to read the next line available to ip.reader. It has a defaultTimeoutMs
// timeout, and returns an error if no measurement was made available on ip.reader in that time or if the line did not
// contain a valid JSON representation of a Measurement.
func (ip *IPhone) readNextMeasurement(ctx context.Context) (*Measurement, error) {
	timeout := time.Now().Add(defaultTimeoutMs * time.Millisecond)
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithDeadline(ctx, timeout)
	defer wg.Wait()
	defer cancel()

	ch := make(chan string, 1)
	wg.Add(1)
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		ip.mut.Lock()
		defer ip.mut.Unlock()
		measurement, err := ip.reader.ReadString('\n')
		if err != nil {
			if err := ip.readCloser.Close(); err != nil {
				ip.log.Errorw("failed to close reader", "error", err)
			}
			// In the error case, it's possible we were disconnected from the underlying iPhone. Attempt to reconnect.
			r, rc, err := ip.Config.getNewReader()
			if err != nil {
				ip.log.Errorw("failed to connect to iphone", "error", err)
			} else {
				ip.readCloser = rc
				ip.reader = r
			}
		} else {
			ch <- measurement
		}
	})

	select {
	case measurement := <-ch:
		var camReading Measurement
		err := json.Unmarshal([]byte(measurement), &camReading)
		if err != nil {
			return nil, err
		}

		return &camReading, nil
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for iphone measurement")
	}
}

// basicPointCloud is the basic implementation of the PointCloud interface backed by
// a map of points keyed by position.
// type basicPointCloud struct {
// 	points     map[key]pointcloud.Point
// 	hasColor   bool
// 	hasValue   bool
// 	minX, maxX float64
// 	minY, maxY float64
// 	minZ, maxZ float64
// }

// Vec3 is a three-dimensional vector.
//type Vec3 r3.Vector

// Vec3s is a series of three-dimensional vectors.
// type Vec3s []Vec3
// type basicPoint struct {
// 	position  Vec3
// 	hasColor  bool
// 	c         color.NRGBA
// 	hasValue  bool
// 	value     int
// 	intensity uint16
//ARFrame.lightEstimate returns <ARLightEstimate: 0x283d30f80 ambientIntensity=945.43 ambientColorTemperature=5927.38>
//}

func (ip *IPhone) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	// camReading := ip.measurement.Load().(Measurement)
	// return camReading.PointCloud, nil

	pc := pointcloud.New()
	return pc, pc.Set(pointcloud.NewColoredPoint(16, 16, 16, color.NRGBA{255, 0, 0, 255}))
}

func (ip *IPhone) Next(ctx context.Context) (image.Image, func(), error) {
	// pc, err := ip.NextPointCloud(ctx)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// minX := 0.0
	// minY := 0.0

	// maxX := 0.0
	// maxY := 0.0

	// pc.Iterate(func(p pointcloud.Point) bool {
	// 	pos := p.Position()
	// 	minX = math.Min(minX, pos.X)
	// 	maxX = math.Max(maxX, pos.X)
	// 	minY = math.Min(minY, pos.Y)
	// 	maxY = math.Max(maxY, pos.Y)
	// 	return true
	// })

	// width := 800
	// height := 800

	// scale := func(x, y float64) (int, int) {
	// 	return int(float64(width) * ((x - minX) / (maxX - minX))),
	// 		int(float64(height) * ((y - minY) / (maxY - minY)))
	// }

	// img := image.NewNRGBA(image.Rect(0, 0, width, height))

	// set := func(xpc, ypc float64, clr color.NRGBA) {
	// 	x, y := scale(xpc, ypc)
	// 	img.SetNRGBA(x, y, clr)
	// }

	// pc.Iterate(func(p pointcloud.Point) bool {
	// 	set(p.Position().X, p.Position().Y, color.NRGBA{255, 0, 0, 255})
	// 	return true
	// })

	// centerSize := .1
	// for x := -1 * centerSize; x < centerSize; x += .01 {
	// 	for y := -1 * centerSize; y < centerSize; y += .01 {
	// 		set(x, y, color.NRGBA{0, 255, 0, 255})
	// 	}
	// }

	// return img, nil, nil

	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	img.Set(16, 16, rimage.Red)
	return img, func() {}, nil
}

func (ip *IPhone) Close() error {
	ip.cancelFn()
	ip.activeBackgroundWorkers.Wait()
	return ip.readCloser.Close()
}
