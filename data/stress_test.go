package data

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

// Pseudo-code for stress test
// Given: some file over some time.Duration
// Ensure that data was consistently captured at CAPTURE_RATE with some MOE (Margin Of Error)
// For every SUB_INTERVAL:
//     msgCount = 0
//     for every msg in SUB_INTERVAL:
//         validateContents(msg)
//         msgCount += 1
//     expCount := SUB_INTERVAL/CAPTURE_RATE
//     ensureMsgCount(msgCount, expCount, MOE)

func TestDataManagerFile(t *testing.T) {
	subInterval := time.Second
	captureRate := time.Millisecond
	capturesPerSubInt := float64(subInterval / captureRate)
	marginOfError := 0.05

	file, err := os.Open("file_name")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	first, _ := readNextSensorData(file)
	last := first.GetMetadata().GetTimeReceived().AsTime()
	msgCount := 0
	for {
		msg, err := readNextSensorData(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("error reading sensor data: %v", err)
		}
		next := msg.GetMetadata().GetTimeReceived().AsTime()
		if next.Sub(last) < subInterval {
			msgCount += 1
		} else {
			if float64(msgCount) < (capturesPerSubInt-(capturesPerSubInt*marginOfError)) ||
				float64(msgCount) > capturesPerSubInt+(capturesPerSubInt*marginOfError) {
				t.Fatalf("msgCount outside of margin of error between %v and %v: %d messages", last, next, msgCount)
			}
			t.Logf("msgCount within margin of error between %v and %v: %d messages", last, next, msgCount)
		}
		last = next
	}
	fmt.Println("yay, passed")
}
