package gripper

import (
	"fmt"
	"io"
	"sync"
)

type SerialGripper struct {
	port io.ReadWriteCloser
	lock sync.Mutex
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

	// HELP
	return false, nil
}

func (sg *SerialGripper) Close() error {
	sg.port.Close()
	return nil
}

func (sg *SerialGripper) send(cmd string, response bool) (string, error) {
	sg.lock.Lock()
	defer sg.lock.Unlock()

	_, err := sg.port.Write([]byte(cmd))
	if err != nil {
		return "", err
	}

	if !response {
		return "", nil
	}

	return "", fmt.Errorf("response not done")
}

func NewSerialGripper(port io.ReadWriteCloser) (Gripper, error) {
	sg := &SerialGripper{}
	sg.port = port
	return sg, nil
}
