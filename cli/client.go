// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	datapb "go.viam.com/api/app/data/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	rconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
)

// moduleUploadChunkSize sets the number of bytes included in each chunk of the upload stream.
var moduleUploadChunkSize = 32 * 1024

// appClient wraps a cli.Context and provides all the CLI command functionality
// needed to talk to the app service but not directly to robot parts.
type appClient struct {
	c          *cli.Context
	conf       *config
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

// ListOrganizationsAction is the corresponding Action for 'organizations list'.
func ListOrganizationsAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}
	orgs, err := client.ListOrganizations()
	if err != nil {
		return errors.Wrap(err, "could not list organizations")
	}
	for i, org := range orgs {
		if i == 0 {
			fmt.Fprintf(c.App.Writer, "organizations for %q:\n", client.config().Auth.User.Email)
		}
		fmt.Fprintf(c.App.Writer, "\t%s (id: %s)\n", org.Name, org.Id)
	}
	return nil
}

// ListLocationsAction is the corresponding Action for 'locations list'.
func ListLocationsAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}
	orgStr := c.Args().First()
	listLocations := func(orgID string) error {
		locs, err := client.ListLocations(orgID)
		if err != nil {
			return errors.Wrap(err, "could not list locations")
		}
		for _, loc := range locs {
			fmt.Fprintf(c.App.Writer, "\t%s (id: %s)\n", loc.Name, loc.Id)
		}
		return nil
	}
	if orgStr == "" {
		orgs, err := client.ListOrganizations()
		if err != nil {
			return errors.Wrap(err, "could not list organizations")
		}
		for i, org := range orgs {
			if i == 0 {
				fmt.Fprintf(c.App.Writer, "locations for %q:\n", client.config().Auth.User.Email)
			}
			fmt.Fprintf(c.App.Writer, "%s:\n", org.Name)
			if err := listLocations(org.Id); err != nil {
				return err
			}
		}
		return nil
	}
	return listLocations(orgStr)
}

// ListRobotsAction is the corresponding Action for 'robots list'.
func ListRobotsAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}
	orgStr := c.String("organization")
	locStr := c.String("location")
	robots, err := client.ListRobots(orgStr, locStr)
	if err != nil {
		return errors.Wrap(err, "could not list robots")
	}

	if orgStr == "" || locStr == "" {
		fmt.Fprintf(c.App.Writer, "%s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name)
	}

	for _, robot := range robots {
		fmt.Fprintf(c.App.Writer, "%s (id: %s)\n", robot.Name, robot.Id)
	}
	return nil
}

// RobotStatusAction is the corresponding Action for 'robot status'.
func RobotStatusAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String("organization")
	locStr := c.String("location")
	robot, err := client.Robot(orgStr, locStr, c.String("robot"))
	if err != nil {
		return err
	}
	parts, err := client.RobotParts(client.SelectedOrg().Id, client.SelectedLoc().Id, robot.Id)
	if err != nil {
		return errors.Wrap(err, "could not get robot parts")
	}

	if orgStr == "" || locStr == "" {
		fmt.Fprintf(c.App.Writer, "%s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name)
	}

	fmt.Fprintf(
		c.App.Writer,
		"ID: %s\nname: %s\nlast access: %s (%s ago)\n",
		robot.Id,
		robot.Name,
		robot.LastAccess.AsTime().Format(time.UnixDate),
		time.Since(robot.LastAccess.AsTime()),
	)

	if len(parts) != 0 {
		fmt.Fprintln(c.App.Writer, "parts:")
	}
	for i, part := range parts {
		name := part.Name
		if part.MainPart {
			name += " (main)"
		}
		fmt.Fprintf(
			c.App.Writer,
			"\tID: %s\n\tname: %s\n\tlast access: %s (%s ago)\n",
			part.Id,
			name,
			part.LastAccess.AsTime().Format(time.UnixDate),
			time.Since(part.LastAccess.AsTime()),
		)
		if i != len(parts)-1 {
			fmt.Fprintln(c.App.Writer, "")
		}
	}

	return nil
}

