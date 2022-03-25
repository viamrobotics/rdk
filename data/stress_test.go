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
	subInterval := 1 * time.Second
	captureRate := time.Millisecond
	capturesPerSubInt := float64(subInterval / captureRate)
	marginOfError := 0.3
	total := 0
	subintCount := 0

	file, err := os.Open("/Users/aaroncasas/2022-03-25T18:17:42.439164936Z")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	first, _ := readNextSensorData(file)
	subIntervalStart := first.GetMetadata().GetTimeReceived().AsTime()
	msgCount := 0
	for {
		msg, err := readNextSensorData(file)
		if err != nil {
			// Why is it getting unexpected EOF? Probably worth digging into
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			t.Fatalf("error reading sensor data: %v", err)
		}
		next := msg.GetMetadata().GetTimeReceived().AsTime()
		diff := next.Sub(subIntervalStart)
		//fmt.Println(msg.GetMetadata().GetTimeReceived().AsTime().Sub(msg.GetMetadata().GetTimeRequested().AsTime()))

		if diff < subInterval {
			msgCount += 1
		} else {
			if float64(msgCount) < (capturesPerSubInt-(capturesPerSubInt*marginOfError)) ||
				float64(msgCount) > capturesPerSubInt+(capturesPerSubInt*marginOfError) {
				t.Fatalf("msgCount outside of margin of error between %v and %v: %d messages", subIntervalStart, next, msgCount)
			}
			//t.Logf("msgCount within margin of error between %v and %v: %d messages", subIntervalStart, next, msgCount)
			subIntervalStart = next
			subintCount += 1
			total += msgCount
			msgCount = 0
		}
	}
	fmt.Printf("Average number of messages per subint: %f\n", float64(total)/float64(subintCount))
	fmt.Println("yay, passed")
}
