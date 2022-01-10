package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
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

	cfg, err := config.Read(ctx, flag.Arg(0), logger)
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	gpsBoard, ok := myRobot.BoardByName(boardName)
	if !ok {
		return fmt.Errorf("failed to find board %s", boardName)
	}
	localB, ok := gpsBoard.(board.LocalBoard)
	if !ok {
		return fmt.Errorf("board %s is not local", boardName)
	}
	i2c, _ := localB.I2CByName("bus1")

	gpsDevice, ok := gps.FromRobot(myRobot, gpsName)
	if !ok {
		return errors.Errorf("%q not found or not a gps", gpsName)
	}

	handle, err := i2c.OpenHandle(dispAddr)
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
		localGps, ok := gpsDevice.(gps.LocalGPS)
		if ok {
			valid, _ := localGps.ReadValid(ctx)
			if valid {
				meters, _ = gpsDevice.ReadAltitude(ctx)
			}
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

		handle, err := i2c.OpenHandle(dispAddr)
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