// RobotLogsAction is the corresponding Action for 'robot logs'.
func RobotLogsAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String("organization")
	locStr := c.String("location")
	robotStr := c.String("robot")
	robot, err := client.Robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	parts, err := client.RobotParts(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot parts")
	}

	for i, part := range parts {
		if i != 0 {
			fmt.Fprintln(c.App.Writer, "")
		}

		var header string
		if orgStr == "" || locStr == "" || robotStr == "" {
			header = fmt.Sprintf("%s -> %s -> %s -> %s", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name, part.Name)
		} else {
			header = part.Name
		}
		if err := client.PrintRobotPartLogs(
			orgStr, locStr, robotStr, part.Id,
			c.Bool("errors"),
			"\t",
			header,
		); err != nil {
			return errors.Wrap(err, "could not print robot logs")
		}
	}

	return nil
}

// RobotPartStatusAction is the corresponding Action for 'robot part status'.
func RobotPartStatusAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String("organization")
	locStr := c.String("location")
	robotStr := c.String("robot")
	robot, err := client.Robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	part, err := client.RobotPart(orgStr, locStr, robotStr, c.String("part"))
	if err != nil {
		return errors.Wrap(err, "could not get robot part")
	}

	if orgStr == "" || locStr == "" || robotStr == "" {
		fmt.Fprintf(c.App.Writer, "%s -> %s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name)
	}

	name := part.Name
	if part.MainPart {
		name += " (main)"
	}
	fmt.Fprintf(
		c.App.Writer,
		"ID: %s\nname: %s\nlast access: %s (%s ago)\n",
		part.Id,
		name,
		part.LastAccess.AsTime().Format(time.UnixDate),
		time.Since(part.LastAccess.AsTime()),
	)

	return nil
}

// RobotPartLogsAction is the corresponding Action for 'robot part logs'.
func RobotPartLogsAction(c *cli.Context) error {
	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String("organization")
	locStr := c.String("location")
	robotStr := c.String("robot")
	robot, err := client.Robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	var header string
	if orgStr == "" || locStr == "" || robotStr == "" {
		header = fmt.Sprintf("%s -> %s -> %s", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name)
	}
	if c.Bool("tail") {
		return client.TailRobotPartLogs(
			orgStr, locStr, robotStr, c.String("part"),
			c.Bool("errors"),
			"",
			header,
		)
	}
	return client.PrintRobotPartLogs(
		orgStr, locStr, robotStr, c.String("part"),
		c.Bool("errors"),
		"",
		header,
	)
}

// RobotPartRunAction is the corresponding Action for 'robot part run'.
func RobotPartRunAction(c *cli.Context) error {
	svcMethod := c.Args().First()
	if svcMethod == "" {
		return errors.New("service method required")
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of "debug" flag.
	logger := zap.NewNop().Sugar()
	if c.Bool("debug") {
		logger = golog.NewDebugLogger("cli")
	}

	return client.RunRobotPartCommand(
		c.String("organization"),
		c.String("location"),
		c.String("robot"),
		c.String("part"),
		svcMethod,
		c.String("data"),
		c.Duration("stream"),
		c.Bool("debug"),
		logger,
	)
}

// RobotPartShellAction is the corresponding Action for 'robot part shell'.
func RobotPartShellAction(c *cli.Context) error {
	Infof(c.App.Writer, "ensure robot part has a valid shell type service")

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of "debug" flag.
	logger := zap.NewNop().Sugar()
	if c.Bool("debug") {
		logger = golog.NewDebugLogger("cli")
	}

	return client.StartRobotPartShell(
		c.String("organization"),
		c.String("location"),
		c.String("robot"),
		c.String("part"),
		c.Bool("debug"),
		logger,
	)
}

// VersionAction is the corresponding Action for 'version'.
func VersionAction(c *cli.Context) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("error reading build info")
	}
	if c.Bool("debug") {
		fmt.Fprintf(c.App.Writer, "%s\n", info.String())
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	version := "?"
	if rev, ok := settings["vcs.revision"]; ok {
		version = rev[:8]
		if settings["vcs.modified"] == "true" {
			version += "+"
		}
	}
	deps := make(map[string]*debug.Module, len(info.Deps))
	for _, dep := range info.Deps {
		deps[dep.Path] = dep
	}
	apiVersion := "?"
	if dep, ok := deps["go.viam.com/api"]; ok {
		apiVersion = dep.Version
	}
	appVersion := rconfig.Version
	if appVersion == "" {
		appVersion = "(dev)"
	}
	fmt.Fprintf(c.App.Writer, "version %s git=%s api=%s\n", appVersion, version, apiVersion)
	return nil
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
func newAppClient(c *cli.Context) (*appClient, error) {
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
		conf = &config{}
	}

	return &appClient{
		c:           c,
		conf:        conf,
		baseURL:     baseURL,
		rpcOpts:     rpcOpts,
		selectedOrg: &apppb.Organization{},
		selectedLoc: &apppb.Location{},
		authFlow:    authFlow,
	}, nil
}

