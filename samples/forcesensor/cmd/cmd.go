package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/core/board"
	// "go.viam.com/core/samples/forcesensor"
	_ "go.viam.com/core/board/detector"
	"go.viam.com/utils"

	"github.com/gorilla/websocket"
)

var logger = golog.NewDevelopmentLogger("force-sensor")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func wsEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request,
				b board.Board, readers []board.AnalogReader) error {
	// Define which GPIO pins are used to activate the force sensor columns
	cols := []string{"io4", "io17", "io27", "io22", "io6"}
	numberCols := len(cols)

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	// helpful log statement to show connections
	log.Println("Client connected")

	// Infinitely loop through, read and send force sensor values to UI
	for {
		select {
		case <- ctx.Done():
			return nil
		default:
		}
		var m [][]int
		for c := 0; c < numberCols; c++ {
			err = b.GPIOSet(cols[c], true)
			if err != nil {
				return err
			}

			for i, pin := range cols {
				if i != c {
					err := b.GPIOSet(pin, false)
					if err != nil {
						return err
					}
				}
			}

			x := []int{}
			for _, analogReader := range readers {
				val, err := analogReader.Read(ctx)
				if err != nil {
					return err
				}
				x = append(x, val)
			}
			m = append(m, x)
		}

		dataBytes, err := json.Marshal(m)
		err = ws.WriteMessage(1, dataBytes)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func createBoard(ctx context.Context, numberAnalogs int, logger golog.Logger) (board.Board, []board.AnalogReader, error) {
	cfg := board.Config{
		Model: "pi",
	}

	for i := 0; i < numberAnalogs; i++ {
		cfg.Analogs = append(cfg.Analogs, board.AnalogConfig{
			Name: fmt.Sprintf("a%d", i),
			Pin:  fmt.Sprintf("%d", i),
		})
	}

	b, err := board.NewBoard(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	readers := []board.AnalogReader{}
	for _, a := range cfg.Analogs {
		readers = append(readers, b.AnalogReader(a.Name))
	}

	return b, readers, nil
}


func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	// Create a board with 5 analog inputs
	numberAnalogs := 5
	b, readers, err := createBoard(ctx, numberAnalogs, logger)
	if err != nil {
		return err
	}
	defer b.Close()

	var wg sync.WaitGroup
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		if err := wsEndpoint(ctx, w, r, b, readers); err != nil {
			fmt.Println(err)
		}
	})

	server := &http.Server{Addr: ":8081", Handler: nil}
	go func() {
		<- ctx.Done()
		server.Shutdown(context.Background())
		wg.Wait()
	} ()
	return server.ListenAndServe()
}