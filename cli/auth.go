package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/term"

	"go.viam.com/rdk/logging"
)

type authFlow struct {
	clientID string
	scopes   []string
	audience string

	// OpenID Discovery endpoint: see https://openid.net/specs/openid-connect-discovery-1_0.html
	oidcDiscoveryEndpoint string

	disableBrowserOpen bool

	httpClient *http.Client
	logger     logging.Logger
	console    io.Writer
}

const (
	defaultOpenIDDiscoveryPath = "/.well-known/openid-configuration"

	prodAuthDomain = "https://viam-prod.fusionauth.io"
	prodAudience   = "c1e41724-9b29-479f-abcc-7bfbe2e3309a"
	prodClientID   = "c1e41724-9b29-479f-abcc-7bfbe2e3309a" // native client ID

	stgAuthDomain = "https://viam.fusionauth.io"
	stgAudience   = "d7eb6419-301c-4ef1-a5e5-8e8bf28a87c0"
	stgClientID   = "d7eb6419-301c-4ef1-a5e5-8e8bf28a87c0" // native client ID

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

type authMethod interface {
	fmt.Stringer
	dialOpts() rpc.DialOption
}

var (
	_ authMethod = (*token)(nil)  // Verify that *token implements authMethod.
	_ authMethod = (*apiKey)(nil) // Verify that *apiKey implements authMethod.
)

// token contains an authorization token and details once logged in.
type token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	TokenURL     string    `json:"token_url"`
	ClientID     string    `json:"client_id"`

	User userData `json:"user_data"`
}

// apiKey holds an id/value pair used to authenticate with app.viam.
type apiKey struct {
	KeyID     string `json:"key_id"`
	KeyCrypto string `json:"key_crypto"`
}

// LoginAction is the corresponding Action for 'login'.
func LoginAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.loginAction(cCtx)
}

func (c *viamClient) loginAction(cCtx *cli.Context) error {
	loggedInMessage := func(t *token, alreadyLoggedIn bool) {
		already := "Already l"
		if !alreadyLoggedIn {
			already = "L"
			// only print the viam logo if we are in an interative terminal
			if term.IsTerminal(int(os.Stdout.Fd())) {
				viamLogo(cCtx.App.Writer)
			}
		}

		printf(cCtx.App.Writer, "%sogged in as %q, expires %s", already, t.User.Email,
			t.ExpiresAt.Format("Mon Jan 2 15:04:05 MST 2006"))
	}

	if _, isAPIKey := c.conf.Auth.(*apiKey); isAPIKey {
		warningf(c.c.App.ErrWriter, "was logged in with an api-key. logging out")
		utils.UncheckedError(c.logout())
	}
	currentToken, _ := c.conf.Auth.(*token) // currentToken can be nil
	if currentToken != nil && !currentToken.isExpired() {
		loggedInMessage(currentToken, true)
		return nil
	}

	var t *token
	var err error
	if currentToken != nil && currentToken.canRefresh() {
		t, err = c.authFlow.refreshToken(c.c.Context, currentToken)
		if err != nil {
			utils.UncheckedError(c.logout())
			return err
		}
	} else {
		t, err = c.authFlow.loginAsUser(c.c.Context)
		if err != nil {
			return err
		}
	}

	// write token to config.
	c.conf.Auth = t
	if err := storeConfigToCache(c.conf); err != nil {
		return err
	}

	loggedInMessage(t, false)
	return nil
}

// LoginWithAPIKeyAction is the corresponding Action for `login api-key`.
func LoginWithAPIKeyAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.loginWithAPIKeyAction(cCtx)
}

func (c viamClient) loginWithAPIKeyAction(cCtx *cli.Context) error {
	key := apiKey{
		KeyID:     cCtx.String(loginFlagKeyID),
		KeyCrypto: cCtx.String(loginFlagKey),
	}
	c.conf.Auth = &key
	if err := storeConfigToCache(c.conf); err != nil {
		return err
	}
	// test the connection
	if _, err := c.listOrganizations(); err != nil {
		return errors.Wrapf(err, "unable to connect to %q using the provided api key", c.conf.BaseURL)
	}
	printf(cCtx.App.Writer, "Successfully logged in with api key %q", key.KeyID)
	return nil
}

// PrintAccessTokenAction is the corresponding Action for 'print-access-token'.
func PrintAccessTokenAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.printAccessTokenAction(cCtx)
}

func (c *viamClient) printAccessTokenAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if token, ok := c.conf.Auth.(*token); ok {
		printf(cCtx.App.Writer, token.AccessToken)
	} else {
		return errors.New("not logged in as a user. Cannot print access token. Run \"viam login\" to sign in with your account")
	}
	return nil
}

