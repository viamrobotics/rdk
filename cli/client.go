// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	rconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
)

// viamClient wraps a cli.Context and provides all the CLI command functionality
// needed to talk to the app and data services but not directly to robot parts.
type viamClient struct {
	c                *cli.Context
	conf             *config
	client           apppb.AppServiceClient
	buildClient      buildpb.BuildServiceClient
	dataClient       datapb.DataServiceClient
	packageClient    packagepb.PackageServiceClient
	datasetClient    datasetpb.DatasetServiceClient
	mlTrainingClient mltrainingpb.MLTrainingServiceClient
	buildClient      buildpb.BuildServiceClient
	baseURL          *url.URL
	rpcOpts          []rpc.DialOption
	authFlow         *authFlow

	selectedOrg *apppb.Organization
	selectedLoc *apppb.Location

	// caches
	orgs *[]*apppb.Organization
	locs *[]*apppb.Location
}

// ListOrganizationsAction is the corresponding Action for 'organizations list'.
func ListOrganizationsAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.listOrganizationsAction(cCtx)
}

func (c *viamClient) listOrganizationsAction(cCtx *cli.Context) error {
	orgs, err := c.listOrganizations()
	if err != nil {
		return errors.Wrap(err, "could not list organizations")
	}
	for i, org := range orgs {
		if i == 0 {
			printf(cCtx.App.Writer, "Organizations for %q:", c.conf.Auth)
		}
		idInfo := fmt.Sprintf("(id: %s)", org.Id)
		namespaceInfo := ""
		if org.PublicNamespace != "" {
			namespaceInfo = fmt.Sprintf(" (namespace: %s)", org.PublicNamespace)
		}
		printf(cCtx.App.Writer, "\t%s %s%s", org.Name, idInfo, namespaceInfo)
	}
	return nil
}

// ListLocationsAction is the corresponding Action for 'locations list'.
func ListLocationsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	orgStr := c.Args().First()
	listLocations := func(orgID string) error {
		locs, err := client.listLocations(orgID)
		if err != nil {
			return errors.Wrap(err, "could not list locations")
		}
		for _, loc := range locs {
			printf(c.App.Writer, "\t%s (id: %s)", loc.Name, loc.Id)
		}
		return nil
	}
	if orgStr == "" {
		orgs, err := client.listOrganizations()
		if err != nil {
			return errors.Wrap(err, "could not list organizations")
		}
		for i, org := range orgs {
			if i == 0 {
				printf(c.App.Writer, "Locations for %q:", client.conf.Auth)
			}
			printf(c.App.Writer, "%s:", org.Name)
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
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robots, err := client.listRobots(orgStr, locStr)
	if err != nil {
		return errors.Wrap(err, "could not list robots")
	}

	if orgStr == "" || locStr == "" {
		printf(c.App.Writer, "%s -> %s", client.selectedOrg.Name, client.selectedLoc.Name)
	}

	for _, robot := range robots {
		printf(c.App.Writer, "%s (id: %s)", robot.Name, robot.Id)
	}
	return nil
}

// RobotsStatusAction is the corresponding Action for 'robots status'.
func RobotsStatusAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robot, err := client.robot(orgStr, locStr, c.String(robotFlag))
	if err != nil {
		return err
	}
	parts, err := client.robotParts(client.selectedOrg.Id, client.selectedLoc.Id, robot.Id)
	if err != nil {
		return errors.Wrap(err, "could not get robot parts")
	}

	if orgStr == "" || locStr == "" {
		printf(c.App.Writer, "%s -> %s", client.selectedOrg.Name, client.selectedLoc.Name)
	}

	printf(
		c.App.Writer,
		"ID: %s\nName: %s\nLast Access: %s (%s ago)",
		robot.Id,
		robot.Name,
		robot.LastAccess.AsTime().Format(time.UnixDate),
		time.Since(robot.LastAccess.AsTime()),
	)

	if len(parts) != 0 {
		printf(c.App.Writer, "Parts:")
	}
	for i, part := range parts {
		name := part.Name
		if part.MainPart {
			name += " (main)"
		}
		printf(
			c.App.Writer,
			"\tID: %s\n\tName: %s\n\tLast Access: %s (%s ago)",
			part.Id,
			name,
			part.LastAccess.AsTime().Format(time.UnixDate),
			time.Since(part.LastAccess.AsTime()),
		)
		if i != len(parts)-1 {
			printf(c.App.Writer, "")
		}
	}

	return nil
}

