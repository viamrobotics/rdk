package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"time"

	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

var logger = golog.Global

func main() {
	utils.ContextualMainQuit(mainWithArgs)
}

func mainWithArgs(ctx context.Context, args []string) error {
	parsed, err := parseFlags(args)
	if err != nil {
		return err
	}

	return readCompass(ctx, parsed.DeviceAddress)
}

// Arguments for the command (parsed).
type Arguments struct {
	DeviceAddress string
}

const (
	deviceFlagName = "device"
)

func parseFlags(args []string) (Arguments, error) {
	cmdLine := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var buf bytes.Buffer
	cmdLine.SetOutput(&buf)

	var address string
	cmdLine.StringVar(&address, deviceFlagName, "", "device address")
	if err := cmdLine.Parse(args[1:]); err != nil {
		return Arguments{}, err
	}

	if len(address) == 0 {
		cmdLine.Usage()
		return Arguments{}, errors.New(buf.String())
	}

	return Arguments{DeviceAddress: address}, nil
}

func readCompass(ctx context.Context, deviceAddress string) error {
	sensor, err := compass.NewWSDevice(ctx, deviceAddress)
	if err != nil {
		return err
	}

	tickRate := 100 * time.Millisecond
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	count := 0
	start := time.Now()
	defer func() {
		logger.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		heading, err := sensor.Heading(context.Background())
		if err != nil {
			logger.Errorw("failed to get sensor heading", "error", err)
			continue
		}
		logger.Infow("heading", "data", heading)
	}
}
