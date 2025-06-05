package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/urfave/cli/v2"
)

// LocalAppTestingAction is the action for the local-app-testing command.
func LocalAppTestingAction(ctx *cli.Context, args emptyArgs) error {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Get the directory of the current source file
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("error getting current file path")
	}
	sourceDir := filepath.Dir(currentFile)

	// Get the absolute path to local_viam_apps_test.html in the source directory
	htmlPath := filepath.Join(sourceDir, "local_viam_apps_test.html")
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("error getting absolute path: %w", err)
	}

	// Verify the file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("local_viam_apps_test.html not found at: %s", absPath)
	}

	// Only serve local_viam_apps_test.html for any path
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set headers to prevent caching
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		// Serve the file
		http.ServeFile(w, r, absPath)
	})

	// Start the server
	port := ":8000"
	serverURL := fmt.Sprintf("http://localhost%s", port)
	logger.Printf("Starting server to locally test viam apps on %s", serverURL)
	logger.Println("Press Ctrl+C to stop the server")

	// Start server in a goroutine so we can handle context cancellation
	server := &http.Server{
		Addr:              port,
		ReadHeaderTimeout: time.Minute * 5,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("Error starting server: %v", err)
		}
	}()

	// Open the browser
	if err := openbrowser(serverURL); err != nil {
		logger.Printf("Warning: Could not open browser: %v", err)
	}

	// Wait for context cancellation
	<-ctx.Context.Done()

	// Gracefully shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}
