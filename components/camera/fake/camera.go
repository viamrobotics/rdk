// Package fake implements a fake camera which always returns the same image with a user specified resolution.
package fake

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/pkg/errors"
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
	initialWidth  = 1280
	initialHeight = 720
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
	resModel, width, height := fakeModel(newConf.Width, newConf.Height)
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
	Width          int  `json:"width,omitempty"`
	Height         int  `json:"height,omitempty"`
	Animated       bool `json:"animated,omitempty"`
	RTPPassthrough bool `json:"rtp_passthrough,omitempty"`
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
		return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a height of %d", conf.Height)
	}

	if conf.Width%2 != 0 {
		return nil, errors.Errorf("odd-number resolutions cannot be rendered, cannot use a width of %d", conf.Width)
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

func fakeModel(width, height int) (*transform.PinholeCameraModel, int, int) {
	fakeModelReshaped := &transform.PinholeCameraModel{
		PinholeCameraIntrinsics: fakeIntrinsics,
		Distortion:              fakeDistortion,
	}
	switch {
	case width > 0 && height > 0:
		widthRatio := float64(width) / float64(initialWidth)
		heightRatio := float64(height) / float64(initialHeight)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * widthRatio),
			Height: int(float64(fakeIntrinsics.Height) * heightRatio),
			Fx:     fakeIntrinsics.Fx * widthRatio,
			Fy:     fakeIntrinsics.Fy * heightRatio,
			Ppx:    fakeIntrinsics.Ppx * widthRatio,
			Ppy:    fakeIntrinsics.Ppy * heightRatio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		return fakeModelReshaped, width, height
	case width > 0 && height <= 0:
		ratio := float64(width) / float64(initialWidth)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * ratio),
			Height: int(float64(fakeIntrinsics.Height) * ratio),
			Fx:     fakeIntrinsics.Fx * ratio,
			Fy:     fakeIntrinsics.Fy * ratio,
			Ppx:    fakeIntrinsics.Ppx * ratio,
			Ppy:    fakeIntrinsics.Ppy * ratio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		newHeight := int(float64(initialHeight) * ratio)
		if newHeight%2 != 0 {
			newHeight++
		}
		return fakeModelReshaped, width, newHeight
	case width <= 0 && height > 0:
		ratio := float64(height) / float64(initialHeight)
		intrinsics := &transform.PinholeCameraIntrinsics{
			Width:  int(float64(fakeIntrinsics.Width) * ratio),
			Height: int(float64(fakeIntrinsics.Height) * ratio),
			Fx:     fakeIntrinsics.Fx * ratio,
			Fy:     fakeIntrinsics.Fy * ratio,
			Ppx:    fakeIntrinsics.Ppx * ratio,
			Ppy:    fakeIntrinsics.Ppy * ratio,
		}
		fakeModelReshaped.PinholeCameraIntrinsics = intrinsics
		newWidth := int(float64(initialWidth) * ratio)
		if newWidth%2 != 0 {
			newWidth++
		}
		return fakeModelReshaped, newWidth, height
	default:
		return fakeModelReshaped, initialWidth, initialHeight
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
	forma := &format.H264{
		PayloadTyp:        96,
		PacketizationMode: 1,
		SPS: []uint8{
			0x67, 0x64, 0x0, 0x15, 0xac, 0xb2, 0x3, 0xc1, 0x1f, 0xd6,
			0x2, 0xdc, 0x8, 0x8, 0x16, 0x94, 0x0, 0x0, 0x3, 0x0, 0x4, 0x0, 0x0, 0x3, 0x0,
			0xf0, 0x3c, 0x58, 0xb9, 0x20,
		},
		PPS: []uint8{0x68, 0xeb, 0xc3, 0xcb, 0x22, 0xc0},
	}
	rtpEnc, err := forma.CreateEncoder()
	if err != nil {
		c.logger.Error(err.Error())
		return err
	}
	rtpTime := &rtptime.Encoder{ClockRate: forma.ClockRate()}
	err = rtpTime.Initialize()
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

			pkts, err := rtpEnc.Encode(aus)
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
				if err := bufAndCB.buf.Publish(func() { bufAndCB.cb(pkts) }); err != nil {
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

// nolint: lll
var (
	worldJpeg       = []byte("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")
	worldH264Base64 = "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"
)
