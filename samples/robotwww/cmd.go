package main

import (
	"flag"

	"github.com/viamrobotics/robotcore/robot"
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

	myRobot, err := robot.NewRobot(cfg)
	if err != nil {
		panic(err)
	}
	defer myRobot.Close()

	err = robot.RunWeb(myRobot)
	if err != nil {
		panic(err)
	}
}
