package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/robot"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
)

const (
	PanCenter  = 83
	TiltCenter = 65
)

// ------

type Rover struct {
	port       io.ReadWriteCloser
	sendLock   sync.Mutex
	lastStatus string
}

func (r *Rover) Close() {
	r.Stop()
	r.port.Close()
}

func (r *Rover) MoveStraight(distanceMM int, speed int, block bool) error {
	if distanceMM == 0 && block {
		return fmt.Errorf("cannot block unless you have a distance")
	}

	if distanceMM != 0 && speed <= 0 {
		return fmt.Errorf("if distanceMM is set, speed has to be positive")
	}

	d := "f"
	if distanceMM < 0 || speed < 0 {
		d = "b"
		distanceMM = rcutil.AbsInt(distanceMM)
		speed = rcutil.AbsInt(speed)
	}

	power := speed // TODO(erh): convert speed to power at some point

	var err error
	ticks := int(float64(distanceMM) * .35)
	err = r.moveTicks(d, d, d, d, power, power, power, power, ticks, ticks, ticks, ticks)

	if err != nil {
		return err
	}

	if !block {
		return nil
	}

	r.waitForMotorsToStop()
	return nil
}

func (r *Rover) waitForMotorsToStop() {
	time.Sleep(10 * time.Millisecond)
	r.lastStatus = ""
	time.Sleep(10 * time.Millisecond)

	for {
		time.Sleep(10 * time.Millisecond)
		r.sendCommand("?\r")
		time.Sleep(10 * time.Millisecond)

		if len(r.lastStatus) == 0 {
			continue
		}

		if r.lastStatus == "#0000" {
			break
		}
	}

}

func (r *Rover) Spin(degrees int, power int, block bool) error {

	if power < 100 {
		power = 100
	}

	a, b := "f", "b"
	if degrees < 0 {
		a, b = "b", "f"
	}

	ticks := rcutil.AbsInt(degrees * 5)

	err := r.moveTicks(
		a, b, a, b,
		power, power, power, power,
		ticks, ticks, ticks, ticks)

	if err != nil {
		return err
	}

	if !block {
		return nil
	}

	r.waitForMotorsToStop()
	return nil
}

func (r *Rover) Stop() error {
	s := fmt.Sprintf("0s\r" +
		"1s\r" +
		"2s\r" +
		"3s\r")
	return r.sendCommand(s)
}

func (r *Rover) moveTicks(a1, a2, a3, a4 string, p1, p2, p3, p4 int, t1, t2, t3, t4 int) error {
	s := fmt.Sprintf("0%s%d,%d\r"+
		"1%s%d,%d\r"+
		"2%s%d,%d\r"+
		"3%s%d,%d\r", a1, p1, t1, a2, p2, t2, a3, p3, t3, a4, p4, t4)
	return r.sendCommand(s)
}

func (r *Rover) sendCommand(cmd string) error {
	if len(cmd) > 2 {
		fmt.Println(strings.ReplaceAll(cmd, "\r", "\n"))
	}

	r.sendLock.Lock()
	defer r.sendLock.Unlock()
	_, err := r.port.Write([]byte(cmd))
	return err
}

func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(PanCenter+(left*-30), TiltCenter-20)
}

func (r *Rover) neckPosition(pan, tilt int) error {
	return r.sendCommand(fmt.Sprintf("p%d\rt%d\r", pan, tilt))
}

func (r *Rover) processLine(line string) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	if line[0] != '#' {
		fmt.Printf("debug line from rover: %s\n", line)
		return
	}

	r.lastStatus = line
}

// ---

func driveMyself(rover *Rover, camera vision.ImageDepthSource) {
	for {
		img, dm, err := camera.NextImageDepthPair(context.TODO())
		if err != nil {
			golog.Global.Debugf("error reading camera: %s", err)
			time.Sleep(2000 * time.Millisecond)
			continue
		}
		func() {
			vImg, err := vision.NewImage(img)
			if err != nil {
				golog.Global.Debugf("error parsing image: %s", err)
				return
			}

			pc := vision.PointCloud{dm, vImg}
			pc, err = pc.CropToDepthData()

			if err != nil || pc.Depth.Width() < 10 || pc.Depth.Height() < 10 {
				golog.Global.Debugf("error getting deth info: %s, backing up", err)
				rover.MoveStraight(-200, 60, true)
				return
			}

			_, points := roverWalk(&pc, false)
			if points < 100 {
				golog.Global.Debugf("safe to move forward")
				err = rover.MoveStraight(200, 50, true)
			} else {
				golog.Global.Debugf("not safe, let's spin")
				err = rover.Spin(-15, 60, true)
			}

			if err != nil {
				fmt.Println(err)
			}
		}()

	}
}

// ---

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	port, err := robot.ConnectArduinoSerial("Mega")
	if err != nil {
		golog.Global.Fatalf("can't connecto to arduino: %v", err)
	}

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	rover := Rover{}
	rover.port = port

	golog.Global.Debug("rover ready")

	go func() {
		for {
			time.Sleep(1500 * time.Millisecond)
			rover.neckCenter()
			time.Sleep(1500 * time.Millisecond)
			rover.neckOffset(-1)
			time.Sleep(1500 * time.Millisecond)
			rover.neckOffset(1)
		}
	}()

	go func() {
		in := bufio.NewReader(port)
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				panic(err)
			} else {
				rover.processLine(line)
			}
		}
	}()

	theRobot := robot.NewBlankRobot()
	theRobot.AddBase(&rover, robot.Component{})
	theRobot.AddCamera(&vision.RotateImageDepthSource{vision.NewIntelServerSource(srcURL, 8181, nil)}, robot.Component{})

	defer theRobot.Close()

	if false {
		go driveMyself(&rover, theRobot.Cameras[0])
	}

	robot.RunWeb(theRobot)
}
