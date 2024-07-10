// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/nathan-fiscaletti/consolesize-go"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	rconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
)

const (
	rdkReleaseURL = "https://api.github.com/repos/viamrobotics/rdk/releases/latest"
	// defaultNumLogs is the same as the number of logs currently returned by app
	// in a single GetRobotPartLogsResponse.
	defaultNumLogs = 100
	// maxNumLogs is an arbitrary limit used to stop CLI users from overwhelming
	// our logs DB with heavy reads.
	maxNumLogs = 10000
)

var errNoShellService = errors.New("shell service is not enabled on this machine part")

// viamClient wraps a cli.Context and provides all the CLI command functionality
// needed to talk to the app and data services but not directly to robot parts.
type viamClient struct {
	c                *cli.Context
	conf             *Config
	client           apppb.AppServiceClient
	dataClient       datapb.DataServiceClient
	packageClient    packagepb.PackageServiceClient
	datasetClient    datasetpb.DatasetServiceClient
	endUserClient    apppb.EndUserServiceClient
	mlTrainingClient mltrainingpb.MLTrainingServiceClient
	buildClient      buildpb.BuildServiceClient
	baseURL          *url.URL
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

// ListRobotsAction is the corresponding Action for 'machines list'.
func ListRobotsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robots, err := client.listRobots(orgStr, locStr)
	if err != nil {
		return errors.Wrap(err, "could not list machines")
	}

	if orgStr == "" || locStr == "" {
		printf(c.App.Writer, "%s -> %s", client.selectedOrg.Name, client.selectedLoc.Name)
	}

	for _, robot := range robots {
		printf(c.App.Writer, "%s (id: %s)", robot.Name, robot.Id)
	}
	return nil
}

// RobotsStatusAction is the corresponding Action for 'machines status'.
func RobotsStatusAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robot, err := client.robot(orgStr, locStr, c.String(machineFlag))
	if err != nil {
		return err
	}
	parts, err := client.robotParts(client.selectedOrg.Id, client.selectedLoc.Id, robot.Id)
	if err != nil {
		return errors.Wrap(err, "could not get machine parts")
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

func getNumLogs(c *cli.Context) (int, error) {
	numLogs := c.Int(logsFlagCount)
	if numLogs < 0 {
		warningf(c.App.ErrWriter, "Provided negative %q value. Defaulting to %d", logsFlagCount, defaultNumLogs)
		return defaultNumLogs, nil
	}
	if numLogs == 0 {
		return defaultNumLogs, nil
	}
	if numLogs > maxNumLogs {
		return 0, errors.Errorf("provided too high of a %q value. Maximum is %d", logsFlagCount, maxNumLogs)
	}
	return numLogs, nil
}

// RobotsLogsAction is the corresponding Action for 'machines logs'.
func RobotsLogsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robotStr := c.String(machineFlag)
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	parts, err := client.robotParts(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine parts")
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
		numLogs, err := getNumLogs(c)
		if err != nil {
			return err
		}
		if err := client.printRobotPartLogs(
			orgStr, locStr, robotStr, part.Id,
			c.Bool(logsFlagErrors),
			"\t",
			header,
			numLogs,
		); err != nil {
			return errors.Wrap(err, "could not print machine logs")
		}
	}

	return nil
}

// RobotsPartStatusAction is the corresponding Action for 'machines part status'.
func RobotsPartStatusAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := c.String(organizationFlag)
	locStr := c.String(locationFlag)
	robotStr := c.String(machineFlag)
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	part, err := client.robotPart(orgStr, locStr, robotStr, c.String(partFlag))
	if err != nil {
		return errors.Wrap(err, "could not get machine part")
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

// RobotsPartLogsAction is the corresponding Action for 'machines part logs'.
func RobotsPartLogsAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.robotsPartLogsAction(c)
}

func (c *viamClient) robotsPartLogsAction(cCtx *cli.Context) error {
	orgStr := cCtx.String(organizationFlag)
	locStr := cCtx.String(locationFlag)
	robotStr := cCtx.String(machineFlag)
	robot, err := c.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	var header string
	if orgStr == "" || locStr == "" || robotStr == "" {
		header = fmt.Sprintf("%s -> %s -> %s", c.selectedOrg.Name, c.selectedLoc.Name, robot.Name)
	}
	if cCtx.Bool(logsFlagTail) {
		return c.tailRobotPartLogs(
			orgStr, locStr, robotStr, cCtx.String(partFlag),
			cCtx.Bool(logsFlagErrors),
			"",
			header,
		)
	}
	numLogs, err := getNumLogs(cCtx)
	if err != nil {
		return err
	}
	return c.printRobotPartLogs(
		orgStr, locStr, robotStr, cCtx.String(partFlag),
		cCtx.Bool(logsFlagErrors),
		"",
		header,
		numLogs,
	)
}

