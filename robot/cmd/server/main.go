package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rlog"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"

	// These are the robot pieces we want by default
	_ "go.viam.com/robotcore/board/detector"
	_ "go.viam.com/robotcore/rimage/imagesource"
	_ "go.viam.com/robotcore/robots/eva" // for eva
	_ "go.viam.com/robotcore/robots/hellorobot"
	_ "go.viam.com/robotcore/robots/robotiq"         // for a gripper
	_ "go.viam.com/robotcore/robots/universalrobots" // for an arm
	_ "go.viam.com/robotcore/robots/vgripper"        // for a gripper
	_ "go.viam.com/robotcore/robots/wx250s"          // for arm and gripper

	"github.com/erh/egoutil"
	"go.opencensus.io/trace"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var webprofile = flag.Bool("webprofile", false, "include profiler in http server")

func main() {
	err := mainReal()
	if err != nil {
		panic(err)
	}
}

var logger = rlog.Logger.Named("robot_server")

func mainReal() error {
	noAutoTile := flag.Bool("noAutoTile", false, "disable auto tiling")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
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

	if flag.NArg() == 0 {
		return fmt.Errorf("need to specify a config file")
	}

	cfgFile := flag.Arg(0)
	cfg, err := api.ReadConfig(cfgFile)
	if err != nil {
		return err
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		return err
	}

	options := web.NewOptions()

	if *noAutoTile {
		options.AutoTile = false
	}
	if *webprofile {
		options.Pprof = true
	}

	err = web.RunWeb(myRobot, options, logger)
	if err != nil {
		return err
	}

	return nil
}
