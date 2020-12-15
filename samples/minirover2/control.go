package main

import (
	"bufio"
	"fmt"
	"io"
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

type Rover struct {
	out io.Writer
}

func (r *Rover) Forward(power int) error {
	s := fmt.Sprintf( "0f%d\r" +
		"1f%d\r" +
		"2f%d\r" +
		"3f%d\r", power, power, power, power);
	_, err := r.out.Write([]byte(s));
	return err
}

func (r *Rover) Backward(power int) error {
	s := fmt.Sprintf( "0b%d\r" +
		"1b%d\r" +
		"2b%d\r" +
		"3b%d\r", power, power, power, power);
	_, err := r.out.Write([]byte(s));
	return err
}

func (r *Rover) Stop() error {
	s := fmt.Sprintf( "0s\r" +
		"1s\r" +
		"2s\r" +
		"3s\r")
	_, err := r.out.Write([]byte(s));
	return err
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

	rover := Rover{port}
	defer rover.Stop()
	
	if true {
		for {
			err = rover.Forward(45)
			if err != nil {
				log.Fatalf("couldn't move rover %v", err)
			}
			
			time.Sleep(2000 * time.Millisecond)
			
			err = rover.Stop()
			if err != nil {
				log.Fatalf("couldn't stop rover %v", err)
			}
			
			time.Sleep(2000 * time.Millisecond)

			err = rover.Backward(45)
			if err != nil {
				log.Fatalf("couldn't move rover %v", err)
			}
		
			time.Sleep(2000 * time.Millisecond)
		}
		return
	}
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("couldn't read from stdin %v", err)
		}

		log.Print(line)
		
		/*
		port.Write([]byte{buf[0], buf[1]})
		time.Sleep(100 * time.Millisecond)
*/
	}

}
