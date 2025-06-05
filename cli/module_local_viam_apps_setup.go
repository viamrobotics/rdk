package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/urfave/cli/v2"
)

// localAppTestingArgs contains the arguments for the local-app-testing command.
type localAppTestingArgs struct {
	Port int `json:"port"`
}

// LocalAppTestingAction is the action for the local-app-testing command.
func LocalAppTestingAction(ctx *cli.Context, args localAppTestingArgs) error {
	htmlPath, err := getHTMLFilePath()
	if err != nil {
		return err
	}

	server := setupHTTPServer(htmlPath, args.Port)
	serverURL := fmt.Sprintf("http://localhost:%d", args.Port)

	printf(ctx.App.Writer, "Starting server to locally test viam apps on %s", serverURL)
	printf(ctx.App.Writer, "Press Ctrl+C to stop the server")

	if err := startServerInBackground(server, ctx.App.Writer); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	if err := openbrowser(serverURL); err != nil {
		printf(ctx.App.Writer, "Warning: Could not open browser: %v", err)
	}

	<-ctx.Context.Done()

	if err := server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}

// getHTMLFilePath returns the absolute path to module_local_viam_apps_test.html.
func getHTMLFilePath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("error getting current file path")
	}
	sourceDir := filepath.Dir(currentFile)

	htmlPath := filepath.Join(sourceDir, "module_local_viam_apps_test.html")
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return "", fmt.Errorf("error getting absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("module_local_viam_apps_test.html not found at: %s", absPath)
	}

	return absPath, nil
}

// setupHTTPServer creates and configures an HTTP server with the given HTML file.
func setupHTTPServer(htmlPath string, port int) *http.Server {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		http.ServeFile(w, r, htmlPath)
	})

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: time.Minute * 5,
	}
}

// startServerInBackground starts the HTTP server in a goroutine and returns any startup errors.
func startServerInBackground(server *http.Server, writer io.Writer) error {
	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			printf(writer, "Error starting server: %v", err)
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil // Server started successfully
	}
}
