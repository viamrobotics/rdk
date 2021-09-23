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

	robotimpl "go.viam.com/core/robot/impl"

	"github.com/edaniels/golog"
)

const (
	boardName      = "altimeterBoard"
	gpsAddr   byte = 0x10
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
	handle, err := i2c.OpenHandle()
	if err != nil {
		return err
	}
	defer handle.Close()

	// Init the display multiple times, hoping at least one works- sometimes it takes several writes to get a good init
	for i := 0; i < 10; i++ {
		initDisp(ctx, handle, false)
	}

	initAnimation(ctx, handle)

	for {
		meters, err := GpsAlt(ctx, handle)
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

		writeAlt(ctx, feetStr, meterStr, handle)

		select {
		case <-ctx.Done():
			return nil
		default:
		}
		time.Sleep(1000 * time.Millisecond)
	}
}
