package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/viamrobotics/robotcore/sensor/compass/mti"

	"github.com/edaniels/golog"
	"nhooyr.io/websocket"
)

func main() {
	var disableRateLimit bool
	flag.BoolVar(&disableRateLimit, "disable-rate-limit", false, "disable rate limiting")
	flag.Parse()

	port := 4444
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(0), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	sensor, err := mti.New("02782090", "/dev/ttyUSB0", 115200)
	if err != nil {
		golog.Global.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	count := 0
	var start time.Time
	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			golog.Global.Error("error making websocket connection", "error", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		var tickRate time.Duration
		if disableRateLimit {
			tickRate = time.Nanosecond
		} else {
			tickRate = 100 * time.Millisecond
		}
		ticker := time.NewTicker(tickRate)
		defer ticker.Stop()
		start = time.Now()
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			select {
			case <-r.Context().Done():
			case <-ticker.C:
			}
			heading, err := sensor.Heading()
			if err != nil {
				golog.Global.Errorw("failed to get sensor heading", "error", err)
				continue
			}
			count++
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(fmt.Sprintf("%f", heading))); err != nil {
				golog.Global.Warnw("failed to write to ws conn", "error", err)
			}
		}
	})

	errChan := make(chan error, 1)
	go func() {
		golog.Global.Infow("listening", "url", fmt.Sprintf("http://localhost:%d", port), "port", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	select {
	case err := <-errChan:
		golog.Global.Errorw("failed to serve", "error", err)
	case <-sig:
	}

	if err := httpServer.Shutdown(context.Background()); err != nil {
		golog.Global.Fatal(err)
	}
	golog.Global.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())

}
