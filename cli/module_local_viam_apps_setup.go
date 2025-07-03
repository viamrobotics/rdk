package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

// localAppTestingArgs contains the arguments for the local-app-testing command.
type localAppTestingArgs struct {
	Port   int    `json:"port"`
	AppURL string `json:"app-url"`
}

// LocalAppTestingAction is the action for the local-app-testing command.
func LocalAppTestingAction(ctx *cli.Context, args localAppTestingArgs) error {
	server := setupHTTPServer(args.Port, args.AppURL, ctx.App.Writer)
	serverURL := fmt.Sprintf("http://localhost:%d", args.Port)

	printf(ctx.App.Writer, "Starting server to locally test viam apps on %s", serverURL)
	printf(ctx.App.Writer, "Proxying local app from: %s", args.AppURL)
	printf(ctx.App.Writer, "Press Ctrl+C to stop the server")

	if err := startServerInBackground(server, ctx.App.Writer); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	<-ctx.Context.Done()

	if err := server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}

// setupHTTPServer creates and configures an HTTP server with the given HTML file.
func setupHTTPServer(port int, targetURL string, writer io.Writer) *http.Server {
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		printf(writer, "Error parsing target URL: %v", err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURLParsed)

	// Modify the director to properly handle the /machine/ prefix and machine IDs
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Store the original path before modifying it
		originalPath := req.URL.Path

		// Strip cache validation headers to force fresh responses
		req.Header.Del("If-Modified-Since")
		req.Header.Del("If-None-Match")
		req.Header.Del("Cache-Control")

		// Handle the pattern /machine/{machineId}/... or /machine/{machineId}
		// Strip /machine and the machine ID, keeping the rest of the path
		pathParts := strings.SplitN(originalPath, "/", 4) // Split into max 4 parts
		if len(pathParts) >= 3 && pathParts[1] == "machine" {
			// We have /machine/{machineId}/... or /machine/{machineId}
			if len(pathParts) >= 4 {
				// /machine/{machineId}/rest-of-path
				req.URL.Path = "/" + pathParts[3]
			} else {
				// /machine/{machineId} - no additional path
				req.URL.Path = "/"
			}
		} else {
			// Fallback: just strip /machine prefix
			req.URL.Path = strings.TrimPrefix(originalPath, "/machine")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		originalDirector(req)
		// Store the original path in the request context for later use
		req.Header.Set("X-Original-Path", originalPath)
	}

	// Add response interceptor
	proxy.ModifyResponse = func(resp *http.Response) error {
		contentType := resp.Header.Get("Content-Type")
		isHTML := strings.Contains(strings.ToLower(contentType), "text/html")

		// Add cache-busting headers to prevent 304 responses
		resp.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resp.Header.Set("Pragma", "no-cache")
		resp.Header.Set("Expires", "0")

		if isHTML {
			// Get the original path from the request header
			originalPath := resp.Request.Header.Get("X-Original-Path")
			if originalPath == "" {
				originalPath = "/machine" // fallback
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("error reading response body: %w", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					printf(writer, "Error closing response body: %v", err)
				}
			}()

			// inject a <base> tag into the <head> to ensure all relative URLs resolve correctly
			// under the current machine-specific path (e.g., /machine/:machineId/).
			// The regex (?i)<head[^>]*> matches the opening <head> tag (case-insensitively),
			// allowing for optional attributes like <head lang="en">
			re := regexp.MustCompile(`(?i)<head[^>]*>`)
			newBody := re.ReplaceAllStringFunc(string(body), func(match string) string {
				// Use a more robust base href that works for different types of relative URLs
				// For machine-specific paths, we want to ensure the base includes the full machine path
				baseHref := originalPath
				if !strings.HasSuffix(baseHref, "/") {
					baseHref += "/"
				}

				baseTag := fmt.Sprintf(`<base href="%s">`, baseHref)
				return match + "\n" + baseTag
			})

			resp.Body = io.NopCloser(strings.NewReader(newBody))
			resp.ContentLength = int64(len(newBody))
			resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
		}

		return nil
	}

	http.Handle("/", proxy)

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
