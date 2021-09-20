package gopro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

const usbHost = "172.27.116.51"

func init() {
	registry.RegisterCamera("gopro", registry.Camera{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
			gCam, err := newCamera(logger)
			if err != nil {
				return nil, err
			}
			gCam.start()
			return &camera.ImageSource{gCam}, nil
		}})
}

func newCamera(logger golog.Logger) (*gpCamera, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &gpCamera{
		cancelCtx:  ctx,
		cancelFunc: cancel,
		logger:     logger,
		buf: &dumbWriter{
			frameCh: make(chan []byte, 1),
		},
	}, nil
}

type gpCamera struct {
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundworkers sync.WaitGroup
	buf                     *dumbWriter
}

func (c *gpCamera) start() {
	c.activeBackgroundworkers.Add(2)
	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundworkers.Done()
		defer http.DefaultClient.CloseIdleConnections()
		for {
			resp, err := http.DefaultClient.Get(fmt.Sprintf("http://%s/gp/gpControl/execute?p1=gpStream&c1=start", usbHost))
			if err != nil {
				c.logger.Errorw("error doing GET", "error", err)
				continue
			}
			utils.UncheckedError(resp.Body.Close())
			if resp.StatusCode != http.StatusOK {
				c.logger.Errorw("unexpected status", "status_code", resp.StatusCode)
				continue
			}
			if !utils.SelectContextOrWait(c.cancelCtx, 5*time.Second) {
				return
			}
		}
	})
	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundworkers.Done()
		// TODO(erd): how to cancel
		if err := ffmpeg.Input(fmt.Sprintf("udp://%s:8554", usbHost)).
			Output("pipe:", ffmpeg.KwArgs{"format": "image2pipe", "vcodec": "mjpeg"}).
			WithOutput(c.buf, os.Stdout).
			Run(); err != nil {
			c.logger.Errorw("error running", "error", err)
		}
	})
}

type dumbWriter struct {
	mu        sync.Mutex
	shouldErr bool
	frameCh   chan []byte
	lastFrame time.Time
}

func (dw *dumbWriter) Write(data []byte) (int, error) {
	now := time.Now()
	if now.Sub(dw.lastFrame) < time.Second/10 {
		return len(data), nil
	}
	dw.lastFrame = now
	dw.mu.Lock()
	shouldErr := dw.shouldErr
	dw.mu.Unlock()
	if shouldErr {
		return 0, errors.New("expected err")
	}
	select {
	case dw.frameCh <- data:
	default:
	}
	return len(data), nil
}

func (c *gpCamera) Next(ctx context.Context) (image.Image, func(), error) {
	for {
		select {
		case <-ctx.Done():
		case frame := <-c.buf.frameCh:
			img, _, err := image.Decode(bytes.NewReader(frame))
			if err != nil {
				if strings.Contains(err.Error(), "Huffman") || strings.Contains(err.Error(), "invalid") {
					continue
				}
				return nil, nil, err
			}
			return img, func() {}, nil
		}
	}
}

func (c *gpCamera) Close() error {
	c.buf.mu.Lock()
	c.buf.shouldErr = true
	c.buf.mu.Unlock()
	c.cancelFunc()
	c.activeBackgroundworkers.Wait()
	return nil
}
