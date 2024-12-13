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
	"path/filepath"
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
	// logoMaxSize is the maximum size of a logo in bytes.
	logoMaxSize = 1024 * 200 // 200 KB
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
func ListOrganizationsAction(cCtx *cli.Context, args emptyArgs) error {
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

type organizationsSupportEmailSetArgs struct {
	OrgID        string
	SupportEmail string
}

// OrganizationsSupportEmailSetAction corresponds to `organizations support-email set`.
func OrganizationsSupportEmailSetAction(cCtx *cli.Context, args organizationsSupportEmailSetArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot set support email without an organization ID")
	}

	supportEmail := args.SupportEmail
	if supportEmail == "" {
		return errors.New("cannot set support email to an empty string")
	}

	return c.organizationsSupportEmailSetAction(cCtx, orgID, supportEmail)
}

func (c *viamClient) organizationsSupportEmailSetAction(cCtx *cli.Context, orgID, supportEmail string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	_, err := c.client.OrganizationSetSupportEmail(c.c.Context, &apppb.OrganizationSetSupportEmailRequest{
		OrgId: orgID,
		Email: supportEmail,
	})
	if err != nil {
		return err
	}
	printf(cCtx.App.Writer, "Successfully set support email for organization %q to %q", orgID, supportEmail)
	return nil
}

type organizationsSupportEmailGetArgs struct {
	OrgID string
}

// OrganizationsSupportEmailGetAction corresponds to `organizations support-email get`.
func OrganizationsSupportEmailGetAction(cCtx *cli.Context, args organizationsSupportEmailGetArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot get support email without an organization ID")
	}

	return c.organizationsSupportEmailGetAction(cCtx, orgID)
}

func (c *viamClient) organizationsSupportEmailGetAction(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	resp, err := c.client.OrganizationGetSupportEmail(c.c.Context, &apppb.OrganizationGetSupportEmailRequest{
		OrgId: orgID,
	})
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Support email for organization %q: %q", orgID, resp.GetEmail())
	return nil
}

type updateBillingServiceArgs struct {
	OrgID   string
	Address string
}

// UpdateBillingServiceAction corresponds to `organizations billing-service update`.
func UpdateBillingServiceAction(cCtx *cli.Context, args updateBillingServiceArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot update billing service without an organization ID")
	}

	address := args.Address
	if address == "" {
		return errors.New("cannot update billing service to an empty address")
	}

	return c.updateBillingServiceAction(cCtx, orgID, address)
}

func (c *viamClient) updateBillingServiceAction(cCtx *cli.Context, orgID, addressAsString string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	address, err := parseBillingAddress(addressAsString)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateBillingService(cCtx.Context, &apppb.UpdateBillingServiceRequest{
		OrgId:          orgID,
		BillingAddress: address,
	})
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Successfully updated billing service for organization %q", orgID)
	printf(cCtx.App.Writer, " --- Billing Address --- ")
	printf(cCtx.App.Writer, "Address Line 1: %s", address.GetAddressLine_1())
	if address.GetAddressLine_2() != "" {
		printf(cCtx.App.Writer, "Address Line 2: %s", address.GetAddressLine_2())
	}
	printf(cCtx.App.Writer, "City: %s", address.GetCity())
	printf(cCtx.App.Writer, "State: %s", address.GetState())
	printf(cCtx.App.Writer, "Postal Code: %s", address.GetZipcode())
	printf(cCtx.App.Writer, "Country: USA")
	return nil
}

type getBillingConfigArgs struct {
	OrgID string
}

// GetBillingConfigAction corresponds to `organizations billing get`.
func GetBillingConfigAction(cCtx *cli.Context, args getBillingConfigArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.getBillingConfig(cCtx, args.OrgID)
}

