package main

import (
	"github.com/echolabsinc/robotcore/gripper"
	"github.com/echolabsinc/robotcore/utils/log"
)

func main() {
	g, err := gripper.NewGripper("192.168.2.2", log.Global)
	if err != nil {
		panic(err)
	}

	err = g.Open()
	if err != nil {
		panic(err)
	}

	err = g.Close()
	if err != nil {
		panic(err)
	}

}
