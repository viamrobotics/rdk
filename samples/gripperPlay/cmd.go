package main

import (
	"github.com/echolabsinc/robotcore/gripper"

	"github.com/edaniels/golog"
)

func main() {
	g, err := gripper.NewGripper("192.168.2.2", golog.Global)
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
