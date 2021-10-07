package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"go.viam.com/utils"

	_ "go.viam.com/core/board/detector"
	"go.viam.com/core/config"
	"go.viam.com/core/sensor/gps"

	robotimpl "go.viam.com/core/robot/impl"

	"github.com/edaniels/golog"
)

const (
	boardName      = "altimeterBoard"
	gpsName        = "gps1"
	dispAddr  byte = 0x3C
)

var logger = golog.NewDevelopmentLogger("gps")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	gpsBoard, ok := myRobot.BoardByName(boardName)
	if !ok {
		return fmt.Errorf("failed to find board %s", boardName)
	}
	i2c, _ := gpsBoard.I2CByName("bus1")

	s, ok := myRobot.SensorByName(gpsName)
	if !ok {
		return fmt.Errorf("no gps named %q", gpsName)
	}

	gpsDevice, ok := s.(gps.GPS)
	if !ok {
		return fmt.Errorf("%q is not a GPS device", gpsName)
	}

	handle, err := i2c.OpenHandle()
	if err != nil {
		return err
	}
	// Init the display multiple times, hoping at least one works- sometimes it takes several writes to get a good init
	for i := 0; i < 10; i++ {
		initDisp(ctx, handle, false)
	}

	initAnimation(ctx, handle)
	handle.Close()

	for {
		meters := -9999.
		valid, _ := gpsDevice.Valid(ctx)
		if valid {
			meters, _ = gpsDevice.Altitude(ctx)
		}
		feet := int(meters * 3.28084)
		meterStr := strconv.Itoa(int(meters))
		feetStr := strconv.Itoa(feet)
		if err != nil {
			feetStr = "gps"
			meterStr = "error"
		} else if meters == -9999 {
			feetStr = "no"
			meterStr = "lock"
		}

		handle, err := i2c.OpenHandle()
		if err != nil {
			return err
		}
		writeAlt(ctx, feetStr, meterStr, handle)
		handle.Close()

		select {
		case <-ctx.Done():
			return nil
		default:
		}
		time.Sleep(1000 * time.Millisecond)
	}
}
