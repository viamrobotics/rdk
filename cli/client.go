// Package cli contains all business logic needed by the CLI command.
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
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	datapb "go.viam.com/api/app/data/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
)

// The AppClient provides all the CLI command functionality needed to talk
// to the app service but not directly to robot parts.
type AppClient struct {
	c          *cli.Context
	conf       *Config
	client     apppb.AppServiceClient
	dataClient datapb.DataServiceClient
	baseURL    *url.URL
	rpcOpts    []rpc.DialOption
	authFlow   *authFlow

	selectedOrg *apppb.Organization
	selectedLoc *apppb.Location

	// caches
	orgs *[]*apppb.Organization
	locs *[]*apppb.Location
}

func checkBaseURL(c *cli.Context) (*url.URL, []rpc.DialOption, error) {
	baseURL := c.String("base-url")
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, nil, err
	}

	if baseURLParsed.Scheme == "https" {
		return baseURLParsed, nil, nil
	}
	return baseURLParsed, []rpc.DialOption{
		rpc.WithInsecure(),
		rpc.WithAllowInsecureWithCredentialsDowngrade(),
	}, nil
}

func isProdBaseURL(baseURL *url.URL) bool {
	return strings.HasSuffix(baseURL.Hostname(), "viam.com")
}

// NewAppClient returns a new app client that may already
// be authenticated.
func NewAppClient(c *cli.Context) (*AppClient, error) {
	baseURL, rpcOpts, err := checkBaseURL(c)
	if err != nil {
		return nil, err
	}

	var authFlow *authFlow
	if isProdBaseURL(baseURL) {
		authFlow = newCLIAuthFlow(c.App.Writer)
	} else {
		authFlow = newStgCLIAuthFlow(c.App.Writer)
	}

	conf, err := configFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		conf = &Config{}
	}

	return &AppClient{
		c:           c,
		conf:        conf,
		baseURL:     baseURL,
		rpcOpts:     rpcOpts,
		selectedOrg: &apppb.Organization{},
		selectedLoc: &apppb.Location{},
		authFlow:    authFlow,
	}, nil
}

// Login goes through the CLI login flow using a device code and browser. Once logged in the access token and user details
// are cached on disk.
func (c *AppClient) Login() error {
	var token *Token
	var err error
	if c.conf.Auth != nil && c.conf.Auth.CanRefresh() {
		token, err = c.authFlow.Refresh(c.c.Context, c.conf.Auth)
		if err != nil {
			utils.UncheckedError(c.Logout())
			return err
		}
	} else {
		token, err = c.authFlow.Login(c.c.Context)
		if err != nil {
			return err
		}
	}

	// write token to config.
	c.conf.Auth = token

	return storeConfigToCache(c.conf)
}

// Config returns the current config.
func (c *AppClient) Config() *Config {
	return c.conf
}

// RPCOpts returns RPC dial options dervied from the base URL
// being used in the current invocation of the CLI.
func (c *AppClient) RPCOpts() []rpc.DialOption {
	rpcOpts := make([]rpc.DialOption, len(c.rpcOpts))
	copy(rpcOpts, c.rpcOpts)
	return rpcOpts
}

// SelectedOrg returns the currently selected organization, possibly zero initialized.
func (c *AppClient) SelectedOrg() *apppb.Organization {
	return c.selectedOrg
}

// SelectedLoc returns the currently selected location, possibly zero initialized.
func (c *AppClient) SelectedLoc() *apppb.Location {
	return c.selectedLoc
}

func (c *AppClient) ensureLoggedIn() error {
	if c.client != nil {
		return nil
	}

	if c.conf.Auth == nil {
		return errors.New("not logged in")
	}

	if c.conf.Auth.IsExpired() {
		if !c.conf.Auth.CanRefresh() {
			utils.UncheckedError(c.Logout())
			return errors.New("token expired and cannot refresh")
		}

		// expired.
		newToken, err := c.authFlow.Refresh(c.c.Context, c.conf.Auth)
		if err != nil {
			utils.UncheckedError(c.Logout()) // clear cache if failed to refresh
			return errors.Wrapf(err, "error while refrshing token")
		}

		// write token to config.
		c.conf.Auth = newToken
		if err := storeConfigToCache(c.conf); err != nil {
			return err
		}
	}

	rpcOpts := append(c.RPCOpts(), rpc.WithStaticAuthenticationMaterial(c.conf.Auth.AccessToken))

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
	return nil
}

