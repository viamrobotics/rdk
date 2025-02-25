package main

import (
	"context"
	"os"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

func main() {
	logger := logging.NewLogger("test")
	// location owner api key
	client, err := app.CreateViamClientWithOptions(context.Background(), app.Options{
		BaseURL: "https://pr-7646-appmain-bplesliplq-uc.a.run.app",
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
	data, err := dataClient.BinaryDataByID2(context.Background(), []string{"4d122ef3-278d-4f9a-8b5c-e6735fc5d529/8gktlzyf06/TjhvJTxBOGZrPLOAukKczu8Uf3b7hRaigFQtRjKBdG5R7yG8UUyeVk98yhlgAuFj"})
	if err != nil {
		logger.Fatalw("error getting binary data", "error", err)
	}
	filename := "output.jpg"
	err = os.WriteFile(filename, data[0].Binary, 0644)
	if err != nil {
		logger.Fatalw("error writing JPEG file", "error", err)
	}

	data, err = dataClient.BinaryDataByIDs(context.Background(), []*app.BinaryID{
		{
			OrganizationID: "4d122ef3-278d-4f9a-8b5c-e6735fc5d529",
			LocationID:     "8gktlzyf06",
			FileID:         "TjhvJTxBOGZrPLOAukKczu8Uf3b7hRaigFQtRjKBdG5R7yG8UUyeVk98yhlgAuFj",
		},
	})
	if err != nil {
		logger.Fatalw("error getting binary data", "error", err)
	}
	filename = "output2.jpg"
	err = os.WriteFile(filename, data[0].Binary, 0644)
	if err != nil {
		logger.Fatalw("error writing JPEG file", "error", err)
	}
}
