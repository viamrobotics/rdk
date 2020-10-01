package main

import (
	"time"

	"github.com/echolabsinc/robotcore/gripper"
)

func main() {
	g, err := gripper.NewGripper("192.168.2.155")
	if err != nil {
		panic(err)
	}

	g.Set("ACT", "1") // robot activate
	g.Set("GTO", "1") // gripper activate

	g.Set("FOR", "50") // force (0-255)

	g.Set("POS", "0") // open
	time.Sleep(500 * time.Millisecond)

	g.Set("POS", "255") // closed
	time.Sleep(500 * time.Millisecond)

	g.Set("POS", "0") // open
	time.Sleep(500 * time.Millisecond)

	g.Set("POS", "255") // closed
	time.Sleep(500 * time.Millisecond)

}
