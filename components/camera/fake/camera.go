// Package fake implements a fake camera which always returns the same image with a user specified resolution.
package fake

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

var (
	// Model is the model of the fake buildin camera.
	Model = resource.DefaultModelFamily.WithModel("fake")
	// ErrRTPPassthroughNotEnabled indicates that rtp_passthrough is not enabled.
	ErrRTPPassthroughNotEnabled = errors.New("rtp_passthrough not enabled")
)

const (
	initialWidth      = 1280
	initialHeight     = 720
	defaultIntrinsics = true
)

func init() {
	resource.RegisterComponent(
		camera.API,
		Model,
		resource.Registration[camera.Camera, *Config]{Constructor: NewCamera},
	)
}

// NewCamera returns a new fake camera.
func NewCamera(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	_, paramErr := newConf.Validate("")
	if paramErr != nil {
		return nil, paramErr
	}
	width := initialWidth
	if newConf.Width > 0 {
		width = newConf.Width
	}
	height := initialHeight
	if height > 0 {
		height = newConf.Height
	}
	intrinsics := defaultIntrinsics
	if newConf.Intrinsics != nil {
		intrinsics = *newConf.Intrinsics
	}
	var resModel *transform.PinholeCameraModel
	if intrinsics {
		resModel = fakeModel(width, height)
	} else {
		resModel = nil
	}
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	cam := &Camera{
		ctx:            cancelCtx,
		cancelFn:       cancelFn,
		Named:          conf.ResourceName().AsNamed(),
		Model:          resModel,
		Width:          width,
		Height:         height,
		Animated:       newConf.Animated,
		RTPPassthrough: newConf.RTPPassthrough,
		bufAndCBByID:   make(map[rtppassthrough.SubscriptionID]bufAndCB),
		logger:         logger,
	}
	src, err := camera.NewVideoSourceFromReader(ctx, cam, resModel, camera.ColorStream)
	if err != nil {
		return nil, err
	}

	if cam.RTPPassthrough {
		msg := "rtp_passthrough is enabled. GetImage will ignore width, height, and animated config params"
		logger.CWarn(ctx, msg)
		d := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(worldJpeg))
		img, err := jpeg.Decode(d)
		if err != nil {
			return nil, err
		}

		cam.cacheImage = img
		if err := cam.startPassthrough(); err != nil {
			return nil, err
		}
	}
	return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
}

// Config are the attributes of the fake camera config.
type Config struct {
	Width          int   `json:"width,omitempty"`
	Height         int   `json:"height,omitempty"`
	Animated       bool  `json:"animated,omitempty"`
	RTPPassthrough bool  `json:"rtp_passthrough,omitempty"`
	Intrinsics     *bool `json:"intrinsics,omitempty"`
}

// Validate checks that the config attributes are valid for a fake camera.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Height > 10000 || conf.Width > 10000 {
		return nil, errors.New("maximum supported pixel height or width for fake cameras is 10000 pixels")
	}

	if conf.Height < 0 || conf.Width < 0 {
		return nil, errors.New("cannot use negative pixel height and width for fake cameras")
	}

	if conf.Height%2 != 0 {
		return nil, fmt.Errorf("odd-number resolutions cannot be rendered, cannot use a height of %d", conf.Height)
	}

	if conf.Width%2 != 0 {
		return nil, fmt.Errorf("odd-number resolutions cannot be rendered, cannot use a width of %d", conf.Width)
	}

	return nil, nil
}

var fakeIntrinsics = &transform.PinholeCameraIntrinsics{
	Width:  1024,
	Height: 768,
	Fx:     821.32642889,
	Fy:     821.68607359,
	Ppx:    494.95941428,
	Ppy:    370.70529534,
}

var fakeDistortion = &transform.BrownConrady{
	RadialK1:     0.11297234,
	RadialK2:     -0.21375332,
	RadialK3:     -0.01584774,
	TangentialP1: -0.00302002,
	TangentialP2: 0.19969297,
}

func fakeModel(width, height int) *transform.PinholeCameraModel {
	intrinsics := *fakeIntrinsics
	if width > 0 {
		intrinsics.Width = width
	}
	if height > 0 {
		intrinsics.Height = height
	}
	return &transform.PinholeCameraModel{
		PinholeCameraIntrinsics: &intrinsics,
		Distortion:              fakeDistortion,
	}
}

// Camera is a fake camera that always returns the same image.
type Camera struct {
	resource.Named
	resource.AlwaysRebuild
	mu                      sync.RWMutex
	Model                   *transform.PinholeCameraModel
	Width                   int
	Height                  int
	Animated                bool
	RTPPassthrough          bool
	ctx                     context.Context
	cancelFn                context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	bufAndCBByID            map[rtppassthrough.SubscriptionID]bufAndCB
	cacheImage              image.Image
	cachePointCloud         pointcloud.PointCloud
	logger                  logging.Logger
}

