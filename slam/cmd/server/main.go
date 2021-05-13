// Package main runs a server running the SLAM algorithm.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/lidar/search"
	pb "go.viam.com/core/proto/slam/v1"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/robots/hellorobot"
	"go.viam.com/core/rpc"
	"go.viam.com/core/sensor/compass"
	compasslidar "go.viam.com/core/sensor/compass/lidar"
	"go.viam.com/core/slam"
	"go.viam.com/core/utils"

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
	Port         utils.NetPortFlag   `flag:"0"`
	BaseType     baseTypeFlag        `flag:"base-type,default=,usage=type of mobile base"`
	Lidars       []config.Component  `flag:"lidar,usage=lidars"`
	LidarOffsets []slam.DeviceOffset `flag:"lidar-offset,usage=lidar offets relative to first"`
	Compass      config.Component    `flag:"compass,usage=compass device"`
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
	for _, comp := range argsParsed.Lidars {
		if comp.Type != config.ComponentTypeLidar {
			return errors.New("only lidar components can be in lidar flag")
		}
	}
	if argsParsed.Compass.Type != "" && (argsParsed.Compass.Type != config.ComponentTypeSensor || argsParsed.Compass.SubType != compass.CompassType) {
		return errors.New("compass flag must be a sensor component")
	}

	if len(argsParsed.Lidars) == 0 {
		argsParsed.Lidars = search.Devices()
		if len(argsParsed.Lidars) != 0 {
			logger.Debugf("detected %d lidars", len(argsParsed.Lidars))
			for _, comp := range argsParsed.Lidars {
				logger.Debug(comp)
			}
		}
	}

	if len(argsParsed.Lidars) == 0 {
		argsParsed.Lidars = append(argsParsed.Lidars,
			config.Component{
				Type:  config.ComponentTypeLidar,
				Host:  "0",
				Model: string(lidar.TypeFake),
			})
	}

	if len(argsParsed.Lidars) == 0 {
		return errors.New("no lidars found")
	}

	if len(argsParsed.LidarOffsets) != 0 && len(argsParsed.LidarOffsets) >= len(argsParsed.Lidars) {
		return fmt.Errorf("can only have up to %d lidar offsets", len(argsParsed.Lidars)-1)
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

	var baseDevice base.Base
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

	components := args.Lidars
	if args.Compass.Type != "" {
		components = append(components, args.Compass)
	}

	r, err := robotimpl.NewRobot(ctx, &config.Config{Components: components}, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, r.Close())
	}()
	lidarNames := r.LidarNames()
	lidars := make([]lidar.Lidar, 0, len(lidarNames))
	for _, name := range lidarNames {
		lidars = append(lidars, r.LidarByName(name))
	}
	var compassSensor compass.Compass
	if args.Compass.Type != "" {
		var ok bool
		sensorDevice := r.SensorByName(r.SensorNames()[0])
		compassSensor, ok = sensorDevice.(compass.Compass)
		if !ok {
			return fmt.Errorf("expected to get a compasss but got a %T", sensorDevice)
		}
	}

	for _, lidarDev := range lidars {
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
		bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidars)
		if err != nil {
			return err
		}
		bestResComp := args.Lidars[bestResDeviceNum]
		logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResComp, bestRes)
		compassSensor = compasslidar.From(bestResDevice)
	}

	if compassSensor != nil {
		if _, isFake := baseDevice.(*fake.Base); !isFake {
			baseDevice = base.AugmentWithCompass(baseDevice, compassSensor, logger)
		}
	}

	lar, err := slam.NewLocationAwareRobot(
		ctx,
		baseDevice,
		area,
		lidars,
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
