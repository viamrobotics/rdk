package main

import (
	"context"
	"flag"

	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
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
