package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/slam"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"go.uber.org/multierr"
)

const fakeDev = "fake"

func main() {
	utils.ContextualMain(mainWithArgs)
}

var (
	defaultPort = 5555
	logger      = golog.Global
)

// Arguments for the command.
type Arguments struct {
	Port         utils.NetPortFlag         `flag:"0"`
	BaseType     baseTypeFlag              `flag:"base-type,default=,usage=type of mobile base"`
	LidarDevices []lidar.DeviceDescription `flag:"lidar,usage=lidar devices"`
	LidarOffsets []slam.DeviceOffset       `flag:"lidar-offset,usage=lidar device offets relative to first"`
	Compass      string                    `flag:"compass,usage=compass device"`
}

type baseTypeFlag string

func (btf *baseTypeFlag) String() string {
	return string(*btf)
}

func (btf *baseTypeFlag) Set(val string) error {
	if val != "" {
		*btf = baseTypeFlag(val)
		return nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	if runtime.GOOS == "linux" && strings.Contains(hostname, "stretch") {
		*btf = "hello"
	} else {
		*btf = fakeDev
	}
	return nil
}

func (btf *baseTypeFlag) Get() interface{} {
	return string(*btf)
}

func mainWithArgs(ctx context.Context, args []string) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = search.Devices()
		if len(argsParsed.LidarDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(argsParsed.LidarDevices))
			for _, desc := range argsParsed.LidarDevices {
				logger.Debugf("%s (%s)", desc.Type, desc.Path)
			}
		}
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = append(argsParsed.LidarDevices,
			lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: "0"})
	}

	if len(argsParsed.LidarDevices) == 0 {
		return errors.New("no lidar devices found")
	}

	if len(argsParsed.LidarOffsets) != 0 && len(argsParsed.LidarOffsets) >= len(argsParsed.LidarDevices) {
		return fmt.Errorf("can only have up to %d lidar device offsets", len(argsParsed.LidarDevices)-1)
	}

	return runSlam(ctx, argsParsed)
}

func runSlam(ctx context.Context, args Arguments) (err error) {
	areaSizeMeters := 50
	unitsPerMeter := 100 // cm
	area, err := slam.NewSquareArea(areaSizeMeters, unitsPerMeter)
	if err != nil {
		return err
	}

	var baseDevice api.Base
	switch args.BaseType {
	case fakeDev:
		baseDevice = &fake.Base{}
	case "hello":
		robot, err := hellorobot.New()
		if err != nil {
			return err
		}
		baseDevice, err = robot.Base()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("do not know how to make a %q base", args.BaseType)
	}

	var compassSensor compass.Device
	if args.Compass != "" {
		sensor, err := compass.NewWSDevice(ctx, args.Compass)
		if err != nil {
			return err
		}
		compassSensor = sensor
	}

	lidarDevices, err := lidar.CreateDevices(ctx, args.LidarDevices)
	if err != nil {
		return err
	}
	for _, lidarDev := range lidarDevices {
		if err := lidarDev.Start(ctx); err != nil {
			return err
		}
		info, infoErr := lidarDev.Info(ctx)
		if infoErr != nil {
			return infoErr
		}
		logger.Infow("device", "info", info)
		dev := lidarDev
		defer func() {
			err = multierr.Combine(err, dev.Stop(context.Background()))
		}()
	}

	if compassSensor == nil {
		bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidarDevices)
		if err != nil {
			return err
		}
		bestResDesc := args.LidarDevices[bestResDeviceNum]
		logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResDesc.Path, bestRes)
		compassSensor = compasslidar.From(bestResDevice)
	}

	if compassSensor != nil {
		if _, isFake := baseDevice.(*fake.Base); !isFake {
			baseDevice = compass.BaseWithCompass(baseDevice, compassSensor)
		}
	}

	lar, err := slam.NewLocationAwareRobot(
		ctx,
		baseDevice,
		area,
		lidarDevices,
		args.LidarOffsets,
		compassSensor,
	)
	if err != nil {
		return err
	}
	if err := lar.Start(); err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(lar.Stop())
	}()
	areaViewer := &slam.AreaViewer{area}

	config := x264.DefaultViewConfig
	config.StreamName = "robot view"
	remoteView, err := gostream.NewView(config)
	if err != nil {
		return err
	}
	lar.RegisterCommands(ctx, remoteView.CommandRegistry())

	clientWidth := 800
	clientHeight := 600

	remoteView.SetOnClickHandler(func(x, y int) {
		logger.Debugw("click", "x", x, "y", y)
		resp, err := lar.HandleClick(ctx, x, y, clientWidth, clientHeight)
		if err != nil {
			remoteView.SendTextToAll(err.Error())
			return
		}
		if resp != "" {
			remoteView.SendTextToAll(resp)
		}
	})

	server := gostream.NewViewServer(int(args.Port), remoteView, logger)
	if err := server.Start(); err != nil {
		return err
	}

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	started := make(chan struct{})
	go func() {
		close(started)
		gostream.StreamNamedSource(ctx, robotViewMatSource, "robot perspective", remoteView)
	}()
	<-started

	utils.ContextMainReadyFunc(ctx)()
	gostream.StreamNamedSource(ctx, worldViewMatSource, "world (published)", remoteView)

	return server.Stop(context.Background())
}
