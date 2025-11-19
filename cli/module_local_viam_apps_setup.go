package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

// localAppTestingArgs contains the arguments for the local-app-testing command.
type localAppTestingArgs struct {
	AppURL    string `json:"app-url"`
	MachineID string `json:"machine-id"`
}

type localAppTestingServer struct {
	machineID       string
	machineHostname string
	machineAPIKey   string
	machineAPIKeyID string
	serverURL       string
	logger          io.Writer
}

// LocalAppTestingAction is the action for the local-app-testing command.
func LocalAppTestingAction(ctx *cli.Context, args localAppTestingArgs) error {
	serverPort := 8012
	viamClient, err := newViamClient(ctx)
	if err != nil {
		printf(ctx.App.ErrWriter, "error initializing the Viam client: "+err.Error())
		return err
	}

	localAppTesting := localAppTestingServer{
		serverURL: fmt.Sprintf("http://localhost:%d", serverPort),
		logger:    ctx.App.Writer,
	}

	printf(ctx.App.Writer, "Starting server to locally test viam apps on %s", localAppTesting.serverURL)
	printf(ctx.App.Writer, "Proxying local app from: %s", args.AppURL)
	printf(ctx.App.Writer, "Press Ctrl+C to stop the server")

	var httpServer *http.Server

	// Single-machine Viam app
	if args.MachineID != "" {
		printf(ctx.App.Writer, "Local testing for a single-machine Viam app, machine ID: %s", args.MachineID)

		machineAPIKeyID, machineAPIKey, err := getMachineAPIKeys(ctx.Context, viamClient.client, args.MachineID)
		if err != nil {
			return err
		}

		machineHostname, err := getMachineHostname(ctx.Context, viamClient.client, args.MachineID)
		if err != nil {
			return err
		}

		localAppTesting.machineID = args.MachineID
		localAppTesting.machineHostname = machineHostname
		localAppTesting.machineAPIKey = machineAPIKey
		localAppTesting.machineAPIKeyID = machineAPIKeyID

		httpServer = localAppTesting.setupHTTPServerSingleMachineApp(serverPort, args.AppURL)
	} else {
		// Multi machine Viam app
		printf(ctx.App.Writer, "Local testing for a multi-machine Viam app")

		currentToken, found := viamClient.conf.Auth.(*token)
		if !found || currentToken.AccessToken == "" {
			printf(ctx.App.ErrWriter, "You need an access token configured in the CLI to proceed. "+
				"Run the `viam login` command to re-authenticate, do NOT use an API key")
		}

		httpServer = localAppTesting.setupHTTPServerMultiMachineApp(serverPort, args.AppURL, currentToken.AccessToken)
	}

	if err := startServerInBackground(httpServer, ctx.App.Writer); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	if err := openbrowser(fmt.Sprintf("%s/start", localAppTesting.serverURL)); err != nil {
		printf(ctx.App.Writer, "Warning: Could not open browser: %v", err)
	}

	notifyCtx, _ := signal.NotifyContext(ctx.Context, os.Interrupt, syscall.SIGTERM)

	<-notifyCtx.Done()

	if err := httpServer.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}

func getMachineAPIKeys(ctx context.Context, viamAppClient apppb.AppServiceClient, machineID string) (string, string, error) {
	resp, err := viamAppClient.GetRobotAPIKeys(ctx, &apppb.GetRobotAPIKeysRequest{
		RobotId: machineID,
	})
	if err != nil {
		return "", "", err
	}

	keys := resp.GetApiKeys()
	if len(keys) == 0 {
		return "", "", errors.Errorf("Machine %s has no API keys", machineID)
	}

	return keys[0].GetApiKey().GetId(), keys[0].GetApiKey().GetKey(), nil
}

func getMachineHostname(ctx context.Context, viamAppClient apppb.AppServiceClient, machineID string) (string, error) {
	resp, err := viamAppClient.GetRobotParts(ctx, &apppb.GetRobotPartsRequest{
		RobotId: machineID,
	})
	if err != nil {
		return "", err
	}

	robotParts := resp.GetParts()
	for _, robotPart := range robotParts {
		if robotPart.MainPart {
			return robotPart.Fqdn, nil
		}
	}

	return "", errors.New("Could not resolve machine hostname, no main part found")
}

