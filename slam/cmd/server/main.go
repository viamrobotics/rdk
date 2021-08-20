// Package main runs a server running the SLAM algorithm.
package main

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/lidar/search"
	pb "go.viam.com/core/proto/slam/v1"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor/compass"
	compasslidar "go.viam.com/core/sensor/compass/lidar"
	"go.viam.com/core/slam"

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
	*btf = fakeDev
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
	if argsParsed.Compass.Type != "" && (argsParsed.Compass.Type != config.ComponentTypeSensor || argsParsed.Compass.SubType != compass.Type) {
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
		return errors.Errorf("can only have up to %d lidar offsets", len(argsParsed.Lidars)-1)
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
	default:
		return errors.Errorf("do not know how to make a %q base", args.BaseType)
	}

	components := args.Lidars
	if args.Compass.Type != "" {
		components = append(components, args.Compass)
	}

	r, err := robotimpl.New(ctx, &config.Config{Components: components}, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, r.Close())
	}()
	lidarNames := r.LidarNames()
	lidars := make([]lidar.Lidar, 0, len(lidarNames))
	for _, name := range lidarNames {
		lidar, ok := r.LidarByName(name)
		if !ok {
			continue
		}
		lidars = append(lidars, lidar)
	}
	var compassensor compass.Compass
	if args.Compass.Type != "" {
		var ok bool
		sensorDevice, ok := r.SensorByName(r.SensorNames()[0])
		if !ok {
			return fmt.Errorf("failed to find sensor %q", r.SensorNames()[0])
		}
		compassensor, ok = sensorDevice.(compass.Compass)
		if !ok {
			return errors.Errorf("expected to get a compass but got a %T", sensorDevice)
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

	if compassensor == nil {
		bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidars)
		if err != nil {
			return err
		}
		bestResComp := args.Lidars[bestResDeviceNum]
		logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResComp, bestRes)
		compassensor = compasslidar.From(bestResDevice)
	}

	if compassensor != nil {
		if _, isFake := baseDevice.(*fake.Base); !isFake {
			baseDevice = base.AugmentWithCompass(baseDevice, compassensor, logger)
		}
	}

	lar, err := slam.NewLocationAwareRobot(
		ctx,
		baseDevice,
		area,
		lidars,
		args.LidarOffsets,
		compassensor,
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

	rpcServer, err := rpcserver.New(logger)
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
