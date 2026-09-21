//go:build windows

// Command driverprobe enumerates the DirectShow capture devices that mediadevices
// registers on Windows and reports, per device, the formats it advertises and
// whether it actually delivers a frame.
//
// It exists because a device that fails to stream is indistinguishable from a
// healthy one at the Go layer: openCamera ignores mediaControl->Run()'s HRESULT
// and BufferCB drops size-mismatched frames to stderr, so video.Reader.Read()
// simply blocks forever. Run this to find out which devices stream and which hang.
//
//	go run ./components/camera/videosource/cmd/driverprobe
//	go run ./components/camera/videosource/cmd/driverprobe -concurrent
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pion/mediadevices/pkg/driver"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

func main() {
	wait := flag.Duration("wait", 5*time.Second, "how long to wait for a device's first frame")
	concurrent := flag.Bool("concurrent", false, "hold every device open at once instead of probing one at a time")
	flag.Parse()

	mediadevicescamera.Initialize()

	drivers := driver.GetManager().Query(driver.FilterVideoRecorder())
	fmt.Printf("registered video drivers: %d\n\n", len(drivers))
	if len(drivers) == 0 {
		fmt.Println("no capture devices registered; nothing to probe")
		return
	}

	if *concurrent {
		probeConcurrent(drivers, *wait)
		return
	}
	for i, d := range drivers {
		probe(i, d, *wait)
	}
}

// probe opens a single device, prints what it advertises, and reports whether a
// frame arrives. The device is closed before returning so the next probe sees it free.
func probe(i int, d driver.Driver, wait time.Duration) {
	props, ok := describe(i, d)
	if !ok {
		return
	}
	defer closeDriver(d)

	if len(props) == 0 {
		fmt.Printf("    NO USABLE FORMATS: the Windows driver only reports YUY2 and NV12, so\n")
		fmt.Printf("    webcam selection cannot choose this device\n\n")
		return
	}

	selected := props[0]
	fmt.Printf("    probing with %dx%d %s\n", selected.Width, selected.Height, selected.FrameFormat)
	report(firstFrame(d, selected, wait))
	fmt.Println()
}

// probeConcurrent starts every device before reading from any of them, which is
// what viam-server does when several webcam components are configured at once.
func probeConcurrent(drivers []driver.Driver, wait time.Duration) {
	type started struct {
		index  int
		driver driver.Driver
		reader video.Reader
	}

	var running []started
	for i, d := range drivers {
		props, ok := describe(i, d)
		if !ok {
			continue
		}
		if len(props) == 0 {
			fmt.Printf("    NO USABLE FORMATS; skipping\n\n")
			closeDriver(d)
			continue
		}

		selected := props[0]
		selected.DiscardFramesOlderThan = time.Second
		recorder, ok := d.(driver.VideoRecorder)
		if !ok {
			fmt.Printf("    not a VideoRecorder; skipping\n\n")
			closeDriver(d)
			continue
		}
		reader, err := recorder.VideoRecord(selected)
		if err != nil {
			fmt.Printf("    VideoRecord failed: %v\n\n", err)
			closeDriver(d)
			continue
		}
		fmt.Printf("    started at %dx%d %s\n\n", selected.Width, selected.Height, selected.FrameFormat)
		running = append(running, started{index: i, driver: d, reader: reader})
	}

	fmt.Printf("all %d device(s) started; now reading from each\n\n", len(running))
	for _, r := range running {
		fmt.Printf("[%d] %s\n", r.index, r.driver.Info().Name)
		report(awaitFrame(r.reader, wait))
		closeDriver(r.driver)
	}
}

// describe opens the driver and prints its identity and advertised formats.
func describe(i int, d driver.Driver) ([]prop.Media, bool) {
	info := d.Info()
	fmt.Printf("[%d] name  = %q\n", i, info.Name)
	fmt.Printf("    label = %q\n", info.Label)
	fmt.Printf("    state = %s\n", d.Status())

	if err := d.Open(); err != nil {
		fmt.Printf("    open failed: %v\n\n", err)
		return nil, false
	}

	props := d.Properties()
	fmt.Printf("    advertised formats: %d\n", len(props))
	for _, p := range props {
		fmt.Printf("      %dx%d %s\n", p.Width, p.Height, p.FrameFormat)
	}
	return props, true
}

// firstFrame starts the device and waits for one frame.
func firstFrame(d driver.Driver, selected prop.Media, wait time.Duration) error {
	recorder, ok := d.(driver.VideoRecorder)
	if !ok {
		return fmt.Errorf("driver is not a VideoRecorder")
	}
	selected.DiscardFramesOlderThan = time.Second
	reader, err := recorder.VideoRecord(selected)
	if err != nil {
		return fmt.Errorf("VideoRecord failed: %w", err)
	}
	return awaitFrame(reader, wait)
}

// awaitFrame reads in the background so a device that never delivers cannot wedge
// the probe. Read blocks indefinitely on Windows when the graph is not producing.
func awaitFrame(reader video.Reader, wait time.Duration) error {
	type result struct {
		width, height int
		err           error
	}
	done := make(chan result, 1)
	go func() {
		img, release, err := reader.Read()
		if release != nil {
			release()
		}
		if err != nil {
			done <- result{err: err}
			return
		}
		b := img.Bounds()
		done <- result{width: b.Dx(), height: b.Dy()}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			return r.err
		}
		fmt.Printf("    FRAME OK: %dx%d\n", r.width, r.height)
		return nil
	case <-time.After(wait):
		return fmt.Errorf("no frame within %s (graph built but nothing is streaming)", wait)
	}
}

func report(err error) {
	if err != nil {
		fmt.Printf("    NO FRAMES: %v\n", err)
	}
}

func closeDriver(d driver.Driver) {
	if err := d.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing %q: %v\n", d.Info().Name, err)
	}
}
