// Package gopro implements a gopro based camra. Support is experimental.
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

	"github.com/edaniels/golog"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const usbHost = "172.27.116.51"

func init() {
	registry.RegisterComponent(camera.Subtype, "gopro", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			gCam := newCamera(logger)
			gCam.start()
			return &camera.ImageSource{ImageSource: gCam}, nil
		},
	})
}

func newCamera(logger golog.Logger) *gpCamera {
	ctx, cancel := context.WithCancel(context.Background())
	return &gpCamera{
		cancelCtx:  ctx,
		cancelFunc: cancel,
		logger:     logger,
		buf: &dumbWriter{
			frameCh: make(chan []byte, 1),
		},
	}
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
			req, err := http.NewRequestWithContext(
				c.cancelCtx,
				http.MethodGet,
				fmt.Sprintf("http://%s/gp/gpControl/execute?p1=gpStream&c1=start", usbHost),
				nil,
			)
			if err != nil {
				c.logger.Errorw("error making GET", "error", err)
				continue
			}
			resp, err := http.DefaultClient.Do(req)
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
	if dw.shouldErr {
		dw.mu.Unlock()
		return 0, errors.New("expected err")
	}
	dw.mu.Unlock()
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

func (c *gpCamera) Close() {
	c.buf.mu.Lock()
	c.buf.shouldErr = true
	c.buf.mu.Unlock()
	c.cancelFunc()
	c.activeBackgroundworkers.Wait()
}
