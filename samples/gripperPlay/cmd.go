package main

import (
	"github.com/echolabsinc/robotcore/gripper"
)

func main() {
	g, err := gripper.NewGripper("192.168.2.2")
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