// LogoutAction is the corresponding Action for 'logout'.
func LogoutAction(cCtx *cli.Context) error {
	// Create basic viam client; no need to check base URL.
	conf, err := ConfigFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		conf = &Config{}
	}

	vc := &viamClient{
		c:    cCtx,
		conf: conf,
	}
	return vc.logoutAction(cCtx)
}

func (c *viamClient) logoutAction(cCtx *cli.Context) error {
	auth := c.conf.Auth
	if auth == nil {
		printf(cCtx.App.Writer, "Already logged out")
		return nil
	}
	if err := c.logout(); err != nil {
		return errors.Wrap(err, "could not logout")
	}
	printf(cCtx.App.Writer, "Logged out from %q", auth)
	return nil
}

// WhoAmIAction is the corresponding Action for 'whoami'.
func WhoAmIAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.whoAmIAction(cCtx)
}

func (c *viamClient) whoAmIAction(cCtx *cli.Context) error {
	auth := c.conf.Auth
	if auth == nil {
		warningf(cCtx.App.Writer, "Not logged in. Run \"login\" command")
		return nil
	}
	printf(cCtx.App.Writer, "%s", auth)
	return nil
}

func (c *viamClient) generateDefaultKeyName() string {
	// Default name is in the form myusername@gmail.com-2009-11-10T23:00:00Z
	// or key-uuid-2009-11-10T23:00:00Z if it was created by a key
	return fmt.Sprintf("%s-%s", c.conf.Auth, time.Now().Format(time.RFC3339))
}

// OrganizationsAPIKeyCreateAction corresponds to `organizations api-key create`.
func OrganizationsAPIKeyCreateAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.organizationsAPIKeyCreateAction(cCtx)
}

func (c *viamClient) organizationsAPIKeyCreateAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	orgID := cCtx.String(generalFlagOrgID)
	keyName := cCtx.String(apiKeyCreateFlagName)
	if keyName == "" {
		keyName = c.generateDefaultKeyName()
		infof(cCtx.App.Writer, "using default key name of %q", keyName)
	}
	resp, err := c.createOrganizationAPIKey(orgID, keyName)
	if err != nil {
		return err
	}
	infof(cCtx.App.Writer, "Successfully created key:")
	printf(cCtx.App.Writer, "Key ID: %s ", resp.GetId())
	printf(cCtx.App.Writer, "Key Value: %s", resp.GetKey())
	warningf(cCtx.App.Writer, "Keep this key somewhere safe; it has full write access to your organization")
	return nil
}

func (c *viamClient) createOrganizationAPIKey(orgID, keyName string) (*apppb.CreateKeyResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	req := &apppb.CreateKeyRequest{
		Authorizations: []*apppb.Authorization{
			{
				AuthorizationType: "role",
				AuthorizationId:   "organization_owner",
				ResourceType:      "organization",
				ResourceId:        orgID,
				IdentityId:        "",
				OrganizationId:    orgID,
				IdentityType:      "api-key",
			},
		},
		Name: keyName,
	}
	return c.client.CreateKey(c.c.Context, req)
}

// LocationAPIKeyCreateAction corresponds to `location api-key create`.
func LocationAPIKeyCreateAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	err = c.locationAPIKeyCreateAction(cCtx)
	return err
}

func (c *viamClient) locationAPIKeyCreateAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	locationID := cCtx.String(generalFlagLocationID)
	orgID := cCtx.String(generalFlagOrgID)
	keyName := cCtx.String(apiKeyCreateFlagName)

	if locationID == "" {
		return errors.New("cannot create an api-key for a location without an ID")
	}

	if keyName == "" {
		keyName = c.generateDefaultKeyName()
		infof(cCtx.App.Writer, "using default key name of %s", keyName)
	}

	req := &apppb.CreateKeyRequest{
		Name: keyName,
		Authorizations: []*apppb.Authorization{
			{
				AuthorizationType: "role",
				AuthorizationId:   "location_owner",
				ResourceType:      "location",
				ResourceId:        locationID,
				OrganizationId:    orgID,
				IdentityType:      "api-key",
				IdentityId:        "",
			},
		},
	}

	key, err := c.client.CreateKey(c.c.Context, req)
	if err != nil {
		if strings.Contains(err.Error(), "multiple orgs") {
			return errors.Errorf("cannot create api-key for location: %s as there are multiple orgs on the location. "+
				"Please re-run the command with an organization-id flag set", locationID)
		}
		return err
	}

	infof(cCtx.App.Writer, "Successfully created key: ")
	printf(cCtx.App.Writer, "Key ID: %s", key.GetId())
	printf(cCtx.App.Writer, "Key Value: %s", key.GetKey())
	warningf(cCtx.App.Writer, "Keep this key somewhere safe; it has full write access to your location")
	return nil
}

