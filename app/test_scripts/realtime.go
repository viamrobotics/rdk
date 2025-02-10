package main

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

var query = map[string]any{
	"$match": map[string]interface{}{
		"location_id": "eszagv4veu",
	},
}

func main() {
	logger := logging.NewLogger("test")
	// location owner api key
	client, err := app.CreateViamClientWithOptions(context.Background(), app.Options{
		BaseURL: "https://pr-7424-appmain-bplesliplq-uc.a.run.app",
		Entity:  "",
		Credentials: rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: "",
		},
	}, logger)
	if err != nil {
		logger.Fatalw("error creating viam client", "error", err)
	}
	dataClient := client.DataClient()
	startTime := time.Now()
	data, err := dataClient.TabularDataByMQL(context.Background(), "35f180c7-fa52-4e78-ade0-ccd32dbf1462", []map[string]any{query}, false)
	if err != nil {
		logger.Fatalw("failed to fetch data", "error", err)
	}
	respTime := time.Since(startTime)
	fmt.Println(data)
	realtimestartTime := time.Now()
	data, err = dataClient.TabularDataByMQL(context.Background(), "35f180c7-fa52-4e78-ade0-ccd32dbf1462", []map[string]any{query}, true)
	if err != nil {
		logger.Fatalw("failed to fetch data", "error", err)
	}
	realtimeRespTIme := time.Since(realtimestartTime)
	fmt.Println(data)
	fmt.Println(fmt.Sprintf("Time taken for normal adf query: %s", respTime.String()))
	fmt.Println(fmt.Sprintf("Time taken for realtime query: %s", realtimeRespTIme.String()))

}