func (c *viamClient) getBillingConfig(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	resp, err := c.client.GetBillingServiceConfig(cCtx.Context, &apppb.GetBillingServiceConfigRequest{
		OrgId: orgID,
	})
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Billing config for organization: %s", orgID)
	printf(cCtx.App.Writer, "Support Email: %s", resp.GetSupportEmail())
	printf(cCtx.App.Writer, "Billing Dashboard URL: %s", resp.GetBillingDashboardUrl())
	printf(cCtx.App.Writer, "Logo URL: %s", resp.GetLogoUrl())
	printf(cCtx.App.Writer, "")
	printf(cCtx.App.Writer, " --- Billing Address --- ")
	printf(cCtx.App.Writer, "Address Line 1: %s", resp.BillingAddress.GetAddressLine_1())
	if resp.BillingAddress.GetAddressLine_2() != "" {
		printf(cCtx.App.Writer, "Address Line 2: %s", resp.BillingAddress.GetAddressLine_2())
	}
	printf(cCtx.App.Writer, "City: %s", resp.BillingAddress.GetCity())
	printf(cCtx.App.Writer, "State: %s", resp.BillingAddress.GetState())
	printf(cCtx.App.Writer, "Postal Code: %s", resp.BillingAddress.GetZipcode())
	printf(cCtx.App.Writer, "Country: %s", "USA")
	return nil
}

type organizationDisableBillingServiceArgs struct {
	OrgID string
}

// OrganizationDisableBillingServiceAction corresponds to `organizations billing disable`.
func OrganizationDisableBillingServiceAction(cCtx *cli.Context, args organizationDisableBillingServiceArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot disable billing service without an organization ID")
	}
	return client.organizationDisableBillingServiceAction(cCtx, orgID)
}

func (c *viamClient) organizationDisableBillingServiceAction(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if _, err := c.client.DisableBillingService(cCtx.Context, &apppb.DisableBillingServiceRequest{
		OrgId: orgID,
	}); err != nil {
		return errors.WithMessage(err, "could not disable billing service")
	}

	printf(cCtx.App.Writer, "Successfully disabled billing service for organization: %s", orgID)
	return nil
}

type organizationsLogoSetArgs struct {
	OrgID    string
	LogoPath string
}

// OrganizationLogoSetAction corresponds to `organizations logo set`.
func OrganizationLogoSetAction(cCtx *cli.Context, args organizationsLogoSetArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot set logo without an organization ID")
	}
	logoFilePath := args.LogoPath
	if logoFilePath == "" {
		return errors.New("cannot set logo to an empty URL")
	}

	return client.organizationLogoSetAction(cCtx, orgID, logoFilePath)
}

func (c *viamClient) organizationLogoSetAction(cCtx *cli.Context, orgID, logoFilePath string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	// determine whether this is a valid file path on the local system
	logoFilePath = strings.ToLower(filepath.Clean(logoFilePath))

	if len(logoFilePath) < 5 || logoFilePath[len(logoFilePath)-4:] != ".png" {
		return errors.Errorf("%s is not a valid .png file path", logoFilePath)
	}

	logoFile, err := os.Open(logoFilePath)
	if err != nil {
		return errors.WithMessagef(err, "could not open logo file: %s", logoFilePath)
	}
	defer func() {
		if err := logoFile.Close(); err != nil {
			warningf(cCtx.App.ErrWriter, "could not close logo file: %s", err)
		}
	}()

	logoBytes, err := io.ReadAll(logoFile)
	if err != nil {
		return errors.WithMessagef(err, "could not read logo file: %s", logoFilePath)
	}

	if len(logoBytes) > logoMaxSize {
		return errors.Errorf("logo file is too large: %d bytes (max size is 200KB)", len(logoBytes))
	}

	_, err = c.client.OrganizationSetLogo(cCtx.Context, &apppb.OrganizationSetLogoRequest{
		OrgId: orgID,
		Logo:  logoBytes,
	})
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Successfully set the logo for organization %s to logo at file-path: %s",
		orgID, logoFilePath)
	return nil
}

type organizationsLogoGetArgs struct {
	OrgID string
}

// OrganizationsLogoGetAction corresponds to `organizations logo get`.
func OrganizationsLogoGetAction(cCtx *cli.Context, args organizationsLogoGetArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot get logo without an organization ID")
	}

	return c.organizationsLogoGetAction(cCtx, args.OrgID)
}

func (c *viamClient) organizationsLogoGetAction(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	resp, err := c.client.OrganizationGetLogo(cCtx.Context, &apppb.OrganizationGetLogoRequest{
		OrgId: orgID,
	})
	if err != nil {
		return err
	}

	if resp.GetUrl() == "" {
		printf(cCtx.App.Writer, "No logo set for organization %q", orgID)
		return nil
	}

	printf(cCtx.App.Writer, "Logo URL for organization %q: %q", orgID, resp.GetUrl())
	return nil
}