// RobotAPIKeyCreateAction corresponds to `machine api-key create`.
func RobotAPIKeyCreateAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	err = c.robotAPIKeyCreateAction(cCtx)
	return err
}

func (c *viamClient) robotAPIKeyCreateAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	robotID := cCtx.String(generalFlagMachineID)
	keyName := cCtx.String(apiKeyCreateFlagName)
	orgID := cCtx.String(generalFlagOrgID)

	if robotID == "" {
		return errors.New("cannot create an api-key for a machine without an ID")
	}

	if keyName == "" {
		keyName = c.generateDefaultKeyName()
		infof(cCtx.App.Writer, "using default key name of %q", keyName)
	}

	// If we pass in an empty OrgID the CreateAPIKey endpoint
	// will try and tie it to the default org on the location tied to the robot
	// This only works if there is a single org on the robot

	req := &apppb.CreateKeyRequest{
		Name: keyName,
		Authorizations: []*apppb.Authorization{
			{
				AuthorizationType: "role",
				AuthorizationId:   "robot_owner",
				ResourceType:      "robot",
				ResourceId:        robotID,
				OrganizationId:    orgID,
				IdentityType:      "api-key",
				IdentityId:        "",
			},
		},
	}

	key, err := c.client.CreateKey(c.c.Context, req)
	if err != nil {
		if strings.Contains(err.Error(), "multiple orgs") {
			return errors.New("cannot create the machine api-key as there are multiple orgs on the location. " +
				"Please re-run the command with an organization-id flag set")
		}
		return err
	}
	infof(cCtx.App.Writer, "Successfully created key:")
	printf(cCtx.App.Writer, "Key ID: %s", key.GetId())
	printf(cCtx.App.Writer, "Key Value: %s", key.GetKey())
	warningf(cCtx.App.Writer, "Keep this key somewhere safe; it has full write access to your machine")

	return nil
}

func (c *viamClient) ensureLoggedIn() error {
	if c.client != nil {
		return nil
	}

	if c.conf.Auth == nil {
		return errors.New("not logged in: run the following command to login:\n\tviam login")
	}

	authToken, ok := c.conf.Auth.(*token)
	if ok && authToken.isExpired() {
		if !authToken.canRefresh() {
			utils.UncheckedError(c.logout())
			return errors.New("token expired and cannot refresh")
		}

		// expired.
		newToken, err := c.authFlow.refreshToken(c.c.Context, authToken)
		if err != nil {
			utils.UncheckedError(c.logout()) // clear cache if failed to refresh
			return errors.Wrapf(err, "error while refreshing token")
		}

		// write token to config.
		c.conf.Auth = newToken
		if err := storeConfigToCache(c.conf); err != nil {
			return err
		}
	}

	rpcOpts, err := c.conf.DialOptions()
	if err != nil {
		return err
	}

	conn, err := rpc.DialDirectGRPC(
		c.c.Context,
		c.baseURL.Host,
		nil,
		rpcOpts...,
	)
	if err != nil {
		return err
	}

	c.client = apppb.NewAppServiceClient(conn)
	c.dataClient = datapb.NewDataServiceClient(conn)
	c.packageClient = packagepb.NewPackageServiceClient(conn)
	c.datasetClient = datasetpb.NewDatasetServiceClient(conn)
	c.endUserClient = apppb.NewEndUserServiceClient(conn)
	c.mlTrainingClient = mltrainingpb.NewMLTrainingServiceClient(conn)
	c.buildClient = buildpb.NewBuildServiceClient(conn)

	return nil
}

// logout logs out the client and clears the config.
func (c *viamClient) logout() error {
	if err := removeConfigFromCache(); err != nil && !os.IsNotExist(err) {
		return err
	}
	c.conf = &Config{}
	return nil
}

