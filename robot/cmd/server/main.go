package main

import (
	"context"
	"os"
	"runtime/pprof"

	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/utils"

	// These are the robot pieces we want by default
	_ "go.viam.com/robotcore/board/detector"
	_ "go.viam.com/robotcore/rimage/imagesource"
	_ "go.viam.com/robotcore/robots/eva" // for eva
	_ "go.viam.com/robotcore/robots/hellorobot"
	_ "go.viam.com/robotcore/robots/robotiq"         // for a gripper
	_ "go.viam.com/robotcore/robots/universalrobots" // for an arm
	_ "go.viam.com/robotcore/robots/vgripper"        // for a gripper
	_ "go.viam.com/robotcore/robots/wx250s"          // for arm and gripper

	"github.com/edaniels/golog"
	"github.com/erh/egoutil"
	"go.opencensus.io/trace"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = golog.NewDevelopmentLogger("robot_server")

// Arguments for the command.
type Arguments struct {
	ConfigFile string            `flag:"0,required,usage=robot config file"`
	NoAutoTile bool              `flag:"noAutoTile,usage=disable auto tiling"`
	CPUProfile string            `flag:"cpuprofile,usage=write cpu profile to file"`
	WebProfile bool              `flag:"webprofile,usage=include profiler in http server"`
	Port       utils.NetPortFlag `flag:"port,usage=port to listen on"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = 8080
	}

	if argsParsed.CPUProfile != "" {
		f, err := os.Create(argsParsed.CPUProfile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	exp := egoutil.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	cfg, err := api.ReadConfig(argsParsed.ConfigFile)
	if err != nil {
		return err
	}

	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)
	myRobot, err := robot.NewRobot(ctx, cfg, logger)
	if err != nil {
		return err
	}

	options := web.NewOptions()
	options.AutoTile = !argsParsed.NoAutoTile
	options.Pprof = argsParsed.WebProfile
	options.Port = int(argsParsed.Port)

	return web.RunWeb(ctx, myRobot, options, logger)
}
