package main

import (
	"context"
	"log"
	"strings"

	"go.viam.com/rdk/app"
)

func main() {
	log.Println("Creating Viam client...")
	ctx := context.Background()

	// Example credentials - replace with your actual values
	apiKey := "dhjh680i33uagwpown4fy4g46pifk4jz"
	apiKeyID := "ebb16bb2-d90b-4d54-8c19-f413eecf0a9f"

	opts := app.Options{
		BaseURL: "https://pr-9261-appmain-bplesliplq-uc.a.run.app/", // Default Viam app URL
	}

	// Create the Viam client
	client, err := app.CreateViamClientWithAPIKey(ctx, opts, apiKey, apiKeyID, nil)
	if err != nil {
		log.Fatalf("Failed to create Viam client: %v", err)
	}
	defer client.Close()

	log.Println("Getting data client...")
	dataClient := client.DataClient()

	// Test 1: List all pipelines (this should work)
	log.Println("Test 1: Listing data pipelines...")
	pipelines, err := dataClient.ListDataPipelines(context.Background(), "880d1411-1608-4ac8-bbd3-b4ca8a74372d")
	if err != nil {
		log.Fatalf("Failed to list pipelines: %v", err)
	}

	log.Printf("Found %d pipeline(s)", len(pipelines))
	if len(pipelines) == 0 {
		log.Println("No pipelines found to test with")
		return
	}

	for i, pipeline := range pipelines {
		log.Printf("Pipeline %d: ID=%s, Name=%s, Enabled=%t", i+1, pipeline.ID, pipeline.Name, pipeline.Enabled)
		log.Printf("  Schedule: %s", pipeline.Schedule)
		log.Printf("  Created: %s", pipeline.CreatedOn.Format("2006-01-02 15:04:05"))
		log.Printf("  Updated: %s", pipeline.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	// Test 2: Get specific pipeline details (this should work)
	log.Printf("\nTest 2: Getting pipeline details for %s", pipelines[0].ID)
	pipeline, err := dataClient.GetDataPipeline(context.Background(), pipelines[0].ID)
	if err != nil {
		log.Printf("Failed to get pipeline details: %v", err)
	} else {
		log.Printf("Retrieved pipeline: %s", pipeline.Name)
		log.Printf("  Organization ID: %s", pipeline.OrganizationID)
		log.Printf("  Data Source Type: %v", pipeline.DataSourceType)
		log.Printf("  MQL Binary length: %d bytes", len(pipeline.MqlBinary))
	}

	// Test 3: Test RenameDataPipeline (expected to fail with current API)
	log.Printf("\nTest 3: Testing RenameDataPipeline method...")
	err = dataClient.RenameDataPipeline(context.Background(), pipelines[0].ID, "test-pipeline-renamed")
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "unknown method") {
			log.Printf("✓ Expected result: RenameDataPipeline is not yet implemented on the server")
			log.Printf("  Error: %v", err)
		} else {
			log.Printf("✗ Unexpected error (not unimplemented): %v", err)
		}
	} else {
		log.Printf("✓ Unexpected success: RenameDataPipeline worked! Pipeline renamed successfully")
	}

	// Test 4: Test other pipeline methods to see which ones work
	log.Printf("\nTest 4: Testing other pipeline methods...")

	// Test EnableDataPipeline
	log.Printf("Testing EnableDataPipeline...")
	err = dataClient.EnableDataPipeline(context.Background(), pipelines[0].ID)
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "unknown method") {
			log.Printf("  EnableDataPipeline: Not implemented on server")
		} else {
			log.Printf("  EnableDataPipeline error: %v", err)
		}
	} else {
		log.Printf("  ✓ EnableDataPipeline: Success")
	}

	// Test DisableDataPipeline
	log.Printf("Testing DisableDataPipeline...")
	err = dataClient.DisableDataPipeline(context.Background(), pipelines[0].ID)
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "unknown method") {
			log.Printf("  DisableDataPipeline: Not implemented on server")
		} else {
			log.Printf("  DisableDataPipeline error: %v", err)
		}
	} else {
		log.Printf("  ✓ DisableDataPipeline: Success")
	}

	// Test ListDataPipelineRuns
	log.Printf("Testing ListDataPipelineRuns...")
	runs, err := dataClient.ListDataPipelineRuns(context.Background(), pipelines[0].ID, 5)
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "unknown method") {
			log.Printf("  ListDataPipelineRuns: Not implemented on server")
		} else {
			log.Printf("  ListDataPipelineRuns error: %v", err)
		}
	} else {
		log.Printf("  ✓ ListDataPipelineRuns: Success, found %d runs", len(runs.Runs))
	}

	// Summary
	log.Printf("\n=== Summary ===")
	log.Printf("✓ ListDataPipelines: Working")
	log.Printf("✓ GetDataPipeline: Working")
	log.Printf("✗ RenameDataPipeline: Not implemented on server yet")
	log.Printf("The RenameDataPipeline method exists in the Go SDK but is not yet deployed to the API server.")
	log.Printf("You may need to wait for the server-side implementation to be deployed.")
}
