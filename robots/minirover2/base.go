package minirover2

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/viamrobotics/robotcore/base"
	"github.com/viamrobotics/robotcore/serial"
	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/golog"
)

const ModelName = "minirover2"

const (
	PanCenter  = 83
	TiltCenter = 100
)

// ------

type Rover struct {
	port       io.ReadWriteCloser
	sendLock   sync.Mutex
	lastStatus string
}

func (r *Rover) Close() {
	r.Stop() // nolint
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
		distanceMM = utils.AbsInt(distanceMM)
		speed = utils.AbsInt(speed)
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

	return r.waitForMotorsToStop()
}

func (r *Rover) waitForMotorsToStop() error {
	time.Sleep(10 * time.Millisecond)
	r.lastStatus = ""
	time.Sleep(10 * time.Millisecond)

	for {
		time.Sleep(10 * time.Millisecond)
		err := r.sendCommand("?\r")
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)

		if len(r.lastStatus) == 0 {
			continue
		}

		if r.lastStatus == "#0000" {
			break
		}
	}

	return nil
}

func (r *Rover) Spin(degrees float64, power int, block bool) error {

	if power < 120 {
		power = 120
	}

	a, b := "f", "b"
	if degrees < 0 {
		a, b = "b", "f"
	}

	ticks := int(math.Abs(degrees * 5))

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

	return r.waitForMotorsToStop()
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
		golog.Global.Debug("rover cmd[%s]", strings.ReplaceAll(cmd, "\r", ""))
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
		golog.Global.Debug("debug line from rover: %s", line)
		return
	}

	r.lastStatus = line
}

func NewRover(devicePath string) (base.Device, error) {
	port, err := serial.OpenDevice(devicePath)
	if err != nil {
		return nil, fmt.Errorf("can't connect to arduino for rover: %v", err)
	}

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	rover := &Rover{}
	rover.port = port

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

	if false {
		go func() {
			for {
				time.Sleep(1500 * time.Millisecond)
				err := rover.neckCenter()
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(-1)
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(1)
				if err != nil {
					panic(err)
				}

			}
		}()
	} else {
		err = rover.neckCenter()
		if err != nil {
			return nil, err
		}
	}

	golog.Global.Debug("rover ready")

	return rover, nil
}