func (c *appClient) config() *config {
	return c.conf
}

// copyRPCOpts returns a copy of the RPC dial options dervied from the base URL
// being used in the current invocation of the CLI.
func (c *appClient) copyRPCOpts() []rpc.DialOption {
	rpcOpts := make([]rpc.DialOption, len(c.rpcOpts))
	copy(rpcOpts, c.rpcOpts)
	return rpcOpts
}

// SelectedOrg returns the currently selected organization, possibly zero initialized.
func (c *appClient) SelectedOrg() *apppb.Organization {
	return c.selectedOrg
}

// SelectedLoc returns the currently selected location, possibly zero initialized.
func (c *appClient) SelectedLoc() *apppb.Location {
	return c.selectedLoc
}

func (c *appClient) ensureLoggedIn() error {
	if c.client != nil {
		return nil
	}

	if c.conf.Auth == nil {
		return errors.New("not logged in: run the following command to login:\n\tviam login")
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
			return errors.Wrapf(err, "error while refreshing token")
		}

		// write token to config.
		c.conf.Auth = newToken
		if err := storeConfigToCache(c.conf); err != nil {
			return err
		}
	}

	rpcOpts := append(c.copyRPCOpts(), rpc.WithStaticAuthenticationMaterial(c.conf.Auth.AccessToken))

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
func (c *appClient) PrepareAuthorization() (string, string, error) {
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
func (c *appClient) Logout() error {
	if err := removeConfigFromCache(); err != nil && !os.IsNotExist(err) {
		return err
	}
	c.conf = &config{}
	return nil
}

func (c *appClient) loadOrganizations() error {
	resp, err := c.client.ListOrganizations(c.c.Context, &apppb.ListOrganizationsRequest{})
	if err != nil {
		return err
	}
	c.orgs = &resp.Organizations
	return nil
}

func (c *appClient) selectOrganization(orgStr string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
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

// GetOrg gets an org by an indentifying string. If the orgStr is an
// org UUID, then this matchs on organization ID, otherwise this will match
// on organization name.
func (c *appClient) GetOrg(orgStr string) (*apppb.Organization, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	resp, err := c.client.ListOrganizations(c.c.Context, &apppb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}
	organizations := resp.GetOrganizations()
	var orgIsID bool
	if _, err := uuid.Parse(orgStr); err == nil {
		orgIsID = true
	}
	for _, org := range organizations {
		if orgIsID {
			if org.Id == orgStr {
				return org, nil
			}
		} else if org.Name == orgStr {
			return org, nil
		}
	}
	return nil, errors.Errorf("no organization found for %q", orgStr)
}

// GetUserOrgByPublicNamespace searches the logged in users orgs to see
// if any have a matching public namespace.
func (c *appClient) GetUserOrgByPublicNamespace(publicNamespace string) (*apppb.Organization, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	if err := c.loadOrganizations(); err != nil {
		return nil, err
	}
	for _, org := range *c.orgs {
		if org.PublicNamespace == publicNamespace {
			return org, nil
		}
	}
	return nil, errors.Errorf("none of your organizations have a public namespace of %q", publicNamespace)
}

// ListOrganizations returns all organizations belonging to the currently authenticated user.
func (c *appClient) ListOrganizations() ([]*apppb.Organization, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	if err := c.loadOrganizations(); err != nil {
		return nil, err
	}
	return (*c.orgs), nil
}

func (c *appClient) loadLocations() error {
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

func (c *appClient) selectLocation(locStr string) error {
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
func (c *appClient) ListLocations(orgID string) ([]*apppb.Location, error) {
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
func (c *appClient) ListRobots(orgStr, locStr string) ([]*apppb.Robot, error) {
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
func (c *appClient) Robot(orgStr, locStr, robotStr string) (*apppb.Robot, error) {
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
func (c *appClient) RobotPart(orgStr, locStr, robotStr, partStr string) (*apppb.RobotPart, error) {
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

func (c *appClient) robotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool) ([]*apppb.LogEntry, error) {
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
func (c *appClient) RobotPartLogs(orgStr, locStr, robotStr, partStr string) ([]*apppb.LogEntry, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.robotPartLogs(orgStr, locStr, robotStr, partStr, false)
}

// RobotPartLogsErrors returns recent error logs for the given robot part.
func (c *appClient) RobotPartLogsErrors(orgStr, locStr, robotStr, partStr string) ([]*apppb.LogEntry, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.robotPartLogs(orgStr, locStr, robotStr, partStr, true)
}

// RobotParts returns all parts of the given robot.
func (c *appClient) RobotParts(orgStr, locStr, robotStr string) ([]*apppb.RobotPart, error) {
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

func (c *appClient) printRobotPartLogsInternal(logs []*apppb.LogEntry, indent string) {
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
func (c *appClient) PrintRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
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
func (c *appClient) TailRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
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

func (c *appClient) prepareDial(
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

	rpcOpts := append(c.copyRPCOpts(),
		rpc.WithExternalAuth(c.baseURL.Host, part.Fqdn),
		rpc.WithStaticExternalAuthenticationMaterial(c.conf.Auth.AccessToken),
	)

	if debug {
		rpcOpts = append(rpcOpts, rpc.WithDialDebug())
	}

	return dialCtx, part.Fqdn, rpcOpts, nil
}

// RunRobotPartCommand runs the given command on a robot part.
func (c *appClient) RunRobotPartCommand(
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
	refClient := grpcreflect.NewClientV1Alpha(refCtx, reflectpb.NewServerReflectionClient(conn))
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
func (c *appClient) StartRobotPartShell(
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
		return errors.Wrap(err, "could not connect to robot part")
	}

	defer func() {
		utils.UncheckedError(robotClient.Close(c.c.Context))
	}()

	// Returns the first shell service found in the robot resources
	var found *resource.Name
	for _, name := range robotClient.ResourceNames() {
		if name.API == shell.API {
			nameCopy := name
			found = &nameCopy
			break
		}
	}
	if found == nil {
		return errors.New("shell service is not enabled on this robot part")
	}

	shellRes, err := robotClient.ResourceByName(*found)
	if err != nil {
		return errors.Wrap(err, "could not get shell service from robot part")
	}

	shellSvc, ok := shellRes.(shell.Service)
	if !ok {
		return errors.New("could not get shell service from robot part")
	}

	input, output, err := shellSvc.Shell(c.c.Context, map[string]interface{}{})
	if err != nil {
		return err
	}

	setRaw := func(isRaw bool) error {
		// NOTE(benjirewis): Linux systems seem to need both "raw" (no processing) and "-echo"
		// (no echoing back inputted characters) in order to allow the input and output loops
		// below to completely control the terminal.
		args := []string{"raw", "-echo"}
		if !isRaw {
			args = []string{"-raw", "echo"}
		}

		rawMode := exec.Command("stty", args...)
		rawMode.Stdin = os.Stdin
		return rawMode.Run()
	}
	if err := setRaw(true); err != nil {
		return err
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

// CreateModule wraps the grpc CreateModule request.
func (c *appClient) CreateModule(moduleName, organizationID string) (*apppb.CreateModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := apppb.CreateModuleRequest{
		Name:           moduleName,
		OrganizationId: organizationID,
	}
	return c.client.CreateModule(c.c.Context, &req)
}

// UpdateModule wraps the grpc UpdateModule request.
func (c *appClient) UpdateModule(moduleID ModuleID, manifest ModuleManifest) (*apppb.UpdateModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	var models []*apppb.Model
	for _, moduleComponent := range manifest.Models {
		models = append(models, moduleComponentToProto(moduleComponent))
	}
	visibility, err := visibilityToProto(manifest.Visibility)
	if err != nil {
		return nil, err
	}
	req := apppb.UpdateModuleRequest{
		ModuleId:    moduleID.toString(),
		Visibility:  visibility,
		Url:         manifest.URL,
		Description: manifest.Description,
		Models:      models,
		Entrypoint:  manifest.Entrypoint,
	}
	return c.client.UpdateModule(c.c.Context, &req)
}

// UploadModuleFile wraps the grpc UploadModuleFile request.
func (c *appClient) UploadModuleFile(
	moduleID ModuleID,
	version,
	platform string,
	file *os.File,
) (*apppb.UploadModuleFileResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	stream, err := c.client.UploadModuleFile(ctx)
	if err != nil {
		return nil, err
	}
	moduleFileInfo := apppb.ModuleFileInfo{
		ModuleId: moduleID.toString(),
		Version:  version,
		Platform: platform,
	}
	req := &apppb.UploadModuleFileRequest{
		ModuleFile: &apppb.UploadModuleFileRequest_ModuleFileInfo{ModuleFileInfo: &moduleFileInfo},
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}

	var errs error
	// We do not add the EOF as an error because all server-side errors trigger an EOF on the stream
	// This results in extra clutter to the error msg
	if err := sendModuleUploadRequests(ctx, stream, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	errs = multierr.Combine(errs, closeErr)
	return resp, errs
}

func sendModuleUploadRequests(ctx context.Context, stream apppb.AppService_UploadModuleFileClient, file *os.File, stdout io.Writer) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()
	uploadedBytes := 0
	// Close the line with the progress reading
	defer fmt.Fprint(stdout, "\n")

	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file or the context expires.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextModuleUploadRequest(file)

		// EOF means we've completed successfully.
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "could not read file")
		}

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
		uploadedBytes += len(uploadReq.GetFile())
		// Simple progress reading until we have a proper tui library
		uploadPercent := int(math.Ceil(100 * float64(uploadedBytes) / float64(fileSize)))
		fmt.Fprintf(stdout, "\r\auploading... %d%% (%d/%d bytes)", uploadPercent, uploadedBytes, fileSize)
	}
}

func getNextModuleUploadRequest(file *os.File) (*apppb.UploadModuleFileRequest, error) {
	// get the next chunk of bytes from the file
	byteArr := make([]byte, moduleUploadChunkSize)
	numBytesRead, err := file.Read(byteArr)
	if err != nil {
		return nil, err
	}
	if numBytesRead < moduleUploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	return &apppb.UploadModuleFileRequest{
		ModuleFile: &apppb.UploadModuleFileRequest_File{
			File: byteArr,
		},
	}, nil
}

func visibilityToProto(visibility ModuleVisibility) (apppb.Visibility, error) {
	switch visibility {
	case ModuleVisibilityPrivate:
		return apppb.Visibility_VISIBILITY_PRIVATE, nil
	case ModuleVisibilityPublic:
		return apppb.Visibility_VISIBILITY_PUBLIC, nil
	default:
		return apppb.Visibility_VISIBILITY_UNSPECIFIED,
			errors.Errorf("invalid module visibility. must be either %q or %q", ModuleVisibilityPublic, ModuleVisibilityPrivate)
	}
}

func moduleComponentToProto(moduleComponent ModuleComponent) *apppb.Model {
	return &apppb.Model{
		Api:   moduleComponent.API,
		Model: moduleComponent.Model,
	}
}