// PrepareAuthorization prepares authorization for this device and returns
// the device token to late authenticate with and the URL to authorize the device
// at.
func (c *AppClient) PrepareAuthorization() (string, string, error) {
	req, err := http.NewRequest(
		http.MethodPost, fmt.Sprintf("%s/auth/device", c.baseURL), nil)
	if err != nil {
		return "", "", err
	}
	req = req.WithContext(c.c.Context)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() {
		utils.UncheckedError(resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var deviceData struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&deviceData); err != nil {
		return "", "", err
	}
	return deviceData.Token, deviceData.URL, nil
}

// Logout logs out the client and clears the config.
func (c *AppClient) Logout() error {
	if err := removeConfigFromCache(); err != nil && !os.IsNotExist(err) {
		return err
	}
	c.conf = &Config{}
	return nil
}

func (c *AppClient) loadOrganizations() error {
	resp, err := c.client.ListOrganizations(c.c.Context, &apppb.ListOrganizationsRequest{})
	if err != nil {
		return err
	}
	c.orgs = &resp.Organizations
	return nil
}

func (c *AppClient) selectOrganization(orgStr string) error {
	if orgStr != "" && (c.selectedOrg.Id == orgStr || c.selectedOrg.Name == orgStr) {
		return nil
	}
	c.orgs = nil
	c.locs = nil

	if err := c.loadOrganizations(); err != nil {
		return err
	}
	var orgIsID bool
	if orgStr == "" {
		if len(*c.orgs) == 0 {
			return errors.New("no organizations to work with")
		}
		c.selectedOrg = (*c.orgs)[0]
		return nil
	} else if _, err := uuid.Parse(orgStr); err == nil {
		orgIsID = true
	}
	var foundOrg *apppb.Organization
	for _, org := range *c.orgs {
		if orgIsID {
			if org.Id == orgStr {
				foundOrg = org
				break
			}
			continue
		}
		if org.Name == orgStr {
			foundOrg = org
			break
		}
	}
	if foundOrg == nil {
		return errors.Errorf("no organization found for %q", orgStr)
	}

	c.selectedOrg = foundOrg
	return nil
}

// ListOrganizations returns all organizations belonging to the currently authenticated user.
func (c *AppClient) ListOrganizations() ([]*apppb.Organization, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	if err := c.loadOrganizations(); err != nil {
		return nil, err
	}
	return (*c.orgs), nil
}

func (c *AppClient) loadLocations() error {
	if c.selectedOrg.Id == "" {
		return errors.New("must select organization first")
	}
	resp, err := c.client.ListLocations(c.c.Context, &apppb.ListLocationsRequest{OrganizationId: c.selectedOrg.Id})
	if err != nil {
		return err
	}
	c.locs = &resp.Locations
	return nil
}

func (c *AppClient) selectLocation(locStr string) error {
	if locStr != "" && (c.selectedLoc.Id == locStr || c.selectedLoc.Name == locStr) {
		return nil
	}
	c.locs = nil

	if err := c.loadLocations(); err != nil {
		return err
	}
	if locStr == "" {
		if len(*c.locs) == 0 {
			return errors.New("no locations to work with")
		}
		c.selectedLoc = (*c.locs)[0]
		return nil
	}
	var foundLocs []*apppb.Location
	for _, loc := range *c.locs {
		if loc.Id == locStr || loc.Name == locStr {
			foundLocs = append(foundLocs, loc)
			break
		}
	}
	if len(foundLocs) == 0 {
		return errors.Errorf("no location found for %q", locStr)
	}
	if len(foundLocs) != 1 {
		return errors.Errorf("multiple locations match %q: %v", locStr, foundLocs)
	}

	c.selectedLoc = foundLocs[0]
	return nil
}

// ListLocations returns all locations in the given organizationbelonging to the currently authenticated user.
func (c *AppClient) ListLocations(orgID string) ([]*apppb.Location, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	if err := c.selectOrganization(orgID); err != nil {
		return nil, err
	}
	if err := c.loadLocations(); err != nil {
		return nil, err
	}
	return (*c.locs), nil
}

// ListRobots returns all robots in the given location.
func (c *AppClient) ListRobots(orgStr, locStr string) ([]*apppb.Robot, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	if err := c.selectOrganization(orgStr); err != nil {
		return nil, err
	}
	if err := c.selectLocation(locStr); err != nil {
		return nil, err
	}
	resp, err := c.client.ListRobots(c.c.Context, &apppb.ListRobotsRequest{
		LocationId: c.selectedLoc.Id,
	})
	if err != nil {
		return nil, err
	}
	return resp.Robots, nil
}

// Robot returns the given robot.
func (c *AppClient) Robot(orgStr, locStr, robotStr string) (*apppb.Robot, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	robots, err := c.ListRobots(orgStr, locStr)
	if err != nil {
		return nil, err
	}

	for _, robot := range robots {
		if robot.Id == robotStr || robot.Name == robotStr {
			return robot, nil
		}
	}
	return nil, errors.Errorf("no robot found for %q", robotStr)
}

// RobotPart returns the given robot part belonging to a robot.
func (c *AppClient) RobotPart(orgStr, locStr, robotStr, partStr string) (*apppb.RobotPart, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	parts, err := c.RobotParts(orgStr, locStr, robotStr)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		if part.Id == partStr || part.Name == partStr {
			return part, nil
		}
	}
	return nil, errors.Errorf("no robot part found for %q", partStr)
}

