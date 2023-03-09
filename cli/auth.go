package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

type authFlow struct {
	clientID string
	scopes   []string
	audience string

	// OpenID Discovery endpoint: see https://openid.net/specs/openid-connect-discovery-1_0.html
	oidcDiscoveryEndpoint string

	disableBrowserOpen bool

	httpClient *http.Client
	logger     golog.Logger
	console    io.Writer
}

const (
	defaultOpenIDDiscoveryPath = "/.well-known/openid-configuration"

	prodAuthDomain = "https://auth.viam.com"
	prodAudience   = "https://app.viam.com/"
	prodClientID   = "HysEkkRKn6cDr2W6UFI6UYJHpiVwXFCk" // cli client

	stgAuthDomain = "https://auth.viam.dev"
	stgAudience   = "https://app.viam.dev/"
	stgClientID   = "o75PSAO21337n6SE0IV2BF9Aj9Er9NF6" // cli client

	defaultWaitInterval = time.Second * 1

	//nolint:gosec
	tokenTypeUserOAuthToken = "user-oauth-token"
)

var errAuthorizationPending = errors.New("authorization pending on user")

type openIDDiscoveryResponse struct {
	TokenEndPoint               string   `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
	UserinfoEndpoint            string   `json:"userinfo_endpoint"`
	ScopesSupported             []string `json:"scopes_supported"`
}

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	VerificationURIComplete string `json:"verification_uri_complete"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Token contains an authorization token and details once logged in.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	TokenURL     string    `json:"token_url"`
	ClientID     string    `json:"client_id"`

	User UserData `json:"user_data"`
}

// IsExpired returns true if the token is expired.
func (t *Token) IsExpired() bool {
	return t.ExpiresAt.Before(time.Now().Add(10 * time.Second))
}

// CanRefresh returns true if the token can be refreshed.
func (t *Token) CanRefresh() bool {
	return t.RefreshToken != "" && t.TokenURL != "" && t.ClientID != ""
}

