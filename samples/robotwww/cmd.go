package main

import (
	"context"
	"flag"

	"github.com/viamrobotics/robotcore/robot"
	"github.com/viamrobotics/robotcore/robot/web"
)

func main() {

	flag.Parse()

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