func (c *AppClient) robotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool) ([]*apppb.LogEntry, error) {
	part, err := c.RobotPart(orgStr, locStr, robotStr, partStr)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetRobotPartLogs(c.c.Context, &apppb.GetRobotPartLogsRequest{
		Id:         part.Id,
		ErrorsOnly: errorsOnly,
	})
	if err != nil {
		return nil, err
	}

	return resp.Logs, nil
}

// RobotPartLogs returns recent logs for the given robot part.
func (c *AppClient) RobotPartLogs(orgStr, locStr, robotStr, partStr string) ([]*apppb.LogEntry, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.robotPartLogs(orgStr, locStr, robotStr, partStr, false)
}

// RobotPartLogsErrors returns recent error logs for the given robot part.
func (c *AppClient) RobotPartLogsErrors(orgStr, locStr, robotStr, partStr string) ([]*apppb.LogEntry, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.robotPartLogs(orgStr, locStr, robotStr, partStr, true)
}

// RobotParts returns all parts of the given robot.
func (c *AppClient) RobotParts(orgStr, locStr, robotStr string) ([]*apppb.RobotPart, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	robot, err := c.Robot(orgStr, locStr, robotStr)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetRobotParts(c.c.Context, &apppb.GetRobotPartsRequest{
		RobotId: robot.Id,
	})
	if err != nil {
		return nil, err
	}
	return resp.Parts, nil
}

func (c *AppClient) printRobotPartLogsInternal(logs []*apppb.LogEntry, indent string) {
	for _, log := range logs {
		fmt.Fprintf(
			c.c.App.Writer,
			"%s%s\t%s\t%s\t%s\n",
			indent,
			log.Time.AsTime().Format("2006-01-02T15:04:05.000Z0700"),
			log.Level,
			log.LoggerName,
			log.Message,
		)
	}
}

// PrintRobotPartLogs prints logs for the given robot part.
func (c *AppClient) PrintRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
	logs, err := c.robotPartLogs(orgStr, locStr, robotStr, partStr, errorsOnly)
	if err != nil {
		return err
	}

	if header != "" {
		fmt.Fprintln(c.c.App.Writer, header)
	}
	if len(logs) == 0 {
		fmt.Fprintf(c.c.App.Writer, "%sno recent logs\n", indent)
		return nil
	}
	c.printRobotPartLogsInternal(logs, indent)
	return nil
}

// TailRobotPartLogs tails and prints logs for the given robot part.
func (c *AppClient) TailRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
	part, err := c.RobotPart(orgStr, locStr, robotStr, partStr)
	if err != nil {
		return err
	}
	tailClient, err := c.client.TailRobotPartLogs(c.c.Context, &apppb.TailRobotPartLogsRequest{
		Id:         part.Id,
		ErrorsOnly: errorsOnly,
	})
	if err != nil {
		return err
	}

	if header != "" {
		fmt.Fprintln(c.c.App.Writer, header)
	}

	for {
		resp, err := tailClient.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		c.printRobotPartLogsInternal(resp.Logs, indent)
	}
}

func (c *AppClient) prepareDial(
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

	part, err := c.RobotPart(c.selectedOrg.Id, c.selectedLoc.Id, robotStr, partStr)
	if err != nil {
		return nil, "", nil, err
	}

	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		utils.UncheckedError(rpcDialer.Close())
	}()
	dialCtx := rpc.ContextWithDialer(c.c.Context, rpcDialer)

	rpcOpts := append(c.RPCOpts(),
		rpc.WithExternalAuth(c.baseURL.Host, part.Fqdn),
		rpc.WithStaticExternalAuthenticationMaterial(c.conf.Auth.AccessToken),
	)

	if debug {
		rpcOpts = append(rpcOpts, rpc.WithDialDebug())
	}

	return dialCtx, part.Fqdn, rpcOpts, nil
}

