package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/utils/log"

	"github.com/jacobsa/go-serial/serial"
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
		if !strings.Contains(l, "Mega") {
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
		log.Global.Fatalf("can't get serial config: %v", err)
	}

	port, err := serial.Open(options)
	if err != nil {
		log.Global.Fatalf("can't option serial port %v", err)
	}
	defer port.Close()

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	log.Global.Debug("ready\n")

	for {
		buf := make([]byte, 10)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			log.Global.Fatalf("couldn't read from stdin (%d), %v", n, err)
		}

		if n != 3 {
			log.Global.Debugf("bad input (%s)\n", string(buf))
		}

		port.Write([]byte{buf[0], buf[1]})
		time.Sleep(100 * time.Millisecond)

	}

}
