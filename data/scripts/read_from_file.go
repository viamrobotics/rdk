package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	utils "go.viam.com/rdk/data"
)

func printStatsHelper(file *os.File, debugMode bool, marginOfError float64, frequencyHz int, printExampleMessage bool) {
	total := 0
	subintCount := 1
<<<<<<< HEAD

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("failed to open file: %v\n", err)
		return
	}

	first, _ := utils.ReadNextSensorData(file)

=======
	first, _ := utils.ReadNextSensorData(file)
	if printExampleMessage {
		data := first.GetData()
		if pc, ok := data.GetFields()["PointCloud"]; ok {
			data_str := pc.GetStringValue()
			data_bytes, _ := base64.StdEncoding.DecodeString(data_str)
			fmt.Println(data_bytes)
		} else {
			fmt.Println(data)
		}
	}
>>>>>>> 80f052e9ac3e268463a51920d3f7e1375604e23b
	firstTimestamp := first.GetMetadata().GetTimeReceived().AsTime()
	next := firstTimestamp
	subIntervalStart := firstTimestamp
	msgCount := 0
	mostRecent := first
	for {
		msg, err := utils.ReadNextSensorData(file)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				fmt.Printf("end of file: %v\n", err)
				break
			}
			fmt.Printf("error reading sensor data: %v\n", err)
			break
		}
		next = msg.GetMetadata().GetTimeReceived().AsTime()
		mostRecent = msg

		diff := next.Sub(subIntervalStart)
		msgCount += 1
		total += 1

		if diff >= time.Second {
			if debugMode {
				fmt.Printf("%d messages between %s and %s\n", msgCount, subIntervalStart, next)
			}
			subIntervalStart = next
			subintCount += 1
			msgCount = 0
		}
	}

	if printExampleMessage {
		data := mostRecent.GetData()
		if pc, ok := data.GetFields()["PointCloud"]; ok {
			data_str := pc.GetStringValue()
			data_bytes, _ := base64.StdEncoding.DecodeString(data_str)
			fmt.Println(data_bytes)
		} else {
			fmt.Println(data)
		}
	}

	fmt.Printf("%d messages over %f minutes\n", total, next.Sub(firstTimestamp).Minutes())
	fmt.Printf("Average number of messages per second: %f\n", float64(total)/float64(subintCount))
}
func printStats(ctx context.Context, filename string, debugMode bool, marginOfError float64, frequencyHz int, printExampleMessage bool, continuousRun bool) {
	if filename == "" {
		fmt.Print("Set -file flag\n")
		return
	}
	if debugMode && frequencyHz == -1 {
		fmt.Print("Set -frequencyHz flag when in debug mode\n")
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("failed to open file: %v\n", err)
		return
	}

	if continuousRun {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				printStatsHelper(file, debugMode, marginOfError, frequencyHz, printExampleMessage)
			}
		}
	} else {
		printStatsHelper(file, debugMode, marginOfError, frequencyHz, printExampleMessage)
	}
}

func main() {
	fileFlag := flag.String("file", "", "file with data")
	debugMode := flag.Bool("debugMode", false, "debug mode")
	printExampleMessage := flag.Bool("printExample", false, "print example message")
	marginOfError := flag.Float64("marginOfError", 0.3, "margin of error when in debug mode")
	frequencyHz := flag.Int("frequencyHz", -1, "frequency in hz used when in debug mode")
	continuousRun := flag.Bool("continuousRun", false, "run in realtime")

	flag.Parse()

	c1, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})

	fmt.Println("Getting file stats. Press ^C to stop.")
	printStats(c1, *fileFlag, *debugMode, *marginOfError, *frequencyHz, *printExampleMessage, *continuousRun)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				exitCh <- struct{}{}
				return
			}
		}
	}(c1)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		select {
		case <-signalCh:
			cancel()
			return
		}
	}()
	<-exitCh
}
