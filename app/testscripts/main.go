package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.viam.com/rdk/app"
)

func main() {
	// Test 1: Check basic connectivity to Viam
	log.Println("Test 1: Checking connectivity to app.viam.com...")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://app.viam.com/")
	if err != nil {
		log.Printf("❌ Cannot reach app.viam.com: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("✅ Successfully connected to app.viam.com (status: %d)", resp.StatusCode)

	// Test 2: Try creating Viam client with very short timeout
	log.Println("\nTest 2: Creating Viam client with 10-second timeout...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apiKey := "dhjh680i33uagwpown4fy4g46pifk4jz"
	apiKeyID := "ebb16bb2-d90b-4d54-8c19-f413eecf0a9f"

	opts := app.Options{
		BaseURL: "https://app.viam.com/",
	}

	// Add channel to detect if client creation hangs
	done := make(chan bool, 1)
	var viamClient *app.ViamClient
	var clientErr error

	go func() {
		viamClient, clientErr = app.CreateViamClientWithAPIKey(ctx, opts, apiKey, apiKeyID, nil)
		done <- true
	}()

	select {
	case <-done:
		if clientErr != nil {
			log.Printf("❌ Failed to create Viam client: %v", clientErr)
			return
		}
		log.Println("✅ Successfully created Viam client!")
		defer viamClient.Close()

		// Test 3: Try a simple API call
		log.Println("\nTest 3: Testing DataClient...")
		dataClient := viamClient.DataClient()

		// Try to list pipelines with timeout
		apiCtx, apiCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer apiCancel()

		pipelines, err := dataClient.ListDataPipelines(apiCtx, "880d1411-1608-4ac8-bbd3-b4ca8a74372d")
		if err != nil {
			log.Printf("❌ Failed to list pipelines: %v", err)
		} else {
			log.Printf("✅ Successfully listed %d pipelines", len(pipelines))
		}

	case <-time.After(15 * time.Second):
		log.Println("❌ Client creation timed out after 15 seconds")
		log.Println("This suggests a network connectivity issue or invalid credentials")
	}

	fmt.Println("\n=== Diagnostic Summary ===")
	fmt.Println("If client creation hangs:")
	fmt.Println("1. Check your network connection")
	fmt.Println("2. Verify your API key and API key ID are correct")
	fmt.Println("3. Check if you're behind a corporate firewall/proxy")
	fmt.Println("4. Try using a different network (e.g., mobile hotspot)")
}