// RobotsPartRunAction is the corresponding Action for 'machines part run'.
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
		c.String(machineFlag),
		c.String(partFlag),
		svcMethod,
		c.String(runFlagData),
		c.Duration(runFlagStream),
		c.Bool(debugFlag),
		logger,
	)
}

// RobotsPartShellAction is the corresponding Action for 'machines part shell'.
func RobotsPartShellAction(c *cli.Context) error {
	infof(c.App.Writer, "Ensure machine part has a valid shell type service")

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
		c.String(machineFlag),
		c.String(partFlag),
		c.Bool(debugFlag),
		logger,
	)
}

var (
	errNoFiles                         = errors.New("must provide files to copy")
	errLastArgOfFromMissing            = errors.New("expected last argument to be <copy to path>")
	errLastArgOfToMissing              = errors.New("expected last argument to be machine:<copy to path>")
	errDirectoryCopyRequestNoRecursion = errors.New("file is a directory but copy recursion not used (you can use -r to enable this)")
)

type copyFromPathInvalidError struct {
	path string
}

func (err copyFromPathInvalidError) Error() string {
	return fmt.Sprintf("expected argument %q to be machine:<copy from path>", err.path)
}

// MachinesPartCopyFilesAction is the corresponding Action for 'machines part cp'.
func MachinesPartCopyFilesAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return machinesPartCopyFilesAction(c, client)
}

func machinesPartCopyFilesAction(c *cli.Context, client *viamClient) error {
	args := c.Args().Slice()
	if len(args) == 0 {
		return errNoFiles
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	if c.Bool(debugFlag) {
		logger = logging.NewDebugLogger("cli")
	}

	// the general format is
	// from:
	//		cp machine:path1 machine:path2 ... local_destination
	// to:
	//		cp path1 path2 ... remote_destination
	// we just need to look for machine: to determine what the user's intent is
	determineDirection := func(args []string) (isFrom bool, destination string, paths []string, err error) {
		const machinePrefix = "machine:"
		isFrom = strings.HasPrefix(args[0], machinePrefix)

		if strings.HasPrefix(args[len(args)-1], machinePrefix) {
			if isFrom {
				return false, "", nil, errLastArgOfFromMissing
			}
		} else if !isFrom {
			return false, "", nil, errLastArgOfToMissing
		}

		destination = args[len(args)-1]
		if !isFrom {
			destination = strings.TrimPrefix(destination, machinePrefix)
		}

		// all but the last arg are what we are copying to/from
		for _, arg := range args[:len(args)-1] {
			if isFrom && !strings.HasPrefix(arg, machinePrefix) {
				return false, "", nil, copyFromPathInvalidError{arg}
			}
			if isFrom {
				arg = strings.TrimPrefix(arg, machinePrefix)
			}
			paths = append(paths, arg)
		}
		return
	}

	isFrom, destination, paths, err := determineDirection(args)
	if err != nil {
		return err
	}

	doCopy := func() error {
		if isFrom {
			return client.copyFilesFromMachine(
				c.String(organizationFlag),
				c.String(locationFlag),
				c.String(machineFlag),
				c.String(partFlag),
				c.Bool(debugFlag),
				c.Bool(cpFlagRecursive),
				c.Bool(cpFlagPreserve),
				paths,
				destination,
				logger,
			)
		}

		return client.copyFilesToMachine(
			c.String(organizationFlag),
			c.String(locationFlag),
			c.String(machineFlag),
			c.String(partFlag),
			c.Bool(debugFlag),
			c.Bool(cpFlagRecursive),
			c.Bool(cpFlagPreserve),
			paths,
			destination,
			logger,
		)
	}
	if err := doCopy(); err != nil {
		if statusErr := status.Convert(err); statusErr != nil &&
			statusErr.Code() == codes.InvalidArgument &&
			statusErr.Message() == shell.ErrMsgDirectoryCopyRequestNoRecursion {
			return errDirectoryCopyRequestNoRecursion
		}
		return err
	}
	return nil
}

// checkUpdateResponse holds the values used to hold release information.
type getLatestReleaseResponse struct {
	Name       string `json:"name"`
	TagName    string `json:"tag_name"`
	TarballURL string `json:"tarball_url"`
}

func getLatestReleaseVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp := getLatestReleaseResponse{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rdkReleaseURL, nil)
	if err != nil {
		return "", err
	}

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return "", err
	}

	defer utils.UncheckedError(res.Body.Close())
	return resp.TagName, err
}