// Read always returns the same image of a yellow to blue gradient.
func (c *Camera) Read(ctx context.Context) (image.Image, func(), error) {
	if c.cacheImage != nil {
		return c.cacheImage, func() {}, nil
	}
	width := float64(c.Width)
	height := float64(c.Height)
	img := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))

	totalDist := math.Sqrt(math.Pow(0-width, 2) + math.Pow(0-height, 2))

	tick := time.Now().UnixMilli() / 20
	var x, y float64
	for x = 0; x < width; x++ {
		for y = 0; y < height; y++ {
			dist := math.Sqrt(math.Pow(0-x, 2) + math.Pow(0-y, 2))
			dist /= totalDist
			thisColor := color.RGBA{uint8(255 - (255 * dist)), uint8(255 - (255 * dist)), uint8(0 + (255 * dist)), 255}

			var px, py int
			if c.Animated {
				px = int(int64(x)+tick) % int(width)
				py = int(y)
			} else {
				px, py = int(x), int(y)
			}
			img.Set(px, py, thisColor)
		}
	}
	if !c.Animated {
		c.cacheImage = img
	}
	return rimage.ConvertImage(img), func() {}, nil
}

// NextPointCloud always returns a pointcloud of a yellow to blue gradient, with the depth determined by the intensity of blue.
func (c *Camera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c.cachePointCloud != nil {
		return c.cachePointCloud, nil
	}
	dm := pointcloud.New()
	width := float64(c.Width)
	height := float64(c.Height)

	totalDist := math.Sqrt(math.Pow(0-width, 2) + math.Pow(0-height, 2))

	var x, y float64
	for x = 0; x < width; x++ {
		for y = 0; y < height; y++ {
			dist := math.Sqrt(math.Pow(0-x, 2) + math.Pow(0-y, 2))
			dist /= totalDist
			thisColor := color.NRGBA{uint8(255 - (255 * dist)), uint8(255 - (255 * dist)), uint8(0 + (255 * dist)), 255}
			err := dm.Set(r3.Vector{X: x, Y: y, Z: 255 * dist}, pointcloud.NewColoredData(thisColor))
			if err != nil {
				return nil, err
			}
		}
	}
	c.cachePointCloud = dm
	return dm, nil
}

type bufAndCB struct {
	cb  rtppassthrough.PacketCallback
	buf *rtppassthrough.Buffer
}

// SubscribeRTP begins a subscription to receive RTP packets.
func (c *Camera) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if !c.RTPPassthrough {
		return rtppassthrough.NilSubscription, ErrRTPPassthroughNotEnabled
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	sub, buf, err := rtppassthrough.NewSubscription(bufferSize)
	if err != nil {
		return rtppassthrough.NilSubscription, err
	}
	webrtcPayloadMaxSize := 1188 // 1200 - 12 (RTP header)
	encoder := &rtph264.Encoder{
		PayloadType:    96,
		PayloadMaxSize: webrtcPayloadMaxSize,
	}

	if err := encoder.Init(); err != nil {
		buf.Close()
		return rtppassthrough.NilSubscription, err
	}

	c.bufAndCBByID[sub.ID] = bufAndCB{
		cb:  packetsCB,
		buf: buf,
	}
	buf.Start()
	return sub, nil
}

// Unsubscribe terminates the subscription.
func (c *Camera) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if !c.RTPPassthrough {
		return ErrRTPPassthroughNotEnabled
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	bufAndCB, ok := c.bufAndCBByID[id]
	if !ok {
		return errors.New("id not found")
	}
	delete(c.bufAndCBByID, id)
	bufAndCB.buf.Close()
	return nil
}

func (c *Camera) startPassthrough() error {
	forma := &format.H264{}
	webrtcPayloadMaxSize := 1188 // 1200 - 12 (RTP header)
	encoder := &rtph264.Encoder{
		PayloadType:    96,
		PayloadMaxSize: webrtcPayloadMaxSize,
	}

	if err := encoder.Init(); err != nil {
		return err
	}

	rtpTime := &rtptime.Encoder{ClockRate: forma.ClockRate()}
	err := rtpTime.Initialize()
	if err != nil {
		c.logger.Error(err.Error())
		return err
	}
	start := time.Now()
	b, err := base64.StdEncoding.DecodeString(worldH264Base64)
	if err != nil {
		c.logger.Error(err.Error())
		return err
	}
	aus, err := h264.AnnexBUnmarshal(b)
	if err != nil {
		c.logger.Error(err.Error())
		return err
	}
	f := func() {
		defer c.unsubscribeAll()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if c.ctx.Err() != nil {
				return
			}

			pkts, err := encoder.Encode(aus)
			if err != nil {
				c.logger.Error(err)
				return
			}

			ts := rtpTime.Encode(time.Since(start))
			for _, pkt := range pkts {
				pkt.Timestamp = ts
			}

			// get current timestamp
			c.mu.RLock()
			for _, bufAndCB := range c.bufAndCBByID {
				if err := bufAndCB.buf.Publish(func() {
					bufAndCB.cb(pkts)
				}); err != nil {
					c.logger.Warn("Publish err: %s", err.Error())
				}
			}
			c.mu.RUnlock()
		}
	}
	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(f, c.activeBackgroundWorkers.Done)
	return nil
}

func (c *Camera) unsubscribeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, bufAndCB := range c.bufAndCBByID {
		delete(c.bufAndCBByID, id)
		bufAndCB.buf.Close()
	}
}

// Close does nothing.
func (c *Camera) Close(ctx context.Context) error {
	c.cancelFn()
	c.activeBackgroundWorkers.Wait()
	return nil
}
