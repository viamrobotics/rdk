package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jacobsa/go-serial/serial"

	"github.com/echolabsinc/robotcore/rcutil"
)

func getSerialConfig() (serial.OpenOptions, error) {

	options := serial.OpenOptions{
		PortName:        "",
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	lines, err := rcutil.ExecuteShellCommand("arduino-cli", "board", "list")
	if err != nil {
		return options, err
	}

	for _, l := range lines {
		if strings.Index(l, "Mega") < 0 {
			continue
		}
		options.PortName = strings.Split(l, " ")[0]
		return options, nil
	}

	return options, fmt.Errorf("couldn't find an arduino")
}

func main() {
	options, err := getSerialConfig()
	if err != nil {
		log.Fatalf("can't get serial config: %v", err)
	}

	port, err := serial.Open(options)
	if err != nil {
		log.Fatalf("can't option serial port %v", err)
	}
	defer port.Close()

	time.Sleep(1000 * time.Millisecond)

	port.Write([]byte("w5"))

}