// RunRobotPartCommand runs the given command on a robot part.
func (c *AppClient) RunRobotPartCommand(
	orgStr, locStr, robotStr, partStr string,
	svcMethod, data string,
	streamDur time.Duration,
	debug bool,
	logger golog.Logger,
) error {
	dialCtx, fqdn, rpcOpts, err := c.prepareDial(orgStr, locStr, robotStr, partStr, debug)
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(dialCtx, fqdn, logger, rpcOpts...)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(conn.Close())
	}()

	refCtx := metadata.NewOutgoingContext(c.c.Context, nil)
	refClient := grpcreflect.NewClient(refCtx, reflectpb.NewServerReflectionClient(conn))
	reflSource := grpcurl.DescriptorSourceFromServer(c.c.Context, refClient)
	descSource := reflSource

	options := grpcurl.FormatOptions{
		EmitJSONDefaultFields: true,
		IncludeTextSeparator:  true,
		AllowUnknownFields:    true,
	}

	invoke := func() (bool, error) {
		rf, formatter, err := grpcurl.RequestParserAndFormatter(
			grpcurl.Format("json"),
			descSource,
			strings.NewReader(data),
			options)
		if err != nil {
			return false, err
		}

		h := &grpcurl.DefaultEventHandler{
			Out:            c.c.App.Writer,
			Formatter:      formatter,
			VerbosityLevel: 0,
		}

		if err := grpcurl.InvokeRPC(
			c.c.Context,
			descSource,
			conn,
			svcMethod,
			nil,
			h,
			rf.Next,
		); err != nil {
			return false, err
		}

		if h.Status.Code() != codes.OK {
			grpcurl.PrintStatus(c.c.App.ErrWriter, h.Status, formatter)
			cli.OsExiter(1)
			return false, nil
		}

		return true, nil
	}

	if streamDur == 0 {
		_, err := invoke()
		return err
	}

	ticker := time.NewTicker(streamDur)
	defer ticker.Stop()

	for {
		if err := c.c.Context.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		select {
		case <-c.c.Context.Done():
			return nil
		case <-ticker.C:
			if ok, err := invoke(); err != nil {
				return err
			} else if !ok {
				return nil
			}
		}
	}
}

// StartRobotPartShell starts a shell on a robot part.
func (c *AppClient) StartRobotPartShell(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	logger golog.Logger,
) error {
	dialCtx, fqdn, rpcOpts, err := c.prepareDial(orgStr, locStr, robotStr, partStr, debug)
	if err != nil {
		return err
	}

	if debug {
		fmt.Fprintln(c.c.App.Writer, "establishing connection...")
	}
	robotClient, err := client.New(dialCtx, fqdn, logger, client.WithDialOptions(rpcOpts...))
	if err != nil {
		fmt.Fprintln(c.c.App.ErrWriter, err)
		cli.OsExiter(1)
		return nil
	}

	defer func() {
		utils.UncheckedError(robotClient.Close(c.c.Context))
	}()

	// Returns the first shell service found in the robot resources
	var found *resource.Name
	for _, name := range robotClient.ResourceNames() {
		if name.Subtype == shell.Subtype {
			nameCopy := name
			found = &nameCopy
			break
		}
	}
	if found == nil {
		fmt.Fprintln(c.c.App.ErrWriter, "shell service is not enabled")
		cli.OsExiter(1)
		return nil
	}

	shellRes, err := robotClient.ResourceByName(*found)
	if err != nil {
		fmt.Fprintln(c.c.App.ErrWriter, err)
		cli.OsExiter(1)
		return nil
	}

	shellSvc, ok := shellRes.(shell.Service)
	if !ok {
		fmt.Fprintln(c.c.App.ErrWriter, "shell service is not a shell service")
		cli.OsExiter(1)
		return nil
	}

	input, output, err := shellSvc.Shell(c.c.Context, map[string]interface{}{})
	if err != nil {
		fmt.Fprintln(c.c.App.ErrWriter, err)
		cli.OsExiter(1)
		return nil
	}

	setRaw := func(isRaw bool) error {
		r := "raw"
		if !isRaw {
			r = "-raw"
		}

		rawMode := exec.Command("stty", r)
		rawMode.Stdin = os.Stdin
		return rawMode.Run()
	}
	if err := setRaw(true); err != nil {
		fmt.Fprintln(c.c.App.ErrWriter, err)
		cli.OsExiter(1)
		return nil
	}
	defer func() {
		utils.UncheckedError(setRaw(false))
	}()

	utils.PanicCapturingGo(func() {
		var data [64]byte
		for {
			select {
			case <-c.c.Context.Done():
				close(input)
				return
			default:
			}

			n, err := os.Stdin.Read(data[:])
			if err != nil {
				close(input)
				return
			}
			select {
			case <-c.c.Context.Done():
				close(input)
				return
			case input <- string(data[:n]):
			}
		}
	})

	outputLoop := func() {
		for {
			select {
			case <-c.c.Context.Done():
				return
			case outputData, ok := <-output:
				if ok {
					if outputData.Output != "" {
						fmt.Fprint(c.c.App.Writer, outputData.Output)
					}
					if outputData.Error != "" {
						fmt.Fprint(c.c.App.ErrWriter, outputData.Error)
					}
					if outputData.EOF {
						return
					}
				} else {
					return
				}
			}
		}
	}

	outputLoop()
	return nil
}