// ListLocationsAction is the corresponding Action for 'locations list'.
func ListLocationsAction(c *cli.Context, args emptyArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	// TODO(RSDK-9288) - this is brittle and inconsistent with how most data is passed.
	// Move this to being a flag (but make sure existing workflows still work!)
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

type listRobotsActionArgs struct {
	Organization string
	Location     string
}

// ListRobotsAction is the corresponding Action for 'machines list'.
func ListRobotsAction(c *cli.Context, args listRobotsActionArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	orgStr := args.Organization
	locStr := args.Location
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

type robotsStatusArgs struct {
	Organization string
	Location     string
	Machine      string
}

// RobotsStatusAction is the corresponding Action for 'machines status'.
func RobotsStatusAction(c *cli.Context, args robotsStatusArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := args.Organization
	locStr := args.Location
	robot, err := client.robot(orgStr, locStr, args.Machine)
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

func getNumLogs(c *cli.Context, numLogs int) (int, error) {
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

type robotsLogsArgs struct {
	Organization string
	Location     string
	Machine      string
	Errors       bool
	Count        int
}

// RobotsLogsAction is the corresponding Action for 'machines logs'.
func RobotsLogsAction(c *cli.Context, args robotsLogsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
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
		numLogs, err := getNumLogs(c, args.Count)
		if err != nil {
			return err
		}
		if err := client.printRobotPartLogs(
			orgStr, locStr, robotStr, part.Id,
			args.Errors,
			"\t",
			header,
			numLogs,
		); err != nil {
			return errors.Wrap(err, "could not print machine logs")
		}
	}

	return nil
}

type robotsPartStatusArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

// RobotsPartStatusAction is the corresponding Action for 'machines part status'.
func RobotsPartStatusAction(c *cli.Context, args robotsPartStatusArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	part, err := client.robotPart(orgStr, locStr, robotStr, args.Part)
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

type robotsPartLogsArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Errors       bool
	Tail         bool
	Count        int
}

// RobotsPartLogsAction is the corresponding Action for 'machines part logs'.
func RobotsPartLogsAction(c *cli.Context, args robotsPartLogsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.robotsPartLogsAction(c, args)
}

func (c *viamClient) robotsPartLogsAction(cCtx *cli.Context, args robotsPartLogsArgs) error {
	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
	robot, err := c.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	var header string
	if orgStr == "" || locStr == "" || robotStr == "" {
		header = fmt.Sprintf("%s -> %s -> %s", c.selectedOrg.Name, c.selectedLoc.Name, robot.Name)
	}
	if args.Tail {
		return c.tailRobotPartLogs(
			orgStr, locStr, robotStr, args.Part,
			args.Errors,
			"",
			header,
		)
	}
	numLogs, err := getNumLogs(cCtx, args.Count)
	if err != nil {
		return err
	}
	return c.printRobotPartLogs(
		orgStr, locStr, robotStr, args.Part,
		args.Errors,
		"",
		header,
		numLogs,
	)
}

type robotsPartRestartArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

// RobotsPartRestartAction is the corresponding Action for 'machines part restart'.
func RobotsPartRestartAction(c *cli.Context, args robotsPartRestartArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.robotPartRestart(c, args)
}

func (c *viamClient) robotPartRestart(cCtx *cli.Context, args robotsPartRestartArgs) error {
	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
	partStr := args.Part

	part, err := c.robotPart(orgStr, locStr, robotStr, partStr)
	if err != nil {
		return err
	}
	_, err = c.client.MarkPartForRestart(cCtx.Context, &apppb.MarkPartForRestartRequest{PartId: part.Id})
	if err != nil {
		return err
	}
	infof(c.c.App.Writer, "Request to restart part sent successfully")
	return nil
}

type robotsPartRunArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Data         string
	Stream       time.Duration
}

// RobotsPartRunAction is the corresponding Action for 'machines part run'.
func RobotsPartRunAction(c *cli.Context, args robotsPartRunArgs) error {
	// TODO(RSDK-9288) - this is brittle and inconsistent with how most data is passed.
	// Move this to being a flag (but make sure existing workflows still work!)
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
	globalArgs := parseStructFromCtx[globalArgs](c)
	if globalArgs.Debug {
		logger = logging.NewDebugLogger("cli")
	}

	return client.runRobotPartCommand(
		args.Organization,
		args.Location,
		args.Machine,
		args.Part,
		svcMethod,
		args.Data,
		args.Stream,
		globalArgs.Debug,
		logger,
	)
}

type robotsPartShellArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

// RobotsPartShellAction is the corresponding Action for 'machines part shell'.
func RobotsPartShellAction(c *cli.Context, args robotsPartShellArgs) error {
	infof(c.App.Writer, "Ensure machine part has a valid shell type service")

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs := parseStructFromCtx[globalArgs](c)
	if globalArgs.Debug {
		logger = logging.NewDebugLogger("cli")
	}

	return client.startRobotPartShell(
		args.Organization,
		args.Location,
		args.Machine,
		args.Part,
		globalArgs.Debug,
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

type machinesPartCopyFilesArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Recursive    bool
	Preserve     bool
}

// MachinesPartCopyFilesAction is the corresponding Action for 'machines part cp'.
func MachinesPartCopyFilesAction(c *cli.Context, args machinesPartCopyFilesArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return machinesPartCopyFilesAction(c, client, args)
}

func machinesPartCopyFilesAction(c *cli.Context, client *viamClient, flagArgs machinesPartCopyFilesArgs) error {
	// TODO(RSDK-9288) - this is brittle and inconsistent with how most data is passed.
	// Move this to being a flag (but make sure existing workflows still work!)
	args := c.Args().Slice()
	if len(args) == 0 {
		return errNoFiles
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs := parseStructFromCtx[globalArgs](c)
	if globalArgs.Debug {
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
				flagArgs.Organization,
				flagArgs.Location,
				flagArgs.Machine,
				flagArgs.Part,
				globalArgs.Debug,
				flagArgs.Recursive,
				flagArgs.Preserve,
				paths,
				destination,
				logger,
			)
		}

		return client.copyFilesToMachine(
			flagArgs.Organization,
			flagArgs.Location,
			flagArgs.Machine,
			flagArgs.Part,
			globalArgs.Debug,
			flagArgs.Recursive,
			flagArgs.Preserve,
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
func CheckUpdateAction(c *cli.Context, args emptyArgs) error {
	globalArgs := parseStructFromCtx[globalArgs](c)
	if globalArgs.Quiet {
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
func VersionAction(c *cli.Context, args emptyArgs) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("error reading build info")
	}
	globalArgs := parseStructFromCtx[globalArgs](c)
	if globalArgs.Debug {
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
		conn, err := net.DialTimeout("tcp", baseURLParsed.Host, 10*time.Second)
		if err != nil {
			return nil, nil, fmt.Errorf("base URL %q (needed for auth) is currently unreachable (%v). "+
				"Ensure URL is valid and you are connected to internet", err.Error(), baseURLParsed.Host)
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

func newViamClientInner(c *cli.Context, disableBrowserOpen bool) (*viamClient, error) {
	conf, err := ConfigFromCache()
	if err != nil {
		if !os.IsNotExist(err) {
			globalArgs := parseStructFromCtx[globalArgs](c)
			debugf(c.App.Writer, globalArgs.Debug, "Cached config parse error: %v", err)
			return nil, errors.New("failed to parse cached config. Please log in again")
		}
		conf = &Config{}
	}

	// If base URL was not specified, assume cached base URL. If no base URL is
	// cached, assume default base URL.
	globalArgs := parseStructFromCtx[globalArgs](c)
	baseURLArg := globalArgs.BaseURL
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

// Creates a new viam client, defaulting to _not_ passing the `disableBrowerOpen` arg (which
// users don't even have an option of setting for any CLI method currently except `Login`).
func newViamClient(c *cli.Context) (*viamClient, error) {
	return newViamClientInner(c, false)
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
						fmt.Fprint(c.c.App.Writer, outputData.Output) //nolint:errcheck // no newline
					}
					if outputData.Error != "" {
						fmt.Fprint(c.c.App.ErrWriter, outputData.Error) //nolint:errcheck // no newline
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
