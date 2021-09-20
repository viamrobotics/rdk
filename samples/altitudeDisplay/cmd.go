package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"go.viam.com/utils"
	"go.viam.com/core/config"

	_ "go.viam.com/core/board/detector"

	robotimpl "go.viam.com/core/robot/impl"

	"github.com/edaniels/golog"
)

var boardName = "gpsBoard"
var END_BYTES = []byte{0x0D, 0x0A}
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
	i2c, _ := gpsBoard.I2CByName("gps")
	handle, err := i2c.OpenHandle()
	defer handle.Close()
	
	// Init the display multiple times, hoping at least one works- sometimes it takes several writes to get a good init
	for i := 0; i < 50; i++ {
		initDisp(handle)
	}
	
	for true{
		meters, err := GpsAlt(ctx, handle)
		feet := int(meters * 3.28084)
		meterStr := strconv.Itoa(int(meters))
		feetStr := strconv.Itoa(feet)
		if err != nil{
			feetStr = "gps"
			meterStr = "error"
		}else if(meters == -9999){
			feetStr = "no"
			meterStr = "lock"
		}
		
		writeAlt(feetStr, meterStr, handle)
		
		select {
			case <-ctx.Done():
				return nil
			default:
		}
		time.Sleep(1000 * time.Millisecond)
	}
	return nil
}
