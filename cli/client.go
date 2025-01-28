// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"bufio"
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
	"go.uber.org/multierr"
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
	// yellow is the format string used to output warnings in yellow color.
	yellow = "\033[1;33m%s\033[0m"
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

type disableAuthServiceArgs struct {
	OrgID string
}

// DisableAuthServiceConfirmation is the Before action for 'organizations auth-service disable'.
// It asks for the user to confirm that they want to disable the auth service.
func DisableAuthServiceConfirmation(c *cli.Context, args disableAuthServiceArgs) error {
	if args.OrgID == "" {
		return errors.New("cannot disable auth service without an organization ID")
	}

	printf(c.App.Writer, yellow, "WARNING!!\n")
	printf(c.App.Writer, yellow, fmt.Sprintf("You are trying to disable the auth service for organization ID %s. "+
		"Once disabled, all custom auth views and emails will be removed from your organization's (%s) "+
		"OAuth applications and permanently deleted.\n", args.OrgID, args.OrgID))
	printf(c.App.Writer, yellow, "If you wish to continue, please type \"disable\":")
	if err := c.Err(); err != nil {
		return err
	}

	rawInput, err := bufio.NewReader(c.App.Reader).ReadString('\n')
	if err != nil {
		return err
	}

	if input := strings.ToUpper(strings.TrimSpace(rawInput)); input != "DISABLE" {
		return errors.New("aborted")
	}
	return nil
}

// DisableAuthServiceAction corresponds to 'organizations auth-service disable'.
func DisableAuthServiceAction(cCtx *cli.Context, args disableAuthServiceArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	return c.disableAuthServiceAction(cCtx, args.OrgID)
}

func (c *viamClient) disableAuthServiceAction(cCtx *cli.Context, orgID string) error {
	if orgID == "" {
		return errors.New("cannot disable auth service without an organization ID")
	}

	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if _, err := c.client.DisableAuthService(cCtx.Context, &apppb.DisableAuthServiceRequest{OrgId: orgID}); err != nil {
		return err
	}

	printf(cCtx.App.Writer, "disabled auth service for organization %q:\n", orgID)
	return nil
}

type enableAuthServiceArgs struct {
	OrgID string
}

// EnableAuthServiceAction corresponds to 'organizations auth-service enable'.
func EnableAuthServiceAction(cCtx *cli.Context, args enableAuthServiceArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot enable auth service without an organization ID")
	}

	return c.enableAuthServiceAction(cCtx, args.OrgID)
}

func (c *viamClient) enableAuthServiceAction(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	_, err := c.client.EnableAuthService(cCtx.Context, &apppb.EnableAuthServiceRequest{OrgId: orgID})
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "enabled auth service for organization %q:\n", orgID)
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

type organizationEnableBillingServiceArgs struct {
	OrgID   string
	Address string
}

// OrganizationEnableBillingServiceAction corresponds to `organizations billing enable`.
func OrganizationEnableBillingServiceAction(cCtx *cli.Context, args organizationEnableBillingServiceArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	orgID := args.OrgID
	if orgID == "" {
		return errors.New("cannot enable billing service without an organization ID")
	}

	address := args.Address
	if address == "" {
		return errors.New("cannot enable billing service to an empty address")
	}

	return client.organizationEnableBillingServiceAction(cCtx, orgID, address)
}

func (c *viamClient) organizationEnableBillingServiceAction(cCtx *cli.Context, orgID, addressAsString string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	address, err := parseBillingAddress(addressAsString)
	if err != nil {
		return err
	}

	_, err = c.client.EnableBillingService(cCtx.Context, &apppb.EnableBillingServiceRequest{
		OrgId:          orgID,
		BillingAddress: address,
	})
	if err != nil {
		return err
	}
	printf(cCtx.App.Writer, "Successfully enabled billing service for organization %q", orgID)
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

	logoFile, err := os.Open(filepath.Clean(logoFilePath))
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

type listOAuthAppsArgs struct {
	OrgID string
}

// ListOAuthAppsAction corresponds to `organizations auth-service oauth-app list`.
func ListOAuthAppsAction(cCtx *cli.Context, args listOAuthAppsArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("organization ID is required to list OAuth apps")
	}

	return c.listOAuthAppsAction(cCtx, orgID)
}