// CheckUpdateAction is the corresponding Action for 'check-update'.
func CheckUpdateAction(c *cli.Context) error {
	if c.Bool(quietFlag) {
		return nil
	}

	dateCompiledRaw := rconfig.DateCompiled

	// `go build` will not set the compilation flags needed for this check
	if dateCompiledRaw == "" {
		return nil
	}

	dateCompiled, err := time.Parse("2006-01-02", dateCompiledRaw)
	if err != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to parse compilation date: %w", err)
		return nil
	}

	// install is less than six weeks old
	if time.Since(dateCompiled) < time.Hour*24*7*6 {
		return nil
	}

	conf, err := ConfigFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			utils.UncheckedError(err)
			return nil
		}
		conf = &Config{}
	}

	var lastCheck time.Time
	if conf.LastUpdateCheck == "" {
		conf.LastUpdateCheck = time.Now().Format("2006-01-02")
	} else {
		lastCheck, err = time.Parse("2006-01-02", conf.LastUpdateCheck)
		if err != nil {
			warningf(c.App.ErrWriter, "CLI Update Check: failed to parse date of last check: %w", err)
			return nil
		}
	}

	// The latest version info is cached to limit api calls to once every three days
	if time.Since(lastCheck) < time.Hour*24*3 && conf.LatestVersion != "" {
		warningf(c.App.ErrWriter, "CLI Update Check: Your CLI is more than 6 weeks old. "+
			"Consider updating to version: %s", conf.LatestVersion)
		return nil
	}

	latestRelease, err := getLatestReleaseVersion()
	if err != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to get latest release information: %w", err)
		return nil
	}

	latestVersion, err := semver.NewVersion(latestRelease)
	if err != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to parse latest version: %w", err)
		return nil
	}

	conf.LatestVersion = latestVersion.String()

	err = storeConfigToCache(conf)
	if err != nil {
		utils.UncheckedError(err)
	}

	appVersion := rconfig.Version
	if appVersion == "" {
		warningf(c.App.ErrWriter, "CLI Update Check: Your CLI is more than 6 weeks old. "+
			"Consider updating to version: %s", latestVersion.Original())
		return nil
	}

	localVersion, err := semver.NewVersion(appVersion)
	if err != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to parse compiled version: %w", err)
		return nil
	}

	if localVersion.LessThan(latestVersion) {
		warningf(c.App.ErrWriter, "CLI Update Check: Your CLI is out of date. Consider updating to version %s. "+
			"See https://docs.viam.com/cli/#install", latestVersion.Original())
	}

	return nil
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
	conf, err := ConfigFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			debugf(c.App.Writer, c.Bool(debugFlag), "Cached config parse error: %v", err)
			return nil, errors.New("failed to parse cached config. Please log in again")
		}
		conf = &Config{}
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
		infof(c.App.ErrWriter, "Using %q as base URL value", conf.BaseURL)
	}
	baseURL, _, err := parseBaseURL(conf.BaseURL, true)
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
		selectedOrg: &apppb.Organization{},
		selectedLoc: &apppb.Location{},
		authFlow:    authFlow,
	}, nil
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
		return nil, errors.Errorf("no machine found for %q", robotStr)
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

	// if we can't find the part via org/location, see if this is an id, and try to find it directly that way
	if robotStr != "" {
		resp, err := c.client.GetRobotParts(c.c.Context, &apppb.GetRobotPartsRequest{
			RobotId: robotStr,
		})
		if err != nil {
			return nil, err
		}
		for _, part := range resp.Parts {
			if part.Id == partStr || part.Name == partStr {
				return part, nil
			}
		}
		if partStr == "" && len(resp.Parts) == 1 {
			return resp.Parts[0], nil
		}
	}

	return nil, errors.Errorf("no machine part found for machine: %q part: %q", robotStr, partStr)
}

// getRobotPart wraps GetRobotPart API.
// note: overlaps with viamClient.robotPart, which wraps GetRobotParts.
// Use this variant if you don't know the robot ID.
func (c *viamClient) getRobotPart(partID string) (*apppb.GetRobotPartResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.client.GetRobotPart(c.c.Context, &apppb.GetRobotPartRequest{Id: partID})
}

func (c *viamClient) updateRobotPart(part *apppb.RobotPart, confMap map[string]any) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	confStruct, err := structpb.NewStruct(confMap)
	if err != nil {
		return errors.Wrap(err, "in NewStruct")
	}
	req := apppb.UpdateRobotPartRequest{
		Id:          part.Id,
		Name:        part.Name,
		RobotConfig: confStruct,
	}
	_, err = c.client.UpdateRobotPart(c.c.Context, &req)
	return err
}