// RobotsLogsAction is the corresponding Action for 'robots logs'.
func RobotsLogsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robotStr := c.String(robotFlag)
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	parts, err := client.robotParts(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot parts")
	}

	for i, part := range parts {
		if i != 0 {
			printf(c.App.Writer, "")
		}

		var header string
		if orgStr == "" || locStr == "" || robotStr == "" {
			header = fmt.Sprintf("%s -> %s -> %s -> %s", client.selectedOrg.Name, client.selectedLoc.Name, robot.Name, part.Name)
		} else {
			header = part.Name
		}
		if err := client.printRobotPartLogs(
			orgStr, locStr, robotStr, part.Id,
			c.Bool(logsFlagErrors),
			"\t",
			header,
		); err != nil {
			return errors.Wrap(err, "could not print robot logs")
		}
	}

	return nil
}

// RobotsPartStatusAction is the corresponding Action for 'robots part status'.
func RobotsPartStatusAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robotStr := c.String(robotFlag)
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	part, err := client.robotPart(orgStr, locStr, robotStr, c.String(partFlag))
	if err != nil {
		return errors.Wrap(err, "could not get robot part")
	}

	if orgStr == "" || locStr == "" || robotStr == "" {
		printf(c.App.Writer, "%s -> %s -> %s", client.selectedOrg.Name, client.selectedLoc.Name, robot.Name)
	}

	name := part.Name
	if part.MainPart {
		name += " (main)"
	}
	printf(
		c.App.Writer,
		"ID: %s\nName: %s\nLast Access: %s (%s ago)",
		part.Id,
		name,
		part.LastAccess.AsTime().Format(time.UnixDate),
		time.Since(part.LastAccess.AsTime()),
	)

	return nil
}

// RobotsPartLogsAction is the corresponding Action for 'robots part logs'.
func RobotsPartLogsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robotStr := c.String(robotFlag)
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get robot")
	}

	var header string
	if orgStr == "" || locStr == "" || robotStr == "" {
		header = fmt.Sprintf("%s -> %s -> %s", client.selectedOrg.Name, client.selectedLoc.Name, robot.Name)
	}
	if c.Bool(logsFlagTail) {
		return client.tailRobotPartLogs(
			orgStr, locStr, robotStr, c.String(partFlag),
			c.Bool(logsFlagErrors),
			"",
			header,
		)
	}
	return client.printRobotPartLogs(
		orgStr, locStr, robotStr, c.String(partFlag),
		c.Bool(logsFlagErrors),
		"",
		header,
	)
}

// RobotsPartRunAction is the corresponding Action for 'robots part run'.
func RobotsPartRunAction(c *cli.Context) error {
	svcMethod := c.Args().First()
	if svcMethod == "" {
		return errors.New("service method required")
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	if c.Bool(debugFlag) {
		logger = logging.NewDebugLogger("cli")
	}

	return client.runRobotPartCommand(
		c.String(organizationFlag),
		c.String(locationFlag),
		c.String(robotFlag),
		c.String(partFlag),
		svcMethod,
		c.String(runFlagData),
		c.Duration(runFlagStream),
		c.Bool(debugFlag),
		logger,
	)
}

// RobotsPartShellAction is the corresponding Action for 'robots part shell'.
func RobotsPartShellAction(c *cli.Context) error {
	infof(c.App.Writer, "Ensure robot part has a valid shell type service")

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	if c.Bool(debugFlag) {
		logger = logging.NewDebugLogger("cli")
	}

	return client.startRobotPartShell(
		c.String(organizationFlag),
		c.String(locationFlag),
		c.String(robotFlag),
		c.String(partFlag),
		c.Bool(debugFlag),
		logger,
	)
}

// VersionAction is the corresponding Action for 'version'.
func VersionAction(c *cli.Context) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("error reading build info")
	}
	if c.Bool(debugFlag) {
		printf(c.App.Writer, "%s", info.String())
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
	printf(c.App.Writer, "Version %s Git=%s API=%s", appVersion, version, apiVersion)
	return nil
}

