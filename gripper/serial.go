package gripper

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
)

type SerialGripper struct {
	port io.ReadWriteCloser
	lock sync.Mutex

	lastLine string // TODO(erh) not thread safe
}

func (sg *SerialGripper) Open() error {
	_, err := sg.send("gg0\r", false)
	return err
}

func (sg *SerialGripper) Grab() (bool, error) {
	_, err := sg.send("gg1\r", false)
	if err != nil {
		return false, err
	}

	time.Sleep(500 * time.Millisecond)

	res, err := sg.send("gp\r", true)
	if err != nil {
		return false, err
	}

	if !strings.HasPrefix(res, "gp:") {
		return false, fmt.Errorf("bad gp response, got (%s)", res)
	}

	res = res[3:]
	res = strings.Split(res, " ")[0]
	val, err := strconv.ParseFloat(res, 32)
	if err != nil {
		return false, fmt.Errorf("bad gp response, can't parse, got (%s)", res)
	}

	return val < .7, nil
}

func (sg *SerialGripper) Close() error {
	sg.port.Close()
	return nil
}

func (sg *SerialGripper) send(cmd string, response bool) (string, error) {
	sg.lock.Lock()
	defer sg.lock.Unlock()

	sg.lastLine = ""

	_, err := sg.port.Write([]byte(cmd))
	if err != nil {
		return "", err
	}

	if !response {
		return "", nil
	}

	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)

		if sg.lastLine == "" {
			continue
		}

		return sg.lastLine, nil
	}
	return "", fmt.Errorf("no last line")
}

func (sg *SerialGripper) processLine(line string) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	if line[0] == '@' {
		golog.Global.Debugf("gripper debug: %s", line)
		return
	}

	sg.lastLine = line
	golog.Global.Debugf("gripper got response: %s", line)
}

func NewSerialGripper(port io.ReadWriteCloser) (Gripper, error) {
	sg := &SerialGripper{}
	sg.port = port

	go func() {
		in := bufio.NewReader(port)
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				golog.Global.Fatalf("can't read from serial: %s", err)
			} else {
				sg.processLine(line)
			}
		}
	}()

	return sg, nil
}
