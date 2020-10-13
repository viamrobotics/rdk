package main

import (
	"github.com/echolabsinc/robotcore/gripper"
)

func main() {
	g, err := gripper.NewGripper("192.168.2.155")
	if err != nil {
		panic(err)
	}

	_, err = g.SetPos("17")
	if err != nil {
		panic(err)
	}

	_, err = g.Open()
	if err != nil {
		panic(err)
	}

	_, err = g.Close()
	if err != nil {
		panic(err)
	}

}