// UserData user details from login.
type UserData struct {
	jwt.Claims

	Email   string `json:"email"`
	Subject string `json:"sub"` // userID
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func newCLIAuthFlow(console io.Writer) *authFlow {
	return newCLIAuthFlowWithAuthDomain(prodAuthDomain, prodAudience, prodClientID, console)
}

func newStgCLIAuthFlow(console io.Writer) *authFlow {
	return newCLIAuthFlowWithAuthDomain(stgAuthDomain, stgAudience, stgClientID, console)
}

func newCLIAuthFlowWithAuthDomain(authDomain, audience, clientID string, console io.Writer) *authFlow {
	return &authFlow{
		clientID:              clientID,
		scopes:                []string{"email", "openid", "offline_access"},
		audience:              audience,
		oidcDiscoveryEndpoint: fmt.Sprintf("%s%s", authDomain, defaultOpenIDDiscoveryPath),

		httpClient: &http.Client{Timeout: time.Second * 30},
		logger:     golog.Global(),
		console:    console,
	}
}

func (a *authFlow) Login(ctx context.Context) (*Token, error) {
	discovery, err := a.loadOIDiscoveryEndpoint(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed retrieving discovery endpoint")
	}

	deviceCode, err := a.makeDeviceCodeRequest(ctx, discovery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed return device code")
	}

	err = a.directUser(deviceCode)
	if err != nil {
		return nil, err
	}

	token, err := a.waitForUser(ctx, deviceCode, discovery)
	if err != nil {
		return nil, err
	}
	return buildToken(token, discovery.TokenEndPoint, a.clientID)
}

func buildToken(token *tokenResponse, tokenURL, clientID string) (*Token, error) {
	userData, err := userDataFromIDToken(token.IDToken)
	if err != nil {
		return nil, err
	}

	return &Token{
		TokenType:    tokenTypeUserOAuthToken,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      token.IDToken,
		ExpiresAt:    time.Now().Add(time.Second * time.Duration(token.ExpiresIn)),
		User:         *userData,
		TokenURL:     tokenURL,
		ClientID:     clientID,
	}, nil
}

func (a *authFlow) makeDeviceCodeRequest(ctx context.Context, discovery *openIDDiscoveryResponse) (*deviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", a.clientID)
	data.Set("audience", a.audience)
	data.Set("scope", strings.Join(a.scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.DeviceAuthorizationEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	//nolint:bodyclose
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(res.Body.Close)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code %d %s", res.StatusCode, body)
	}

	var resp deviceCodeResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (a *authFlow) directUser(code *deviceCodeResponse) error {
	fmt.Fprintf(a.console, "To authorize this device, visit:\n\t%s\n", code.VerificationURIComplete)

	if a.disableBrowserOpen {
		return nil
	}

	return openbrowser(code.VerificationURIComplete)
}

func (a *authFlow) waitForUser(ctx context.Context, code *deviceCodeResponse, discovery *openIDDiscoveryResponse) (*tokenResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(code.ExpiresIn*int(time.Second)))
	defer cancel()

	waitInterval := defaultWaitInterval
	for {
		if !utils.SelectContextOrWait(ctxWithTimeout, waitInterval) {
			return nil, errors.New("timed out getting token	")
		}

		data := url.Values{}
		data.Set("client_id", a.clientID)
		data.Add("device_code", code.DeviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.TokenEndPoint, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

		//nolint:bodyclose // processTokenResponse() closes it
		res, err := a.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		resp, err := processTokenResponse(res)
		if err != nil && !errors.Is(err, errAuthorizationPending) {
			return nil, err
		} else if err == nil {
			return resp, nil
		}

		waitInterval = time.Duration(code.Interval * int(time.Second))
	}
}

func (a *authFlow) loadOIDiscoveryEndpoint(ctx context.Context) (*openIDDiscoveryResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.oidcDiscoveryEndpoint, nil)
	if err != nil {
		return nil, err
	}

	//nolint:bodyclose
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(res.Body.Close)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	resp := openIDDiscoveryResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func openbrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = errors.New("unsupported platform")
	}
	return err
}

func userDataFromIDToken(token string) (*UserData, error) {
	userData := UserData{}
	jwtParser := jwt.NewParser()

	// We assume the ID token returned form the authorization endpoint is going to give
	// us a valid ID Token to get a email from. We already trust the AccessToken returned
	// from the call.
	_, _, err := jwtParser.ParseUnverified(token, &userData)
	if err != nil {
		return nil, err
	}

	if userData.Email == "" {
		return nil, errors.New("missing email in id_token claims")
	}

	if userData.Subject == "" {
		return nil, errors.New("missing sub in id_token claims")
	}

	return &userData, nil
}

func (a *authFlow) Refresh(ctx context.Context, token *Token) (*Token, error) {
	return refreshToken(ctx, a.httpClient, token)
}

func refreshToken(ctx context.Context, httpClient *http.Client, token *Token) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", token.ClientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", token.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, token.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	//nolint:bodyclose // processTokenResponse() closes it
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	resp, err := processTokenResponse(res)
	if err != nil {
		return nil, err
	} else if resp == nil {
		return nil, errors.New("expecting new token")
	}

	return buildToken(resp, token.TokenURL, token.ClientID)
}

func processTokenResponse(res *http.Response) (*tokenResponse, error) {
	defer utils.UncheckedErrorFunc(res.Body.Close)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		resp := tokenErrorResponse{}
		err = json.Unmarshal(body, &resp)
		if err != nil {
			return nil, err
		}

		if resp.Error == "authorization_pending" {
			return nil, errAuthorizationPending
		}

		return nil, fmt.Errorf("%s: %s", resp.Error, resp.ErrorDescription)
	}

	resp := tokenResponse{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	if resp.AccessToken == "" {
		return nil, errors.New("missing access_token in response")
	}

	if resp.IDToken == "" {
		return nil, errors.New("missing id_token in response")
	}

	if resp.RefreshToken == "" {
		return nil, errors.New("missing refresh_token in response")
	}

	return &resp, nil
}
