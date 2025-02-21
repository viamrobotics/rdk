package main

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

var query = []map[string]any{
	{
		"$match": map[string]interface{}{
			"robot_id": "e1aced6f-50c7-47dc-b67b-9b15c5ba44f9",
		},
	},
	{
		"$limit": 100,
	},
}

func runQueries(dataClient *app.DataClient, queries [][]map[string]any, logger logging.Logger) {
	for _, query := range queries {
		// Run normal query
		startTime := time.Now()
		data, err := dataClient.TabularDataByMQL(context.Background(), "8fd9f6ec-08df-45dd-9ae9-80ff54533348", query, &app.TabularDataByMQLOptions{
			UseRecentData: false,
		})
		if err != nil {
			logger.Errorw("failed to fetch data", "error", err, "query", query)
			continue
		}
		respTime := time.Since(startTime)

		// Run realtime query
		realtimeStartTime := time.Now()
		realtimeData, err := dataClient.TabularDataByMQL(context.Background(), "8fd9f6ec-08df-45dd-9ae9-80ff54533348", query, &app.TabularDataByMQLOptions{
			UseRecentData: true,
		})
		if err != nil {
			logger.Errorw("failed to fetch realtime data", "error", err, "query", query)
			continue
		}
		realtimeRespTime := time.Since(realtimeStartTime)

		// Print results
		fmt.Printf("Query: %v\n", query)
		fmt.Printf("Normal query results: %v\n", data)
		fmt.Printf("Realtime query results: %v\n", realtimeData)
		fmt.Printf("Time taken for normal query: %s\n", respTime)
		fmt.Printf("Time taken for realtime query: %s\n", realtimeRespTime)
		fmt.Println("----------------------------------------")
	}
}

func main1() {
	logger := logging.NewLogger("test")
	// location owner api key
	client, err := app.CreateViamClientWithOptions(context.Background(), app.Options{
		BaseURL: "https://pr-7601-appmain-bplesliplq-uc.a.run.app",
		Entity:  "8fba595a-10fd-4d0e-8380-66908fcbb271",
		Credentials: rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: "mfnfe5usraa2zsfz6f6081urrq1birkh",
		},
	}, logger)
	if err != nil {
		logger.Fatalw("error creating viam client", "error", err)
	}

	// Example queries
	queries := [][]map[string]any{
		{
			{
				"$match": map[string]interface{}{
					"robot_id": "f25b20a0-dae5-44ab-abaf-fef72285cfc3",
				},
			},
			{
				"$limit": 1,
			},
		}, // Using the existing query
		{
			{
				"$match": map[string]interface{}{
					"part_id": "4e80bcc7-502d-4fae-99ab-9d42e0344171",
				},
			},
			{
				"$count": "foo",
			},
		},
		// ... existing code ...
		// {
		// 	{
		// 		"$match": map[string]interface{}{
		// 			"$or": []map[string]interface{}{
		// 				{"part_id": "6027eb6a-444b-487e-b50b-d2bef59b98a0"},
		// 				{"robot_id": "e1aced6f-50c7-47dc-b67b-9b15c5ba44f9"},
		// 			},
		// 		},
		// 	},
		// 	{
		// 		"$group": map[string]interface{}{
		// 			"_id": nil,
		// 			"avg": map[string]interface{}{
		// 				"$avg": "$data.value",
		// 			},
		// 			"avg2": map[string]interface{}{
		// 				"$avg": "$data.readings.a",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	{
		// 		"$match": map[string]interface{}{
		// 			"organization_id": "2534fed7-cd10-4b08-bfda-3d8be63c6172",
		// 			"robot_id":        "e1aced6f-50c7-47dc-b67b-9b15c5ba44f9",
		// 		},
		// 	},
		// 	{
		// 		"$group": map[string]interface{}{
		// 			"_id": nil,
		// 			"avg": map[string]interface{}{
		// 				"$avg": "$data.readings.a",
		// 			},
		// 		},
		// 	},
		// },
		// Add more queries as needed
	}

	runQueries(client.DataClient(), queries, logger)
}
