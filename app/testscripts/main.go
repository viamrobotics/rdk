package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.viam.com/rdk/app"
)

// TestTabularDataFunctions demonstrates how to use the Go SDK to call
// ExportTabularData() and GetLatestTabularData() functions.
func TestTabularDataFunctions() {

	// Create a Viam client with API key authentication
	// Note: In a real application, you would get these from environment variables
	// or configuration files
	ctx := context.Background()

	// Example credentials - replace with your actual values
	apiKey := "dhjh680i33uagwpown4fy4g46pifk4jz"
	apiKeyID := "ebb16bb2-d90b-4d54-8c19-f413eecf0a9f"

	opts := app.Options{
		BaseURL: "https://app.viam.dev", // Default Viam app URL
	}

	// Create the Viam client
	client, err := app.CreateViamClientWithAPIKey(ctx, opts, apiKey, apiKeyID, nil)
	if err != nil {
		log.Fatalf("Failed to create Viam client: %v", err)
	}
	defer client.Close()

	// Get the data client
	dataClient := client.DataClient()

	// Test parameters - replace with your actual values
	partID := "3380647d-127a-4fb8-aa7e-253ef7dbccc7"
	resourceName := "sensor-1"                // e.g., "my-sensor"
	resourceSubtype := "rdk:component:sensor" // e.g., "sensor"
	methodName := "DoCommand"                 // e.g., "Readings"

	fmt.Println("=== Testing GetLatestTabularData ===")

	// Test GetLatestTabularData
	latestData, err := dataClient.GetLatestTabularData(ctx, partID, resourceName, resourceSubtype, methodName, nil)
	if err != nil {
		log.Printf("Error getting latest tabular data: %v", err)
	} else {
		fmt.Printf("Latest tabular data retrieved successfully:\n")
		fmt.Printf("  Time Captured: %s\n", latestData.TimeCaptured.Format(time.RFC3339))
		fmt.Printf("  Time Synced: %s\n", latestData.TimeSynced.Format(time.RFC3339))
		fmt.Printf("  Payload: %+v\n", latestData.Payload)
	}

	// Test GetLatestTabularData with no additional parameters
	fmt.Println("=== Testing GetLatestTabularData with no additional parameters")
	latestData, err = dataClient.GetLatestTabularData(ctx, partID, resourceName, resourceSubtype, methodName, nil)
	if err != nil {
		log.Printf("Error getting latest tabular data with no additional parameters: %v", err)
	} else {
		fmt.Printf("Latest tabular data retrieved successfully:\n")
		fmt.Printf("  Time Captured: %s\n", latestData.TimeCaptured.Format(time.RFC3339))
		fmt.Printf("  Time Synced: %s\n", latestData.TimeSynced.Format(time.RFC3339))
		fmt.Printf("  Payload: %+v\n", latestData.Payload)
	}

	// Example with additional parameters
	fmt.Println("=== Testing with Additional Parameters ===")

	additionalParams := &app.TabularDataOptions{
		AdditionalParameters: map[string]interface{}{
			"docommand_input": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	latestDataWithParams, err := dataClient.GetLatestTabularData(ctx, partID, resourceName, resourceSubtype, methodName, additionalParams)
	if err != nil {
		log.Printf("Error getting latest tabular data with parameters: %v", err)
	} else {
		fmt.Printf("Latest tabular data with parameters retrieved successfully:\n")
		fmt.Printf("  Time Captured: %s\n", latestDataWithParams.TimeCaptured.Format(time.RFC3339))
		fmt.Printf("  Time Synced: %s\n", latestDataWithParams.TimeSynced.Format(time.RFC3339))
		fmt.Printf("  Payload: %+v\n", latestDataWithParams.Payload)
	}

	// Test GetLatestTabularData with invalid
	fmt.Println("\n=== Testing ExportTabularData ===")

	// Create a time interval for the last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-72 * time.Hour)

	captureInterval := app.CaptureInterval{
		Start: startTime,
		End:   endTime,
	}

	// Test ExportTabularData
	exportedData, err := dataClient.ExportTabularData(ctx, partID, resourceName, resourceSubtype, methodName, captureInterval, nil)
	if err != nil {
		log.Printf("Error exporting tabular data: %v", err)
	} else {
		fmt.Printf("Exported %d tabular data records:\n", len(exportedData))
		for i, record := range exportedData {
			fmt.Printf("  Record %d:\n", i+1)
			fmt.Printf("    Organization ID: %s\n", record.OrganizationID)
			fmt.Printf("    Location ID: %s\n", record.LocationID)
			fmt.Printf("    Robot ID: %s\n", record.RobotID)
			fmt.Printf("    Robot Name: %s\n", record.RobotName)
			fmt.Printf("    Part ID: %s\n", record.PartID)
			fmt.Printf("    Part Name: %s\n", record.PartName)
			fmt.Printf("    Resource Name: %s\n", record.ResourceName)
			fmt.Printf("    Resource Subtype: %s\n", record.ResourceSubtype)
			fmt.Printf("    Method Name: %s\n", record.MethodName)
			fmt.Printf("    Time Captured: %s\n", record.TimeCaptured.Format(time.RFC3339))
			fmt.Printf("    Method Parameters: %+v\n", record.MethodParameters)
			fmt.Printf("    Tags: %v\n", record.Tags)
			fmt.Printf("    Payload: %+v\n", record.Payload)
			fmt.Println()
		}
	}

	fmt.Println("=== Testing ExportTabularData with Additional Parameters ===")
	exportedDataWithParams, err := dataClient.ExportTabularData(ctx, partID, resourceName, resourceSubtype, methodName, captureInterval, additionalParams)
	if err != nil {
		log.Printf("Error exporting tabular data with parameters: %v", err)
	} else {
		fmt.Printf("Exported %d tabular data records with parameters:\n", len(exportedDataWithParams))
	}

	fmt.Println("\n=== Test completed ===")
}

func main() {
	fmt.Println("Starting Tabular Data API Test...")
	TestTabularDataFunctions()
}
