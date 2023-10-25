//go:build linux

/*
Package vcamera creates and streams video to virtual V4L2 capture devices on Linux.

It uses [V4L2Loopback] to create virtual camera drivers and [GStreamer] to display test video streams. You can install
both using our V4L2Loopback setup [script]. This script also needs to run `sudo modprobe v4l2loopback`. Since
feeding the root password to that command at runtime would be impractical, it's recommended that you allow the current user
to run that command without requiring a password. The script above will display a prompt asking if you want this behavior.
Select "yes".

Usage:

	// create a builder object
	config := vcamera.Builder()

	// create 1-to-N cameras
	config = config.NewCamera(1, "Low-res Camera", vcamera.Resolution{Width: 640, Height: 480})
	config = config.NewCamera(2, "Hi-res Camera", vcamera.Resolution{Width: 1280, Height: 720})
	...

	// start streaming
	config, err := Stream()
	if err != nil {
		// handle error
	}

	// shutdown streams
	config.Shutdown()

Because this class allows for method chaining the above could be accomplished like so

	config, err := vcamera.Builder().
		NewCamera(1, "Low-res Camera", vcamera.Resolution{Width: 640, Height: 480}).
		NewCamera(2, "Hi-res Camera", vcamera.Resolution{Width: 1280, Height: 720}).
		Stream()

	// DO NOT forget to stop streaming to avoid leaking resources.
	defer config.Shutdown()

[V4L2Loopback]: https://github.com/umlaeute/v4l2loopback/tree/8cb5270d20484bde26eabb085011a8ea27285446
[GStreamer]: https://gstreamer.freedesktop.org/
[script]: https://github.com/viamrobotics/rdk/blob/0f49904ec550e5a33fc649e3153b4d465256a02a/etc/v4l2loopback_setup.sh
*/
package vcamera

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

// Resolution stores the Width and Height in pixels for a camera resolution.
type Resolution struct {
	Width  int
	Height int
}

type device struct {
	key        string
	label      string
	resolution Resolution
}

// Config is a builder object used to create virtual cameras.
type Config struct {
	deviceMap               map[int]bool
	devices                 []device
	err                     error
	cancelCtx               context.Context
	cancelFn                func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  logging.Logger
}

// Builder creates a new vcamera.Config builder object.
func Builder(logger logging.Logger) *Config {
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	return &Config{
		deviceMap: make(map[int]bool),
		cancelFn:  cancelFn,
		cancelCtx: cancelCtx,
		logger:    logger,
	}
}

// NewCamera lazily creates a new camera. No cameras are actually created until Stream is called.
func (c *Config) NewCamera(id int, label string, res Resolution) *Config {
	if c.err != nil {
		return c
	}

	if _, ok := c.deviceMap[id]; ok {
		c.err = fmt.Errorf("duplicate camera id %d", id)
		return c
	}

	c.deviceMap[id] = true
	device := device{
		key:        strconv.Itoa(id),
		label:      fmt.Sprintf("\"%s\"", label), // add quotes
		resolution: res,
	}
	c.devices = append(c.devices, device)
	return c
}

func createCameras(c *Config) error {
	var devKeys []string
	var devLabels []string
	for _, d := range c.devices {
		devKeys = append(devKeys, d.key)
		devLabels = append(devLabels, d.label)
	}

	devices := fmt.Sprintf("%s=%s", "video_nr", strings.Join(devKeys, ","))
	labels := fmt.Sprintf("%s=%s", "card_label", strings.Join(devLabels, ","))

	cmd := fmt.Sprintf("sudo modprobe v4l2loopback %s %s", devices, labels)
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput() //nolint:gosec
	if err != nil {
		return errors.New(string(out))
	}
	return nil
}

func startStream(config *Config, dev device) (<-chan struct{}, error) {
	//nolint:gosec
	cmd := exec.CommandContext(config.cancelCtx, "bash", "-c", fmt.Sprintf(
		"gst-launch-1.0 -v videotestsrc "+
			"! video/x-raw,format=YUY2,width=320,height=240 "+
			"! videoscale "+
			"! video/x-raw,format=YUY2,width=%d,height=%d "+
			"! v4l2sink device=/dev/video%s",
		dev.resolution.Width, dev.resolution.Height, dev.key))

	// capture output from stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// wait for cmd to finish to avoid leak
	config.activeBackgroundWorkers.Add(1)
	go func() {
		defer config.activeBackgroundWorkers.Done()
		if err := cmd.Wait(); err != nil {
			config.logger.Warn(err)
		}
	}()

	// broadcast channel, closed when stream is in state "PLAYING"
	readyCh := make(chan struct{})
	scanner := bufio.NewScanner(stdout)

	config.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer config.activeBackgroundWorkers.Done()
		for {
			select {
			case <-config.cancelCtx.Done():
				return
			default:
				scanner.Scan()
				// looks for state "PLAYING" in stdout
				if line := scanner.Text(); strings.Contains(line, "PLAYING") {
					close(readyCh)
					return
				}
			}
		}
	})
	return readyCh, nil
}

// Stream starts streaming videos to all virtual cameras.
//
// This function blocks until all virtual cameras successfully start streaming, or an error occurs.
// Shutdown will stop all streams started by this method.
func (c *Config) Stream() (*Config, error) {
	if c.err != nil {
		return c, c.err
	}

	if c.err = createCameras(c); c.err != nil {
		return c, c.err
	}

	var streamChs []<-chan struct{}
	for _, d := range c.devices {
		var ch <-chan struct{}
		ch, c.err = startStream(c, d)
		streamChs = append(streamChs, ch)
		if c.err != nil {
			return c, c.err
		}
	}

	// wait for streams to become ready or timeout
	timeout := time.After(time.Second)
	for _, s := range streamChs {
		select {
		case <-s:
		case <-timeout:
			c.err = errors.New("timeout waiting for stream")
			return c, c.err
		}
	}

	return c, c.err
}

// Shutdown stops streaming to and removes all virtual cameras.
func (c *Config) Shutdown() error {
	c.cancelFn()
	c.activeBackgroundWorkers.Wait()
	c.err = errors.New("stopped streaming")

	// removes all virtual cameras
	if out, err := exec.Command("bash", "-c", "sudo modprobe v4l2loopback -r").CombinedOutput(); err != nil {
		return errors.New(string(out))
	}
	return nil
}