func (c *viamClient) listOAuthAppsAction(cCtx *cli.Context, orgID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	resp, err := c.client.ListOAuthApps(cCtx.Context, &apppb.ListOAuthAppsRequest{
		OrgId: orgID,
	})
	if err != nil {
		return err
	}

	if len(resp.ClientIds) == 0 {
		printf(cCtx.App.Writer, "No OAuth apps found for organization %q\n", orgID)
		return nil
	}

	printf(cCtx.App.Writer, "OAuth apps for organization %q:\n", orgID)
	for _, id := range resp.ClientIds {
		printf(cCtx.App.Writer, " - %s\n", id)
	}
	return nil
}

type listLocationsArgs struct {
	Organization string
}

// ListLocationsAction is the corresponding Action for 'locations list'.
func ListLocationsAction(c *cli.Context, args listLocationsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
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
	orgStr := args.Organization
	if orgStr == "" {
		orgStr = c.Args().First()
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

func (c *viamClient) getOrgAndLocationNamesForRobot(ctx context.Context, robot *apppb.Robot) (string, string, error) {
	orgs, err := c.client.GetOrganizationsWithAccessToLocation(
		ctx, &apppb.GetOrganizationsWithAccessToLocationRequest{LocationId: robot.Location},
	)
	if err != nil {
		return "", "", err
	}
	if len(orgs.OrganizationIdentities) == 0 {
		return "", "", errors.Errorf("no parent org found for robot: %s", robot.Id)
	}
	org := orgs.OrganizationIdentities[0]

	location, err := c.client.GetLocation(ctx, &apppb.GetLocationRequest{LocationId: robot.Location})
	if err != nil {
		return "", "", err
	}

	return org.Name, location.Location.Name, nil
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
		orgName, locName, err := client.getOrgAndLocationNamesForRobot(c.Context, robot)
		if err != nil {
			return err
		}
		printf(c.App.Writer, "%s -> %s", orgName, locName)
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
		warningf(c.App.ErrWriter, "Provided negative %q value. Defaulting to %d", generalFlagCount, defaultNumLogs)
		return defaultNumLogs, nil
	}
	if numLogs == 0 {
		return defaultNumLogs, nil
	}
	if numLogs > maxNumLogs {
		return 0, errors.Errorf("provided too high of a %q value. Maximum is %d", generalFlagCount, maxNumLogs)
	}
	return numLogs, nil
}

type robotsLogsArgs struct {
	Organization string
	Location     string
	Machine      string
	Output       string
	Format       string
	Keyword      string
	Levels       []string
	Start        string
	End          string
	Count        int
}

// RobotsLogsAction is the corresponding Action for 'machines logs'.
func RobotsLogsAction(c *cli.Context, args robotsLogsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Check if both start time and count are provided
	// TODO: [APP-7415] Enhance LogsForPart API to Support Sorting Options for Log Display Order
	// TODO: [APP-7450] Implement "Start Time with Count without End Time" Functionality in LogsForPart
	if args.Start != "" && args.Count > 0 && args.End == "" {
		return errors.New("unsupported functionality: specifying both a start time and a count without an end time is not supported. " +
			"This behavior can be counterintuitive because logs are currently only sorted in descending order. " +
			"For example, if there are 200 logs after the specified start time and you request 10 logs, it will return the 10 most recent logs, " +
			"rather than the 10 logs closest to the start time. " +
			"Please provide either a start time and an end time to define a clear range, or a count without a start time for recent logs",
		)
	}

	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
	robot, err := client.robot(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine")
	}

	// TODO(RSDK-9727) - this is a little inefficient insofar as a `robot` is created immediately
	// above and then also again within this `robotParts` call. Might be nice to have a helper
	// API for getting parts when we already have a `Robot`
	parts, err := client.robotParts(orgStr, locStr, robotStr)
	if err != nil {
		return errors.Wrap(err, "could not get machine parts")
	}

	// Determine the output destination
	var writer io.Writer
	if args.Output != "" {
		file, err := os.OpenFile(args.Output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return errors.Wrap(err, "could not open file for writing")
		}
		//nolint:errcheck
		defer file.Close()
		writer = file
	} else {
		// Output to console
		writer = c.App.Writer
	}

	return client.fetchAndSaveLogs(robot, parts, args, writer)
}

// fetchLogs fetches logs for all parts and writes them to the provided writer.
func (c *viamClient) fetchAndSaveLogs(robot *apppb.Robot, parts []*apppb.RobotPart, args robotsLogsArgs, writer io.Writer) error {
	for i, part := range parts {
		// Write a header for text format
		if args.Format == "text" || args.Format == "" {
			// Add robot information as a header for context
			if i == 0 {
				header := fmt.Sprintf("Robot: %s -> Location: %s -> Organization: %s -> Machine: %s\n",
					robot.Name, args.Location, args.Organization, args.Machine)
				if _, err := fmt.Fprintln(writer, header); err != nil {
					return errors.Wrap(err, "failed to write robot header")
				}
			}

			if _, err := fmt.Fprintf(writer, "===== Logs for Part: %s =====\n", part.Name); err != nil {
				return errors.Wrap(err, "failed to write header to writer")
			}
		}

		// Stream logs for the part
		if err := c.streamLogsForPart(part, args, writer); err != nil {
			return errors.Wrapf(err, "could not stream logs for part %s", part.Name)
		}
	}
	return nil
}

// streamLogsForPart streams logs for a specific part directly to a file.
func (c *viamClient) streamLogsForPart(part *apppb.RobotPart, args robotsLogsArgs, writer io.Writer) error {
	maxLogsToFetch, err := getNumLogs(c.c, args.Count)
	if err != nil {
		return err
	}

	startTime, err := parseTimeString(args.Start)
	if err != nil {
		return errors.Wrap(err, "invalid start time format")
	}
	endTime, err := parseTimeString(args.End)
	if err != nil {
		return errors.Wrap(err, "invalid end time format")
	}

	keyword := &args.Keyword

	// Tracks the token for the next page of logs to fetch, allowing pagination through log results.
	var pageToken string

	// Fetch logs in batches and write them to the output.
	for fetchedLogCount := 0; fetchedLogCount < maxLogsToFetch; {
		// We do not request the exact limit specified by the user in the `count` argument because the API enforces a maximum
		// limit of 100 logs per batch fetch. To keep the RDK independent of specific limits imposed by the app API,
		// we always request the next full batch of logs as allowed by the API (currently 100). This approach
		// ensures that if the API limit changes in the future, only the app API logic needs to be updated without requiring
		// changes in the RDK.
		resp, err := c.client.GetRobotPartLogs(c.c.Context, &apppb.GetRobotPartLogsRequest{
			Id:        part.Id,
			Filter:    keyword,
			PageToken: &pageToken,
			Levels:    args.Levels,
			Start:     startTime,
			End:       endTime,
		})
		if err != nil {
			return errors.Wrap(err, "failed to fetch logs")
		}

		// End of pagination if no logs are returned.
		if len(resp.Logs) == 0 {
			break
		}

		// The API may return more logs than the user requested via the `count` argument.
		// This is because the API uses pagination internally and fetches logs in batches.
		// To ensure we do not append more logs than the user requested, we calculate the
		// `remainingLogsNeeded` by subtracting the logs we have already fetched (`logsFetched`)
		// from the total number of logs the user asked for (`numLogs`).
		// If the current batch contains more logs than the remaining needed, we truncate the
		// batch to include only the necessary number of logs.
		// This ensures the output strictly adheres to the `count` limit specified by the user.
		remainingLogsNeeded := maxLogsToFetch - fetchedLogCount
		if remainingLogsNeeded < len(resp.Logs) {
			resp.Logs = resp.Logs[:remainingLogsNeeded]
		}

		for _, log := range resp.Logs {
			formattedLog, err := formatLog(log, part.Name, args.Format)
			if err != nil {
				return errors.Wrap(err, "failed to format log")
			}

			if _, err := fmt.Fprintln(writer, formattedLog); err != nil {
				return errors.Wrap(err, "failed to write log to writer")
			}
		}

		fetchedLogCount += len(resp.Logs)

		// End of pagination if there is no next page token.
		if pageToken = resp.NextPageToken; pageToken == "" {
			break
		}
	}

	return nil
}

// formatLog formats a single log entry based on the specified format.
func formatLog(log *commonpb.LogEntry, partName, format string) (string, error) {
	fieldsString, err := logEntryFieldsToString(log.Fields)
	if err != nil {
		fieldsString = fmt.Sprintf("error formatting fields: %v", err)
	}

	switch format {
	case "json":
		logMap := map[string]interface{}{
			"part":    partName,
			"ts":      log.Time.AsTime().Unix(),
			"time":    log.Time.AsTime().Format(logging.DefaultTimeFormatStr),
			"message": log.Message,
			"level":   log.Level,
			"logger":  log.LoggerName,
			"fields":  fieldsString,
		}
		logJSON, err := json.Marshal(logMap)
		if err != nil {
			return "", errors.Wrap(err, "failed to marshal log to JSON")
		}
		return string(logJSON), nil
	case "text", "":
		return fmt.Sprintf(
			"%s\t%s\t%s\t%s\t%s",
			log.Time.AsTime().Format(logging.DefaultTimeFormatStr),
			log.Level,
			log.LoggerName,
			log.Message,
			fieldsString,
		), nil
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
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
	part, err := client.robotPart(orgStr, locStr, robotStr, args.Part)
	if err != nil {
		return errors.Wrap(err, "could not get machine part")
	}

	if orgStr == "" || locStr == "" || robotStr == "" {
		robot, err := client.robot(orgStr, locStr, part.Robot)
		if err != nil {
			return err
		}
		orgName, locName, err := client.getOrgAndLocationNamesForRobot(c.Context, robot)
		if err != nil {
			return err
		}
		printf(c.App.Writer, "%s -> %s -> %s", orgName, locName, robot.Name)
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
	partStr := args.Part

	var header string
	if orgStr == "" || locStr == "" || robotStr == "" {
		// TODO(RSDK-9727) - this is a little inefficient insofar as a `part` is created immediately
		// here then also again within this `{tail|print}RobotPartLogs` call. Might be nice to have a
		// helper API for getting logs from an already-existing `part`
		part, err := c.robotPart(orgStr, locStr, robotStr, partStr)
		if err != nil {
			return err
		}
		robot, err := c.robot(orgStr, locStr, part.Robot)
		if err != nil {
			return err
		}
		orgName, locName, err := c.getOrgAndLocationNamesForRobot(cCtx.Context, robot)
		if err != nil {
			return err
		}
		header = fmt.Sprintf("%s -> %s -> %s", orgName, locName, robot.Name)
	}
	if args.Tail {
		return c.tailRobotPartLogs(
			orgStr, locStr, robotStr, partStr,
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
		orgStr, locStr, robotStr, partStr,
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

type machinesPartRunArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Data         string
	Stream       time.Duration
	Method       string
}

// MachinesPartRunAction is the corresponding Action for 'machines part run'.
func MachinesPartRunAction(c *cli.Context, args machinesPartRunArgs) error {
	svcMethod := args.Method
	if svcMethod == "" {
		svcMethod = c.Args().First()
	}
	if svcMethod == "" {
		return errors.New("service method required")
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
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
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
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

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	if globalArgs.Debug {
		logger = logging.NewDebugLogger("cli")
	}

	return client.machinesPartCopyFilesAction(c, args, logger)
}

func (c *viamClient) machinesPartCopyFilesAction(
	ctx *cli.Context,
	flagArgs machinesPartCopyFilesArgs,
	logger logging.Logger,
) error {
	args := ctx.Args().Slice()
	if len(args) == 0 {
		return errNoFiles
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

	globalArgs, err := getGlobalArgs(ctx)
	if err != nil {
		return err
	}

	doCopy := func() error {
		if isFrom {
			return c.copyFilesFromMachine(
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

		return c.copyFilesToMachine(
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
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
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

	conf, err := ConfigFromCache(c)
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
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
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
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return nil, err
	}
	conf, err := ConfigFromCache(c)
	if err != nil {
		if !os.IsNotExist(err) {
			debugf(c.App.Writer, globalArgs.Debug, "Cached config parse error: %v", err)
			return nil, errors.New("failed to parse cached config. Please log in again")
		}
		conf = &Config{}
		whichProfile, _ := whichProfile(globalArgs)
		if !globalArgs.DisableProfiles && whichProfile != nil {
			conf.profile = *whichProfile
		}
	}

	// If base URL was not specified, assume cached base URL. If no base URL is
	// cached, assume default base URL.
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
	part, err := c.robotPartInner(orgStr, locStr, robotStr, partStr)
	if err == nil {
		return part, nil
	}

	// if we still haven't found the part, it's possible no robotStr was passed. That's okay
	// so long as the partStr was passed as an ID, so let's try to get the part with just that.
	resp, err2 := c.getRobotPart(partStr)
	if err2 == nil {
		return resp.Part, nil
	}

	return nil, multierr.Combine(err, err2)
}

func (c *viamClient) robotPartInner(orgStr, locStr, robotStr, partStr string) (*apppb.RobotPart, error) {
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

type readOAuthAppArgs struct {
	OrgID    string
	ClientID string
}

const (
	clientAuthenticationPrefix = "CLIENT_AUTHENTICATION_"
	pkcePrefix                 = "PKCE_"
	urlValidationPrefix        = "URL_VALIDATION_"
	enabledGrantPrefix         = "ENABLED_GRANT_"
)

// ReadOAuthAppAction is the corresponding action for 'organizations auth-service oauth-app read'.
func ReadOAuthAppAction(c *cli.Context, args readOAuthAppArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.readOAuthAppAction(c, args.OrgID, args.ClientID)
}

func (c *viamClient) readOAuthAppAction(cCtx *cli.Context, orgID, clientID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	req := &apppb.ReadOAuthAppRequest{OrgId: orgID, ClientId: clientID}
	resp, err := c.client.ReadOAuthApp(c.c.Context, req)
	if err != nil {
		return err
	}

	config := resp.OauthConfig
	printf(cCtx.App.Writer, "OAuth config for client ID %s:", clientID)
	printf(cCtx.App.Writer, "")
	printf(cCtx.App.Writer, "Client Authentication: %s", formatStringForOutput(config.ClientAuthentication.String(),
		clientAuthenticationPrefix))
	printf(cCtx.App.Writer, "PKCE (Proof Key for Code Exchange): %s", formatStringForOutput(config.Pkce.String(), pkcePrefix))
	printf(cCtx.App.Writer, "URL Validation Policy: %s", formatStringForOutput(config.UrlValidation.String(), urlValidationPrefix))
	printf(cCtx.App.Writer, "Logout URL: %s", config.LogoutUri)
	printf(cCtx.App.Writer, "Redirect URLs: %s", strings.Join(config.RedirectUris, ", "))
	if len(config.OriginUris) > 0 {
		printf(cCtx.App.Writer, "Origin URLs: %s", strings.Join(config.OriginUris, ", "))
	}

	var enabledGrants []string
	for _, eg := range config.GetEnabledGrants() {
		enabledGrants = append(enabledGrants, formatStringForOutput(eg.String(), enabledGrantPrefix))
	}
	printf(cCtx.App.Writer, "Enabled Grants: %s", strings.Join(enabledGrants, ", "))

	return nil
}

type deleteOAuthAppArgs struct {
	OrgID    string
	ClientID string
}

// DeleteOAuthAppConfirmation is the Before action for 'organizations auth-service oauth-app delete'.
// It asks for the user to confirm that they want to delete the oauth app.
func DeleteOAuthAppConfirmation(c *cli.Context, args deleteOAuthAppArgs) error {
	if args.OrgID == "" {
		return errors.New("cannot delete oauth app without an organization ID")
	}

	if args.ClientID == "" {
		return errors.New("cannot delete oauth app without a client ID")
	}

	printf(c.App.Writer, yellow, "WARNING!!\n")
	printf(c.App.Writer, yellow, fmt.Sprintf("You are trying to delete an OAuth application with client ID %s. "+
		"Once deleted, any existing apps that rely on this OAuth application will no longer be able to authenticate users.\n", args.ClientID))
	printf(c.App.Writer, yellow, "If you wish to continue, please type \"delete\":")
	if err := c.Err(); err != nil {
		return err
	}

	rawInput, err := bufio.NewReader(c.App.Reader).ReadString('\n')
	if err != nil {
		return err
	}

	input := strings.ToUpper(strings.TrimSpace(rawInput))
	if input != "DELETE" {
		return errors.New("aborted")
	}
	return nil
}

// DeleteOAuthAppAction is the corresponding action for 'oauth-app delete'.
func DeleteOAuthAppAction(c *cli.Context, args deleteOAuthAppArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.deleteOAuthAppAction(c, args.OrgID, args.ClientID)
}

func (c *viamClient) deleteOAuthAppAction(cCtx *cli.Context, orgID, clientID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	req := &apppb.DeleteOAuthAppRequest{
		OrgId:    orgID,
		ClientId: clientID,
	}

	_, err := c.client.DeleteOAuthApp(c.c.Context, req)
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Successfully deleted OAuth application")
	return nil
}

type pkce string

// the valid pkce values.
const (
	PKCEUnspecified                              pkce = "unspecified"
	PKCERequired                                 pkce = "required"
	PKCENotRequired                              pkce = "not_required"
	PKCENotRequiredWhenUsingClientAuthentication pkce = "not_required_when_using_client_authentication"
)

func pkceToProto(stringPKCE string) (apppb.PKCE, error) {
	switch pkce(stringPKCE) {
	case PKCENotRequired:
		return apppb.PKCE_PKCE_NOT_REQUIRED, nil
	case PKCERequired:
		return apppb.PKCE_PKCE_REQUIRED, nil
	case PKCENotRequiredWhenUsingClientAuthentication:
		return apppb.PKCE_PKCE_NOT_REQUIRED_WHEN_USING_CLIENT_AUTHENTICATION, nil
	case PKCEUnspecified:
		return apppb.PKCE_PKCE_UNSPECIFIED, nil
	}
	return apppb.PKCE_PKCE_UNSPECIFIED, errors.Errorf("--%s must be a valid PKCE, got %s. "+
		"See `viam organizations auth-service oauth-app update --help` for supported options",
		oauthAppFlagPKCE, stringPKCE)
}

type clientAuthentication string

// the valid client authentication values.
const (
	ClientAuthenticationUnspecified              clientAuthentication = "unspecified"
	ClientAuthenticationRequired                 clientAuthentication = "required"
	ClientAuthenticationNotRequired              clientAuthentication = "not_required"
	ClientAuthenticationNotRequiredWhenUsingPKCE clientAuthentication = "not_required_when_using_pkce"
)

func clientAuthToProto(clientAuth string) (apppb.ClientAuthentication, error) {
	switch clientAuthentication(clientAuth) {
	case ClientAuthenticationNotRequired:
		return apppb.ClientAuthentication_CLIENT_AUTHENTICATION_NOT_REQUIRED, nil
	case ClientAuthenticationRequired:
		return apppb.ClientAuthentication_CLIENT_AUTHENTICATION_REQUIRED, nil
	case ClientAuthenticationNotRequiredWhenUsingPKCE:
		return apppb.ClientAuthentication_CLIENT_AUTHENTICATION_NOT_REQUIRED_WHEN_USING_PKCE, nil
	case ClientAuthenticationUnspecified:
		return apppb.ClientAuthentication_CLIENT_AUTHENTICATION_UNSPECIFIED, nil
	}
	return apppb.ClientAuthentication_CLIENT_AUTHENTICATION_UNSPECIFIED, errors.Errorf("--%s must be a valid ClientAuthentication, got %s. "+
		"See `viam organizations auth-service oauth-app update --help` for supported options",
		oauthAppFlagClientAuthentication, clientAuth)
}

type urlValidation string

// the accepted url validation values.
const (
	URLValidationUnspecified    urlValidation = "unspecified"
	URLValidationExactMatch     urlValidation = "exact_match"
	URLValidationAllowWildcards urlValidation = "allow_wildcards"
)

func urlValidationToProto(urlValid string) (apppb.URLValidation, error) {
	switch urlValidation(urlValid) {
	case URLValidationAllowWildcards:
		return apppb.URLValidation_URL_VALIDATION_ALLOW_WILDCARDS, nil
	case URLValidationExactMatch:
		return apppb.URLValidation_URL_VALIDATION_EXACT_MATCH, nil
	case URLValidationUnspecified:
		return apppb.URLValidation_URL_VALIDATION_UNSPECIFIED, nil
	}
	return apppb.URLValidation_URL_VALIDATION_UNSPECIFIED, errors.Errorf("--%s must be a valid UrlValidation, got %s. "+
		"See `viam organizations auth-service oauth-app update --help` for supported options",
		oauthAppFlagURLValidation, urlValid)
}

type enabledGrant string

// the accepted enabled grant values.
const (
	EnabledGrantUnspecified       enabledGrant = "unspecified"
	EnabledGrantAuthorizationCode enabledGrant = "authorization_code"
	EnabledGrantImplicit          enabledGrant = "implicit"
	EnabledGrantPassword          enabledGrant = "password"
	EnabledGrantRefreshToken      enabledGrant = "refresh_token"
	EnabledGrantDeviceCode        enabledGrant = "device_code"
)

func enabledGrantToProto(eg string) (apppb.EnabledGrant, error) {
	switch enabledGrant(eg) {
	case EnabledGrantAuthorizationCode:
		return apppb.EnabledGrant_ENABLED_GRANT_AUTHORIZATION_CODE, nil
	case EnabledGrantImplicit:
		return apppb.EnabledGrant_ENABLED_GRANT_IMPLICIT, nil
	case EnabledGrantPassword:
		return apppb.EnabledGrant_ENABLED_GRANT_PASSWORD, nil
	case EnabledGrantRefreshToken:
		return apppb.EnabledGrant_ENABLED_GRANT_REFRESH_TOKEN, nil
	case EnabledGrantDeviceCode:
		return apppb.EnabledGrant_ENABLED_GRANT_DEVICE_CODE, nil
	case EnabledGrantUnspecified:
		return apppb.EnabledGrant_ENABLED_GRANT_UNSPECIFIED, nil
	}
	return apppb.EnabledGrant_ENABLED_GRANT_UNSPECIFIED, errors.Errorf("%s must consist of valid EnabledGrants, got %s. "+
		"See `viam organizations auth-service oauth-app update --help` for supported options",
		oauthAppFlagEnabledGrants, eg)
}

type createOAuthAppArgs struct {
	OrgID                string
	ClientName           string
	ClientAuthentication string
	Pkce                 string
	LogoutURI            string
	UrlValidation        string //nolint:revive,stylecheck
	OriginURIs           []string
	RedirectURIs         []string
	EnabledGrants        []string
}

// CreateOAuthAppAction is the corresponding action for 'oauth-app create'.
func CreateOAuthAppAction(c *cli.Context, args createOAuthAppArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.createOAuthAppAction(c, args)
}

func (c *viamClient) createOAuthAppAction(cCtx *cli.Context, args createOAuthAppArgs) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	config, err := generateOAuthConfig(args.ClientAuthentication, args.Pkce, args.UrlValidation,
		args.LogoutURI, args.OriginURIs, args.RedirectURIs, args.EnabledGrants)
	if err != nil {
		return err
	}

	req := &apppb.CreateOAuthAppRequest{
		OrgId:       args.OrgID,
		ClientName:  args.ClientName,
		OauthConfig: config,
	}

	response, err := c.client.CreateOAuthApp(c.c.Context, req)
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Successfully created OAuth app %s with client ID %s and client secret %s",
		args.ClientName, response.ClientId, response.ClientSecret)
	return nil
}

type updateOAuthAppArgs struct {
	OrgID                string
	ClientID             string
	ClientName           string
	ClientAuthentication string
	Pkce                 string
	LogoutURI            string
	UrlValidation        string //nolint:revive,stylecheck
	OriginURIs           []string
	RedirectURIs         []string
	EnabledGrants        []string
}

// UpdateOAuthAppAction is the corresponding action for 'oauth-app update'.
func UpdateOAuthAppAction(c *cli.Context, args updateOAuthAppArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.updateOAuthAppAction(c, args)
}

func (c *viamClient) updateOAuthAppAction(cCtx *cli.Context, args updateOAuthAppArgs) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	req, err := createUpdateOAuthAppRequest(args)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateOAuthApp(c.c.Context, req)
	if err != nil {
		return err
	}

	printf(cCtx.App.Writer, "Successfully updated OAuth app %s", args.ClientID)
	return nil
}

func generateOAuthConfig(clientAuthentication, pkce, urlValidation, logoutURI string,
	originURIs, redirectURIs, enabledGrants []string,
) (*apppb.OAuthConfig, error) {
	clientAuthProto, err := clientAuthToProto(clientAuthentication)
	if err != nil {
		return nil, err
	}
	pkceProto, err := pkceToProto(pkce)
	if err != nil {
		return nil, err
	}

	urlValidationProto, err := urlValidationToProto(urlValidation)
	if err != nil {
		return nil, err
	}

	egProto, err := enabledGrantsToProto(enabledGrants)
	if err != nil {
		return nil, err
	}

	return &apppb.OAuthConfig{
		ClientAuthentication: clientAuthProto,
		Pkce:                 pkceProto,
		UrlValidation:        urlValidationProto,
		OriginUris:           originURIs,
		RedirectUris:         redirectURIs,
		LogoutUri:            logoutURI,
		EnabledGrants:        egProto,
	}, nil
}

func createUpdateOAuthAppRequest(args updateOAuthAppArgs) (*apppb.UpdateOAuthAppRequest, error) {
	orgID := args.OrgID
	clientID := args.ClientID
	clientName := args.ClientName

	oauthConfig, err := generateOAuthConfig(args.ClientAuthentication, args.Pkce, args.UrlValidation,
		args.LogoutURI, args.OriginURIs, args.RedirectURIs, args.EnabledGrants)
	if err != nil {
		return nil, err
	}
	req := &apppb.UpdateOAuthAppRequest{
		OrgId:       orgID,
		ClientId:    clientID,
		ClientName:  clientName,
		OauthConfig: oauthConfig,
	}
	return req, nil
}

func enabledGrantsToProto(enabledGrants []string) ([]apppb.EnabledGrant, error) {
	if enabledGrants == nil {
		return nil, nil
	}
	var enabledGrantsProto []apppb.EnabledGrant
	for _, eg := range enabledGrants {
		enabledGrant, err := enabledGrantToProto(eg)
		if err != nil {
			return nil, err
		}
		enabledGrantsProto = append(enabledGrantsProto, enabledGrant)
	}
	return enabledGrantsProto, nil
}