func (c *viamClient) robotPartLogs(orgStr, locStr, robotStr, partStr string, errorsOnly bool,
	numLogs int,
) ([]*commonpb.LogEntry, error) {
	part, err := c.robotPart(orgStr, locStr, robotStr, partStr)
	if err != nil {
		return nil, err
	}

	// Use page tokens to get batches of 100 up to numLogs and throw away any
	// extra logs in last batch.
	logs := make([]*commonpb.LogEntry, 0, numLogs)
	var pageToken string
	for i := 0; i < numLogs; {
		resp, err := c.client.GetRobotPartLogs(c.c.Context, &apppb.GetRobotPartLogsRequest{
			Id:         part.Id,
			ErrorsOnly: errorsOnly,
			PageToken:  &pageToken,
		})
		if err != nil {
			return nil, err
		}

		pageToken = resp.NextPageToken
		// Break in the event of no logs in GetRobotPartLogsResponse or when
		// page token is empty (no more pages).
		if resp.Logs == nil || pageToken == "" {
			break
		}

		// Truncate this intermediate slice of resp.Logs based on how many logs
		// are still required by numLogs.
		remainingLogsNeeded := numLogs - i
		if remainingLogsNeeded < len(resp.Logs) {
			resp.Logs = resp.Logs[:remainingLogsNeeded]
		}
		logs = append(logs, resp.Logs...)

		i += len(resp.Logs)
	}

	return logs, nil
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

func (c *viamClient) printRobotPartLogsInner(logs []*commonpb.LogEntry, indent string) {
	// Iterate over logs in reverse because they are returned in
	// order of latest to oldest but we should print from oldest -> newest
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		fieldsString, err := logEntryFieldsToString(log.Fields)
		if err != nil {
			warningf(c.c.App.ErrWriter, "%v", err)
			fieldsString = ""
		}
		printf(
			c.c.App.Writer,
			"%s%s\t%s\t%s\t%s\t%s",
			indent,
			log.Time.AsTime().Format(logging.DefaultTimeFormatStr),
			log.Level,
			log.LoggerName,
			log.Message,
			fieldsString,
		)
	}
}

func (c *viamClient) printRobotPartLogs(orgStr, locStr, robotStr, partStr string,
	errorsOnly bool, indent, header string, numLogs int,
) error {
	logs, err := c.robotPartLogs(orgStr, locStr, robotStr, partStr, errorsOnly, numLogs)
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

func (c *viamClient) connectToShellService(orgStr, locStr, robotStr, partStr string,
	debug bool,
	logger logging.Logger,
) (shell.Service, func(ctx context.Context) error, error) {
	dialCtx, fqdn, rpcOpts, err := c.prepareDial(orgStr, locStr, robotStr, partStr, debug)
	if err != nil {
		return nil, nil, err
	}
	return c.connectToShellServiceInner(dialCtx, fqdn, rpcOpts, debug, logger)
}

// connectToShellServiceFqdn is a shell service dialer that doesn't check org or re-fetch the part.
func (c *viamClient) connectToShellServiceFqdn(
	partFqdn string,
	debug bool,
	logger logging.Logger,
) (shell.Service, func(ctx context.Context) error, error) {
	dialCtx, fqdn, rpcOpts, err := c.prepareDialInner(partFqdn, debug)
	if err != nil {
		return nil, nil, err
	}
	return c.connectToShellServiceInner(dialCtx, fqdn, rpcOpts, debug, logger)
}

func (c *viamClient) connectToShellServiceInner(
	dialCtx context.Context,
	fqdn string,
	rpcOpts []rpc.DialOption,
	debug bool,
	logger logging.Logger,
) (shell.Service, func(ctx context.Context) error, error) {
	if debug {
		printf(c.c.App.Writer, "Establishing connection...")
	}
	robotClient, err := client.New(dialCtx, fqdn, logger, client.WithDialOptions(rpcOpts...))
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not connect to machine part")
	}

	var successful bool
	defer func() {
		if !successful {
			utils.UncheckedError(robotClient.Close(c.c.Context))
		}
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
		return nil, nil, errNoShellService
	}

	shellRes, err := robotClient.ResourceByName(*found)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get shell service from machine part")
	}

	shellSvc, ok := shellRes.(shell.Service)
	if !ok {
		return nil, nil, errors.New("could not get shell service from machine part")
	}
	successful = true
	return shellSvc, robotClient.Close, nil
}