var defaultBaseURL = "https://app.viam.com:443"

func parseBaseURL(baseURL string, verifyConnection bool) (*url.URL, []rpc.DialOption, error) {
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, nil, err
	}

	// Go URL parsing can place the host in Path if no scheme is provided; place
	// Path in Host in this case.
	if baseURLParsed.Host == "" && baseURLParsed.Path != "" {
		baseURLParsed.Host = baseURLParsed.Path
		baseURLParsed.Path = ""
	}

	// Assume "https" scheme if none is provided, and assume 8080 port for "http"
	// scheme and 443 port for "https" scheme.
	var secure bool
	switch baseURLParsed.Scheme {
	case "http":
		if baseURLParsed.Port() == "" {
			baseURLParsed.Host = baseURLParsed.Host + ":" + "8080"
		}
	case "https", "":
		secure = true
		baseURLParsed.Scheme = "https"
		if baseURLParsed.Port() == "" {
			baseURLParsed.Host = baseURLParsed.Host + ":" + "443"
		}
	}

	if verifyConnection {
		// Check if URL is even valid with a TCP dial.
		conn, err := net.DialTimeout("tcp", baseURLParsed.Host, 3*time.Second)
		if err != nil {
			return nil, nil, fmt.Errorf("base URL %q (needed for auth) is currently unreachable. "+
				"Ensure URL is valid and you are connected to internet", baseURLParsed.Host)
		}
		utils.UncheckedError(conn.Close())
	}

	if secure {
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

func newViamClient(c *cli.Context) (*viamClient, error) {
	conf, err := configFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		conf = &config{}
	}

	// If base URL was not specified, assume cached base URL. If no base URL is
	// cached, assume default base URL.
	baseURLArg := c.String(baseURLFlag)
	switch {
	case conf.BaseURL == "" && baseURLArg == "":
		conf.BaseURL = defaultBaseURL
	case conf.BaseURL == "" && baseURLArg != "":
		conf.BaseURL = baseURLArg
	case baseURLArg != "" && conf.BaseURL != "" && conf.BaseURL != baseURLArg:
		return nil, fmt.Errorf("cached base URL for this session is %q. "+
			"Please logout and login again to use provided base URL %q", conf.BaseURL, baseURLArg)
	}

	if conf.BaseURL != defaultBaseURL {
		infof(c.App.Writer, "Using %q as base URL value", conf.BaseURL)
	}
	baseURL, rpcOpts, err := parseBaseURL(conf.BaseURL, true)
	if err != nil {
		return nil, err
	}

	var authFlow *authFlow
	disableBrowserOpen := c.Bool(loginFlagDisableBrowser)
	if isProdBaseURL(baseURL) {
		authFlow = newCLIAuthFlow(c.App.Writer, disableBrowserOpen)
	} else {
		authFlow = newStgCLIAuthFlow(c.App.Writer, disableBrowserOpen)
	}

	return &viamClient{
		c:           c,
		conf:        conf,
		baseURL:     baseURL,
		rpcOpts:     rpcOpts,
		selectedOrg: &apppb.Organization{},
		selectedLoc: &apppb.Location{},
		authFlow:    authFlow,
	}, nil
}

func (c *viamClient) copyRPCOpts() []rpc.DialOption {
	rpcOpts := make([]rpc.DialOption, len(c.rpcOpts))
	copy(rpcOpts, c.rpcOpts)
	return rpcOpts
}

func (c *viamClient) loadOrganizations() error {
	resp, err := c.client.ListOrganizations(c.c.Context, &apppb.ListOrganizationsRequest{})
	if err != nil {
		return err
	}
	c.orgs = &resp.Organizations
	return nil
}

