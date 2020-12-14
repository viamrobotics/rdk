package main

import (
	"fmt"
	"log"
	"os"
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

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	fmt.Printf("ready\n")

	for {
		buf := make([]byte, 10)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			log.Fatalf("couldn't read from stdin (%d), %v", n, err)
		}

		if n != 3 {
			log.Printf("bad input (%s)\n", string(buf))
		}

		port.Write([]byte{buf[0], buf[1]})
		time.Sleep(100 * time.Millisecond)

	}

}