func (c *viamClient) startRobotPartShell(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	logger logging.Logger,
) error {
	shellSvc, closeClient, err := c.connectToShellService(orgStr, locStr, robotStr, partStr, debug, logger)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(closeClient(c.c.Context))
	}()

	getWinChMsg := func() map[string]interface{} {
		cols, rows := consolesize.GetConsoleSize()
		return map[string]interface{}{
			"message": "window-change",
			"cols":    cols,
			"rows":    rows,
		}
	}

	input, inputOOB, output, err := shellSvc.Shell(c.c.Context, map[string]interface{}{
		"messages": []interface{}{getWinChMsg()},
	})
	if err != nil {
		return err
	}

	if sig, ok := sigwinchSignal(); ok {
		winchCh := make(chan os.Signal, 1)
		signal.Notify(winchCh, sig)
		utils.PanicCapturingGo(func() {
			defer close(inputOOB)
			for {
				if !utils.SelectContextOrWaitChan(c.c.Context, winchCh) {
					return
				}
				select {
				case <-c.c.Context.Done():
					return
				case inputOOB <- getWinChMsg():
				}
			}
		})
	}

	setRaw := func(isRaw bool) error {
		// NOTE(benjirewis): Linux systems seem to need both "raw" (no processing) and "-echo"
		// (no echoing back inputted characters) in order to allow the input and output loops
		// below to completely control the terminal.
		args := []string{"raw", "-echo", "-echoctl"}
		if !isRaw {
			args = []string{"-raw", "echo", "echoctl"}
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

func (c *viamClient) copyFilesToMachine(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
	logger logging.Logger,
) error {
	shellSvc, closeClient, err := c.connectToShellService(orgStr, locStr, robotStr, partStr, debug, logger)
	if err != nil {
		return err
	}
	return c.copyFilesToMachineInner(shellSvc, closeClient, allowRecursion, preserve, paths, destination)
}

// copyFilesToFqdn is a copyFilesToMachine variant that makes use of pre-fetched part FQDN.
func (c *viamClient) copyFilesToFqdn(
	fqdn string,
	debug bool,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
	logger logging.Logger,
) error {
	shellSvc, closeClient, err := c.connectToShellServiceFqdn(fqdn, debug, logger)
	if err != nil {
		return err
	}
	return c.copyFilesToMachineInner(shellSvc, closeClient, allowRecursion, preserve, paths, destination)
}

// copyFilesToMachineInner is the common logic for both copyFiles variants.
func (c *viamClient) copyFilesToMachineInner(
	shellSvc shell.Service,
	closeClient func(ctx context.Context) error,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
) error {
	defer func() {
		utils.UncheckedError(closeClient(c.c.Context))
	}()

	// prepare a factory that understands the file copying service (RPC or not).
	copyFactory := shell.NewCopyFileToMachineFactory(destination, preserve, shellSvc)
	// make a reader copier that just does the traversal and copy work for us. Think of
	// this as a tee reader.
	readCopier, err := shell.NewLocalFileReadCopier(paths, allowRecursion, false, copyFactory)
	if err != nil {
		return err
	}
	defer func() {
		if err := readCopier.Close(c.c.Context); err != nil {
			utils.UncheckedError(err)
		}
	}()

	// ReadAll the files into the copier.
	return readCopier.ReadAll(c.c.Context)
}

func (c *viamClient) copyFilesFromMachine(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
	logger logging.Logger,
) error {
	shellSvc, closeClient, err := c.connectToShellService(orgStr, locStr, robotStr, partStr, debug, logger)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(closeClient(c.c.Context))
	}()

	// prepare a factory that understands how to work with our local filesystem.
	factory, err := shell.NewLocalFileCopyFactory(destination, preserve, false)
	if err != nil {
		return err
	}

	// let the shell service figure out how to grab the files for and pass them to our copier.
	return shellSvc.CopyFilesFromMachine(c.c.Context, paths, allowRecursion, preserve, factory, nil)
}

func logEntryFieldsToString(fields []*structpb.Struct) (string, error) {
	// if there are no fields, don't return anything, otherwise we add lots of {}'s
	// to the logs
	if len(fields) == 0 {
		return "", nil
	}
	// we have to manually format these fields as json because
	// marshalling a go object will not preserve the order of the fields
	message := "{"
	for i, field := range fields {
		key, value, err := logging.FieldKeyAndValueFromProto(field)
		if err != nil {
			return "", err
		}
		if i > 0 {
			// split fields with space and comma after first entry
			message += ", "
		}
		if _, isStr := value.(string); isStr {
			message += fmt.Sprintf("%q: %q", key, value)
		} else {
			message += fmt.Sprintf("%q: %v", key, value)
		}
	}
	return message + "}", nil
}
