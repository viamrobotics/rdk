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
	"go.viam.com/core/samples/forcesensor"
	_ "go.viam.com/core/board/detector"
	"go.viam.com/utils"

	"github.com/gorilla/websocket"
)

var logger = golog.NewDevelopmentLogger("force-sensor")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// GPIO pins and analog channels - the order matters for reading out the values
	// from the force sensor matrix
	gpioPins := []string{"io26", "io16", "io13"}	// equivalent to GPIO pins 26, 16, 13
	analogChannels := []int{3, 4, 2}				// analog channels 3, 4, 2

	fmc, err := forcesensor.NewForceMatrixController(ctx, gpioPins, analogChannels, logger)
	if err != nil {
		return err
	}
	defer fmc.Close()

	// myRobot, err := robotimpl.New(ctx, &config.Config{}, logger)
	// if err != nil {
	// 	return err
	// }
	// myRobot.AddBoard(b, config.Component{Name: "board1"})

	var wg sync.WaitGroup
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		if err := wsEndpoint(ctx, w, r, fmc); err != nil {
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

func wsEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request,
				fmc *forcesensor.ForceMatrixController) error {

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	log.Println("Client connected")

	// Infinitely loop through, read and send force sensor values to UI
	for {
		select {
		case <- ctx.Done():
			return nil
		default:
		}
		m, err := fmc.Matrix(ctx)
		if err != nil {
			return err
		}

		dataBytes, err := json.Marshal(m)
		err = ws.WriteMessage(1, dataBytes)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}
