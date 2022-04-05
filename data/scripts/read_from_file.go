package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	utils "go.viam.com/rdk/data"
)

func printMetadata(filename string, debugMode bool, marginOfError float64, frequencyHz float64) {
	if filename == "" {
		fmt.Printf("Set -file flag\n")
		return
	}
	if debugMode && frequencyHz == -1 {
		fmt.Printf("Set -frequencyHz flag when in debug mode\n")
		return
	}

	total := 0
	subintCount := 0

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("failed to open file: %v\n", err)
		return
	}

	first, _ := utils.ReadNextSensorData(file)
	firstTimestamp := first.GetMetadata().GetTimeReceived().AsTime()
	next := firstTimestamp
	subIntervalStart := firstTimestamp
	msgCount := 0
	for {
		msg, err := utils.ReadNextSensorData(file)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			fmt.Printf("error reading sensor data: %v\n", err)
			return
		}
		next = msg.GetMetadata().GetTimeReceived().AsTime()
		diff := next.Sub(subIntervalStart)

		if diff < time.Second {
			msgCount += 1
		} else {
			if debugMode {
				if float64(msgCount) < (frequencyHz-(frequencyHz*marginOfError)) ||
					float64(msgCount) > frequencyHz+(frequencyHz*marginOfError) {
					fmt.Printf("msgCount outside of margin of error between %v and %v: %d messages\n", subIntervalStart, next, msgCount)
				}
				fmt.Printf("%d messages between %s and %s\n", msgCount, subIntervalStart, next)
			}
			subIntervalStart = next
			subintCount += 1
			total += msgCount
			msgCount = 0
		}
	}
	fmt.Printf("%d messages over %f minutes\n", total, next.Sub(firstTimestamp).Minutes())
	fmt.Printf("Average number of messages per second: %f\n", float64(total)/float64(subintCount))
}

func main() {
	fileFlag := flag.String("file", "", "file with data")
	debugMode := flag.Bool("debugMode", false, "debug mode")
	marginOfError := flag.Float64("marginOfError", 0.3, "margin of error when in debug mode")
	frequencyHz := flag.Int("frequencyHz", -1, "frequency in hz used when in debug mode")

	flag.Parse()

	printMetadata(*fileFlag, *debugMode, *marginOfError, float64(*frequencyHz))
}
