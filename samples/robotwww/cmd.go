package main

import (
	"context"
	"flag"
	"os"
	"runtime/pprof"

	"github.com/erh/egoutil"
	"go.opencensus.io/trace"

	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	exp := egoutil.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	if flag.NArg() == 0 {
		panic("need to specify a config file")
	}

	cfgFile := flag.Arg(0)
	cfg, err := robot.ReadConfig(cfgFile)
	if err != nil {
		panic(err)
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg)
	if err != nil {
		panic(err)
	}

	err = web.RunWeb(myRobot)
	if err != nil {
		panic(err)
	}
}
