package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"nhooyr.io/websocket"
)

func main() {
	var printData bool
	flag.BoolVar(&printData, "print", false, "print data")
	flag.Parse()

	port := 4444
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(0), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	conn, _, err := websocket.Dial(context.Background(), fmt.Sprintf("ws://localhost:%d", port), nil)
	if err != nil {
		golog.Global.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	count := 0
	start := time.Now()
READ:
	for {
		select {
		case <-sig:
			break READ
		default:
		}
		_, data, err := conn.Read(context.Background())
		if err != nil {
			if errors.Is(err, io.EOF) {
				break READ
			}
			golog.Global.Fatal(err)
		}
		if printData {
			golog.Global.Infow("heading", "data", string(data))
		}
		count++
	}
	golog.Global.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())
}