// setupHTTPServerSingleMachineApp creates and configures an HTTP server for a single-machine Viam app.
func (l *localAppTestingServer) setupHTTPServerSingleMachineApp(port int, targetURL string) *http.Server {
	// Endpoint to start the flow
	http.HandleFunc("/start", l.singleMachineCookieSetup)

	// Proxy setup
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		printf(l.logger, "Error parsing target URL: %v", err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURLParsed)

	// Modify the director to properly handle the /machine/ prefix and machine IDs
	proxy.Director = removeMachinePathFromURL(proxy.Director)

	// Add response interceptor
	//nolint: bodyclose
	proxy.ModifyResponse = l.addBaseTagToHTMLResponse()

	http.Handle("/", proxy)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: time.Minute * 5,
	}
}

type machineAuthCookieValue struct {
	Hostname    string             `json:"hostname"`
	MachineID   string             `json:"machineId"`
	Credentials machineCredentials `json:"credentials"`
	APIKey      machineAPIKey      `json:"apiKey"`
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

func (l *localAppTestingServer) singleMachineCookieSetup(resp http.ResponseWriter, req *http.Request) {
	// Generate machine auth cookie
	cookieValue := machineAuthCookieValue{
		Hostname:  l.machineHostname,
		MachineID: l.machineID,
		Credentials: machineCredentials{
			Type:       "api-key",
			Payload:    l.machineAPIKey,
			AuthEntity: l.machineAPIKeyID,
		},
		APIKey: machineAPIKey{
			Key: l.machineAPIKey,
			ID:  l.machineAPIKeyID,
		},
	}

	cookieValueBytes, err := json.Marshal(cookieValue)
	if err != nil {
		printf(l.logger, err.Error())
	}
	cookieValueString := url.QueryEscape(string(cookieValueBytes))

	// Add cookies
	http.SetCookie(resp, &http.Cookie{
		Name:  l.machineID,
		Value: cookieValueString,
	})

	http.SetCookie(resp, &http.Cookie{
		Name:  l.machineHostname,
		Value: cookieValueString,
	})

	// redirect to the machine path
	http.Redirect(resp, req, fmt.Sprintf("%s/machine/%s", l.serverURL, l.machineHostname), http.StatusFound)
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

func (l *localAppTestingServer) addBaseTagToHTMLResponse() func(resp *http.Response) error {
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
			if err := resp.Body.Close(); err != nil {
				printf(l.logger, "Error closing response body: %v", err)
			}

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
			} else {
				resp.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		return nil
	}
}

// setupHTTPServerMultiMachineApp creates and configures an HTTP server for a multi-machine Viam app.
func (l *localAppTestingServer) setupHTTPServerMultiMachineApp(port int, targetURL, accessToken string) *http.Server {
	// Endpoint to start the flow
	http.HandleFunc("/start", l.multiMachineCookieSetup(accessToken))

	// Proxy setup
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		printf(l.logger, "Error parsing target URL: %v", err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURLParsed)

	http.Handle("/", proxy)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: time.Minute * 5,
	}
}

type userTokenCookie struct {
	AccesToken string `json:"access_token"`
}

func (l *localAppTestingServer) multiMachineCookieSetup(accessToken string) func(http.ResponseWriter, *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		// Generate user token cookie
		cookieValue := userTokenCookie{
			AccesToken: accessToken,
		}

		cookieValueBytes, err := json.Marshal(cookieValue)
		if err != nil {
			printf(l.logger, err.Error())
		}
		cookieValueString := url.QueryEscape(string(cookieValueBytes))

		// Add cookie
		http.SetCookie(resp, &http.Cookie{
			Name:  "userToken",
			Value: cookieValueString,
		})

		// redirect to the selected path
		http.Redirect(resp, req, "/", http.StatusFound)
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
