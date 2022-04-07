package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
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

	myRobot, err := robotimpl.RobotFromConfigPath(ctx, flag.Arg(0), logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	gpsBoard, err := board.FromRobot(myRobot, boardName)
	if err != nil {
		return err
	}
	localB, ok := gpsBoard.(board.LocalBoard)
	if !ok {
		return fmt.Errorf("board %s is not local", boardName)
	}
	i2c, _ := localB.I2CByName("bus1")

	gpsDevice, err := gps.FromRobot(myRobot, gpsName)
	if err != nil {
		return err
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