func (c *viamClient) selectOrganization(orgStr string) error {
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

// getOrg gets an org by an indentifying string. If the orgStr is an
// org UUID, then this matchs on organization ID, otherwise this will match
// on organization name.
func (c *viamClient) getOrg(orgStr string) (*apppb.Organization, error) {
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

// getUserOrgByPublicNamespace searches the logged in users orgs to see
// if any have a matching public namespace.
func (c *viamClient) getUserOrgByPublicNamespace(publicNamespace string) (*apppb.Organization, error) {
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

func (c *viamClient) listOrganizations() ([]*apppb.Organization, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	if err := c.loadOrganizations(); err != nil {
		return nil, err
	}
	return (*c.orgs), nil
}

func (c *viamClient) loadLocations() error {
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

func (c *viamClient) selectLocation(locStr string) error {
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

func (c *viamClient) listLocations(orgID string) ([]*apppb.Location, error) {
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

func (c *viamClient) listRobots(orgStr, locStr string) ([]*apppb.Robot, error) {
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

func (c *viamClient) robot(orgStr, locStr, robotStr string) (*apppb.Robot, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	robots, err := c.listRobots(orgStr, locStr)
	if err != nil {
		return nil, err
	}
	for _, robot := range robots {
		if robot.Id == robotStr || robot.Name == robotStr {
			return robot, nil
		}
	}

	// check if the robot is a cloud robot using the ID
	resp, err := c.client.GetRobot(c.c.Context, &apppb.GetRobotRequest{
		Id: robotStr,
	})
	if err != nil {
		return nil, errors.Errorf("no robot found for %q", robotStr)
	}

	return resp.GetRobot(), nil
}

func (c *viamClient) robotPart(orgStr, locStr, robotStr, partStr string) (*apppb.RobotPart, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	parts, err := c.robotParts(orgStr, locStr, robotStr)
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

func (c *viamClient) robotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool) ([]*apppb.LogEntry, error) {
	part, err := c.robotPart(orgStr, locStr, robotStr, partStr)
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

func (c *viamClient) robotParts(orgStr, locStr, robotStr string) ([]*apppb.RobotPart, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	robot, err := c.robot(orgStr, locStr, robotStr)
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

func (c *viamClient) printRobotPartLogsInner(logs []*apppb.LogEntry, indent string) {
	for _, log := range logs {
		printf(
			c.c.App.Writer,
			"%s%s\t%s\t%s\t%s",
			indent,
			log.Time.AsTime().Format("2006-01-02T15:04:05.000Z0700"),
			log.Level,
			log.LoggerName,
			log.Message,
		)
	}
}

func (c *viamClient) printRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
	logs, err := c.robotPartLogs(orgStr, locStr, robotStr, partStr, errorsOnly)
	if err != nil {
		return err
	}

	if header != "" {
		printf(c.c.App.Writer, header)
	}
	if len(logs) == 0 {
		printf(c.c.App.Writer, "%sNo recent logs", indent)
		return nil
	}
	c.printRobotPartLogsInner(logs, indent)
	return nil
}

// tailRobotPartLogs tails and prints logs for the given robot part.
func (c *viamClient) tailRobotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool, indent, header string) error {
	part, err := c.robotPart(orgStr, locStr, robotStr, partStr)
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
		printf(c.c.App.Writer, header)
	}

	for {
		resp, err := tailClient.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		c.printRobotPartLogsInner(resp.Logs, indent)
	}
}

func (c *viamClient) runRobotPartCommand(
	orgStr, locStr, robotStr, partStr string,
	svcMethod, data string,
	streamDur time.Duration,
	debug bool,
	logger logging.Logger,
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

func (c *viamClient) startRobotPartShell(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	logger logging.Logger,
) error {
	dialCtx, fqdn, rpcOpts, err := c.prepareDial(orgStr, locStr, robotStr, partStr, debug)
	if err != nil {
		return err
	}

	if debug {
		printf(c.c.App.Writer, "Establishing connection...")
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
						fmt.Fprint(c.c.App.Writer, outputData.Output) // no newline
					}
					if outputData.Error != "" {
						fmt.Fprint(c.c.App.ErrWriter, outputData.Error) // no newline
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
