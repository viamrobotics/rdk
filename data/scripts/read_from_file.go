package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"os"
	"os/signal"
	"time"

	utils "go.viam.com/rdk/data"
)

// 100hz
func writeDummyData(ctx context.Context, filename string) {
	ticker := time.NewTicker(time.Millisecond * 10)
	file, err := os.OpenFile(filename, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		fmt.Printf("failed to open file: %v\n", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			md := v1.SensorMetadata{
				TimeRequested: timestamppb.New(time.Now().UTC()),
				TimeReceived:  timestamppb.New(time.Now().UTC()),
			}
			data, _ := utils.StructToStructPb(&md)
			nextReading := v1.SensorData{
				Metadata: &md,
				Data:     data,
			}
			_, err := pbutil.WriteDelimited(file, &nextReading)
			if err != nil {
				fmt.Printf("error writing file: %v", err)
			}
		}
	}
}

func printStats(ctx context.Context, filename string, debugMode bool, marginOfError float64, frequencyHz int, printExampleMessage bool) {
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
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := 0
			subintCount := 1
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
					break
				}
				next = msg.GetMetadata().GetTimeReceived().AsTime()
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
			fmt.Printf("%d messages over %f seconds\n", total, next.Sub(firstTimestamp).Seconds())
			fmt.Printf("Average number of messages per second: %f\n", float64(total)/float64(subintCount))
		}
	}
}

func main() {
	fileFlag := flag.String("file", "", "file with data")
	debugMode := flag.Bool("debugMode", false, "debug mode")
	printExampleMessage := flag.Bool("printExample", false, "print example message")
	marginOfError := flag.Float64("marginOfError", 0.3, "margin of error when in debug mode")
	frequencyHz := flag.Int("frequencyHz", -1, "frequency in hz used when in debug mode")

	flag.Parse()

	c1, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})

	fmt.Println("Getting file stats. Press ^C to stop.")
	//go writeDummyData(c1, *fileFlag)
	printStats(c1, *fileFlag, *debugMode, *marginOfError, *frequencyHz, *printExampleMessage)

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
