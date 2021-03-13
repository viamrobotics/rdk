package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/erh/egoutil"
	"go.opencensus.io/trace"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	err := mainReal()
	if err != nil {
		panic(err)
	}
}

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

	myRobot, err := robot.NewRobot(context.Background(), cfg)
	if err != nil {
		return err
	}

	options := web.NewOptions()

	if *noAutoTile {
		options.AutoTile = false
	}

	err = web.RunWeb(myRobot, options)
	if err != nil {
		return err
	}

	return nil
}
