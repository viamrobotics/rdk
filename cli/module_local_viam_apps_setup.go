package cli

import (
	"context"
	"encoding/json"
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
	AppURL          string `json:"app-url"`
	MachineID       string `json:"machine-id"`
	MachineApiKey   string `json:"machine-api-key"`
	MachineApiKeyID string `json:"machine-api-key-id"`
}

type localAppTestingServer struct {
	MachineID       string
	MachineApiKey   string
	MachineApiKeyID string
	ServerURL       string
	Logger          io.Writer
}

// LocalAppTestingAction is the action for the local-app-testing command.
func LocalAppTestingAction(ctx *cli.Context, args localAppTestingArgs) error {
	serverPort := 8000
	localAppTesting := localAppTestingServer{
		MachineID:       args.MachineID,
		MachineApiKey:   args.MachineApiKey,
		MachineApiKeyID: args.MachineApiKeyID,
		ServerURL:       fmt.Sprintf("http://localhost:%d", serverPort),
		Logger:          ctx.App.Writer,
	}

	httpServer := localAppTesting.setupHTTPServer(serverPort, args.AppURL)

	printf(ctx.App.Writer, "Starting server to locally test viam apps on %s", localAppTesting.ServerURL)
	printf(ctx.App.Writer, "Proxying local app from: %s", args.AppURL)
	printf(ctx.App.Writer, "Press Ctrl+C to stop the server")

	if err := startServerInBackground(httpServer, ctx.App.Writer); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	if err := openbrowser(fmt.Sprintf("%s/start", localAppTesting.ServerURL)); err != nil {
		printf(ctx.App.Writer, "Warning: Could not open browser: %v", err)
	}

	<-ctx.Context.Done()

	if err := httpServer.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}

// setupHTTPServer creates and configures an HTTP server with the given HTML file.
func (l *localAppTestingServer) setupHTTPServer(port int, targetURL string) *http.Server {
	// Endpoint to start the flow
	http.HandleFunc("/start", l.cookieSetup)

	// Proxy setup
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		printf(l.Logger, "Error parsing target URL: %v", err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURLParsed)

	// Modify the director to properly handle the /machine/ prefix and machine IDs
	proxy.Director = removeMachinePathFromURL(proxy.Director)

	// Add response interceptor
	proxy.ModifyResponse = addBaseTagToResponseBody(l.Logger)

	http.Handle("/", proxy)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: time.Minute * 5,
	}
}

type machineAuthcCokieValue struct {
	Hostname    string             `json:"hostname"`
	MachineID   string             `json:"machineId"`
	Credentials machineCredentials `json:"credentials"`
	ApiKey      machineAPIKey      `json:"apiKey"`
}

type machineCredentials struct {
	Type       string `json:"type"`
	Payload    string `json:"payload"`
	AuthEntity string `json:"authEntity"`
}

type machineAPIKey struct {
	Key string `json:"key"`
	ID  string `json:"id"`
}

func (l *localAppTestingServer) cookieSetup(resp http.ResponseWriter, req *http.Request) {
	// Generate machine auth cookie
	// TODO
	machineHostname := "TODO"

	cookieValue := machineAuthcCokieValue{
		Hostname:  machineHostname,
		MachineID: l.MachineID,
		Credentials: machineCredentials{
			Type:       "api-key",
			Payload:    l.MachineApiKey,
			AuthEntity: l.MachineApiKeyID,
		},
		ApiKey: machineAPIKey{
			Key: l.MachineApiKey,
			ID:  l.MachineApiKeyID,
		},
	}

	cookieValueBytes, err := json.Marshal(cookieValue)
	if err != nil {
		printf(l.Logger, err.Error())
	}
	cookieValueString := url.QueryEscape(string(cookieValueBytes))

	// Add cookies
	http.SetCookie(resp, &http.Cookie{
		Name:  l.MachineID,
		Value: cookieValueString,
	})

	http.SetCookie(resp, &http.Cookie{
		Name:  machineHostname,
		Value: cookieValueString,
	})

	// redirect to the machine path
	http.Redirect(resp, req, fmt.Sprintf("%s/machine/%s", l.ServerURL, l.MachineID), http.StatusFound)
}

func removeMachinePathFromURL(originalDirector func(*http.Request)) func(*http.Request) {
	return func(req *http.Request) {
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
}

func addBaseTagToResponseBody(writer io.Writer) func(resp *http.Response) error {
	return func(resp *http.Response) error {
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

			if strings.Contains(originalPath, "/machine") {
				// take first 3 components if available ([1 empty]/[2 machine]/[3 machine ID]/....)
				baseHrefComponents := strings.Split(originalPath, "/")[:3]
				baseHref := fmt.Sprintf("%s/", strings.Join(baseHrefComponents, "/"))

				// inject a <base> tag into the <head> to ensure all relative URLs resolve correctly
				// under the current machine-specific path (e.g., /machine/:machineId/).
				// The regex (?i)<head[^>]*> matches the opening <head> tag (case-insensitively),
				// allowing for optional attributes like <head lang="en">
				re := regexp.MustCompile(`(?i)<head[^>]*>`)
				newBody := re.ReplaceAllStringFunc(string(body), func(match string) string {
					// Use a more robust base href that works for different types of relative URLs
					// For machine-specific paths, we want to ensure the base includes the full machine path

					baseTag := fmt.Sprintf(`<base href="%s">`, baseHref)
					return match + "\n" + baseTag
				})

				resp.Body = io.NopCloser(strings.NewReader(newBody))
				resp.ContentLength = int64(len(newBody))
				resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
			}
		}
		return nil
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
	case <-time.After(1000 * time.Millisecond):
		return nil // Server started successfully
	}
}
