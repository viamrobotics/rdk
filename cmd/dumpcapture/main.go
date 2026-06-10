// Command dumpcapture prints the metadata and decoded SensorData payloads of a
// .capture/.prog file so you can see exactly what the data manager captured.
//
// Usage:
//
//	go run ./cmd/dumpcapture /path/to/file.capture
package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/data"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "" {
		fmt.Println("usage: go run ./cmd/dumpcapture <file.capture>")
		os.Exit(2)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	cf, err := data.ReadCaptureFile(f)
	if err != nil {
		panic(err)
	}

	fmt.Println("=== METADATA ===")
	fmt.Println(protojson.Format(cf.ReadMetadata()))

	fmt.Println("=== SENSOR DATA PAYLOADS ===")
	i := 0
	for {
		sd, err := cf.ReadNext()
		if err != nil {
			break // EOF
		}
		i++
		fmt.Printf("--- reading %d ---\n%s\n", i, protojson.Format(sd))
	}
	fmt.Printf("total readings: %d\n", i)
}
