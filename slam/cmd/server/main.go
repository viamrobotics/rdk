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
	pb "go.viam.com/robotcore/proto/slam/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/slam"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
)

const fakeDev = "fake"

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 5555
	logger      = golog.NewDevelopmentLogger("slam_server")
)

// Arguments for the command.
type Arguments struct {
	Port         utils.NetPortFlag     `flag:"0"`
	BaseType     baseTypeFlag          `flag:"base-type,default=,usage=type of mobile base"`
	LidarDevices []api.ComponentConfig `flag:"lidar,usage=lidar devices"`
	LidarOffsets []slam.DeviceOffset   `flag:"lidar-offset,usage=lidar device offets relative to first"`
	Compass      api.ComponentConfig   `flag:"compass,usage=compass device"`
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

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}
	for _, comp := range argsParsed.LidarDevices {
		if comp.Type != api.ComponentTypeLidar {
			return errors.New("only lidar components can be in lidar flag")
		}
	}
	if argsParsed.Compass.Type != "" && (argsParsed.Compass.Type != api.ComponentTypeSensor || argsParsed.Compass.SubType != compass.DeviceType) {
		return errors.New("compass flag must be a sensor component")
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = search.Devices()
		if len(argsParsed.LidarDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(argsParsed.LidarDevices))
			for _, comp := range argsParsed.LidarDevices {
				logger.Debug(comp)
			}
		}
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = append(argsParsed.LidarDevices,
			api.ComponentConfig{
				Type:  api.ComponentTypeLidar,
				Host:  "0",
				Model: string(lidar.DeviceTypeFake),
			})
	}

	if len(argsParsed.LidarDevices) == 0 {
		return errors.New("no lidar devices found")
	}

	if len(argsParsed.LidarOffsets) != 0 && len(argsParsed.LidarOffsets) >= len(argsParsed.LidarDevices) {
		return fmt.Errorf("can only have up to %d lidar device offsets", len(argsParsed.LidarDevices)-1)
	}

	return runSlam(ctx, argsParsed, logger)
}

func runSlam(ctx context.Context, args Arguments, logger golog.Logger) (err error) {
	areaSizeMeters := 50.
	unitsPerMeter := 100. // cm
	area, err := slam.NewSquareArea(areaSizeMeters, unitsPerMeter, logger)
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

	components := args.LidarDevices
	if args.Compass.Type != "" {
		components = append(components, args.Compass)
	}

	r, err := robot.NewRobot(ctx, &api.Config{Components: components}, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, r.Close())
	}()
	lidarNames := r.LidarDeviceNames()
	lidarDevices := make([]lidar.Device, 0, len(lidarNames))
	for _, name := range lidarNames {
		lidarDevices = append(lidarDevices, r.LidarDeviceByName(name))
	}
	var compassSensor compass.Device
	if args.Compass.Type != "" {
		var ok bool
		sensorDevice := r.SensorByName(r.SensorNames()[0])
		compassSensor, ok = sensorDevice.(compass.Device)
		if !ok {
			return fmt.Errorf("expected to get a compasss but got a %T", sensorDevice)
		}
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
		bestResComp := args.LidarDevices[bestResDeviceNum]
		logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResComp, bestRes)
		compassSensor = compasslidar.From(bestResDevice)
	}

	if compassSensor != nil {
		if _, isFake := baseDevice.(*fake.Base); !isFake {
			baseDevice = api.BaseWithCompass(baseDevice, compassSensor, logger)
		}
	}

	lar, err := slam.NewLocationAwareRobot(
		ctx,
		baseDevice,
		area,
		lidarDevices,
		args.LidarOffsets,
		compassSensor,
		logger,
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

	clientWidth := 800
	clientHeight := 600

	rpcServer, err := rpc.NewServer()
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()
	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.SlamService_ServiceDesc,
		slam.NewLocationAwareRobotServer(lar),
		pb.RegisterSlamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	utils.PanicCapturingGo(func() {
		if err := rpcServer.Start(); err != nil {
			logger.Errorw("error starting", "error", err)
		}
	})

	grpcConn, err := grpc.DialContext(ctx, rpcServer.InternalAddr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, grpcConn.Close())
	}()
	slamClient := pb.NewSlamServiceClient(grpcConn)

	remoteView.SetOnDataHandler(func(ctx context.Context, data []byte, responder gostream.ClientResponder) {
		resp, err := rpc.CallClientMethodLineJSON(ctx, slamClient, data)
		if err != nil {
			responder.SendText(err.Error())
			return
		}
		responder.SendText(string(resp))
	})

	remoteView.SetOnClickHandler(func(ctx context.Context, x, y int, responder gostream.ClientResponder) {
		logger.Debugw("click", "x", x, "y", y)
		resp, err := lar.HandleClick(ctx, x, y, clientWidth, clientHeight)
		if err != nil {
			responder.SendText(err.Error())
			return
		}
		if resp != "" {
			responder.SendText(resp)
		}
	})

	server := gostream.NewViewServer(int(args.Port), remoteView, logger)
	if err := server.Start(); err != nil {
		return err
	}

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	started := make(chan struct{})
	utils.PanicCapturingGo(func() {
		close(started)
		gostream.StreamNamedSource(ctx, robotViewMatSource, "robot perspective", remoteView)
	})
	<-started

	utils.ContextMainReadyFunc(ctx)()
	gostream.StreamNamedSource(ctx, worldViewMatSource, "world (published)", remoteView)

	return server.Stop(context.Background())
}
