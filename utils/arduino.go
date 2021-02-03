package utils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jacobsa/go-serial/serial"

	"github.com/viamrobotics/robotcore/rcutil"
)

func findArduinoPort(filter string) (string, error) {
	for _, possibleFile := range []string{"/dev/ttyTHS0"} {
		_, err := os.Stat(possibleFile)
		if err == nil {
			return possibleFile, nil
		}
	}

	lines, err := rcutil.ExecuteShellCommand("arduino-cli", "board", "list")
	if err != nil {
		return "", err
	}

	for _, l := range lines {
		if filter != "" && !strings.Contains(l, filter) {
			continue
		}
		return strings.Split(l, " ")[0], nil
	}

	return "", fmt.Errorf("couldn't find an arduino")
}

func getArduinoSerialConfig(filter string) (serial.OpenOptions, error) {

	options := serial.OpenOptions{
		PortName:        "",
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	portName, err := findArduinoPort(filter)
	if err != nil {
		return options, err
	}

	options.PortName = portName

	return options, nil
}

func ConnectArduinoSerial(filter string) (io.ReadWriteCloser, error) {
	options, err := getArduinoSerialConfig(filter)
	if err != nil {
		return nil, err
	}

	port, err := serial.Open(options)
	if err != nil {
		return nil, err
	}

	return port, nil
}