func (c *viamClient) prepareDial(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
) (context.Context, string, []rpc.DialOption, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, "", nil, err
	}
	if err := c.selectOrganization(orgStr); err != nil {
		return nil, "", nil, err
	}
	if err := c.selectLocation(locStr); err != nil {
		return nil, "", nil, err
	}

	part, err := c.robotPart(c.selectedOrg.Id, c.selectedLoc.Id, robotStr, partStr)
	if err != nil {
		return nil, "", nil, err
	}

	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		utils.UncheckedError(rpcDialer.Close())
	}()
	dialCtx := rpc.ContextWithDialer(c.c.Context, rpcDialer)

	rpcOpts, err := c.conf.DialOptions()
	if err != nil {
		return nil, "", nil, err
	}
	rpcOpts = append(rpcOpts, rpc.WithExternalAuth(c.baseURL.Host, part.Fqdn))

	if debug {
		rpcOpts = append(rpcOpts, rpc.WithDialDebug())
	}

	return dialCtx, part.Fqdn, rpcOpts, nil
}

func (t *token) isExpired() bool {
	return t.ExpiresAt.Before(time.Now().Add(10 * time.Second))
}

func (t *token) canRefresh() bool {
	return t.RefreshToken != "" && t.TokenURL != "" && t.ClientID != ""
}

func (t *token) dialOpts() rpc.DialOption {
	return rpc.WithStaticAuthenticationMaterial(t.AccessToken)
}

func (t *token) String() string {
	return t.User.Email
}

func (k *apiKey) dialOpts() rpc.DialOption {
	return rpc.WithEntityCredentials(
		k.KeyID,
		rpc.Credentials{
			Type:    "api-key",
			Payload: k.KeyCrypto,
		},
	)
}

func (k *apiKey) String() string {
	return fmt.Sprintf("key-%s", k.KeyID)
}

type userData struct {
	jwt.Claims

	Email   string `json:"email"`
	Subject string `json:"sub"` // userID
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func newCLIAuthFlow(console io.Writer, disableBrowserOpen bool) *authFlow {
	return newCLIAuthFlowWithAuthDomain(prodAuthDomain, prodAudience, prodClientID, console, disableBrowserOpen)
}

func newStgCLIAuthFlow(console io.Writer, disableBrowserOpen bool) *authFlow {
	return newCLIAuthFlowWithAuthDomain(stgAuthDomain, stgAudience, stgClientID, console, disableBrowserOpen)
}

func newCLIAuthFlowWithAuthDomain(authDomain, audience, clientID string, console io.Writer, disableBrowserOpen bool) *authFlow {
	return &authFlow{
		clientID:              clientID,
		scopes:                []string{"email", "openid", "offline_access"},
		audience:              audience,
		oidcDiscoveryEndpoint: fmt.Sprintf("%s%s", authDomain, defaultOpenIDDiscoveryPath),

		disableBrowserOpen: disableBrowserOpen,
		httpClient:         &http.Client{Timeout: time.Second * 30},
		logger:             logging.Global(),
		console:            console,
	}
}

func (a *authFlow) loginAsUser(ctx context.Context) (*token, error) {
	discovery, err := a.loadOIDiscoveryEndpoint(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed retrieving discovery endpoint")
	}

	deviceCode, err := a.makeDeviceCodeRequest(ctx, discovery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to return device code")
	}

	err = a.directUser(deviceCode)
	if err != nil {
		return nil, fmt.Errorf("unable to open the browser to complete the login flow due to %w."+
			"You can use the --%s flag to skip this behavior", err, loginFlagDisableBrowser)
	}

	token, err := a.waitForUser(ctx, deviceCode, discovery)
	if err != nil {
		return nil, err
	}
	return buildToken(token, discovery.TokenEndPoint, a.clientID)
}

func buildToken(t *tokenResponse, tokenURL, clientID string) (*token, error) {
	userData, err := userDataFromIDToken(t.IDToken)
	if err != nil {
		return nil, err
	}

	return &token{
		TokenType:    tokenTypeUserOAuthToken,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		IDToken:      t.IDToken,
		ExpiresAt:    time.Now().Add(time.Second * time.Duration(t.ExpiresIn)),
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
	infof(a.console, `You can log into Viam through the opened browser window or follow the URL below.
Ensure the code in the URL matches the one shown in your browser.
  %s`, code.VerificationURIComplete)

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
			return nil, fmt.Errorf("timed out getting token after %f seconds", waitInterval.Seconds())
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

func userDataFromIDToken(token string) (*userData, error) {
	userData := userData{}
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

func (a *authFlow) refreshToken(ctx context.Context, t *token) (*token, error) {
	data := url.Values{}
	data.Set("client_id", t.ClientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", t.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.TokenURL, strings.NewReader(data.Encode()))
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
	if err != nil {
		return nil, err
	} else if resp == nil {
		return nil, errors.New("expecting new token")
	}

	return buildToken(resp, t.TokenURL, t.ClientID)
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
