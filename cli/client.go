// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/huh"
	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/nathan-fiscaletti/consolesize-go"
	"github.com/pkg/errors"
	cron "github.com/robfig/cron/v3"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mlinferencepb "go.viam.com/api/app/mlinference/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/cli/module_generate/modulegen"
	rconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

const (
	rdkReleaseURL = "https://api.github.com/repos/viamrobotics/rdk/releases/latest"
	osWindows     = "windows"
	// defaultNumLogs is the same as the number of logs currently returned by app
	// in a single GetRobotPartLogsResponse.
	defaultNumLogs = 100
	// maxNumLogs is an arbitrary limit used to stop CLI users from overwhelming
	// our logs DB with heavy reads.
	maxNumLogs = 10000
	// logoMaxSize is the maximum size of a logo in bytes.
	logoMaxSize = 1024 * 200 // 200 KB
	// defaultLogStartTime is set to the last 12 hours,
	// logs older than 24 hours are stored in the online archive.
	//
	// 12 hours is a temporary decrease from the matching 24 hour window to
	// avoid an edge case where network latency always triggers an online
	// archive query and causes a "resource usage limit exceeded" error.
	defaultLogStartTime = -12 * time.Hour
	// yellow is the format string used to output warnings in yellow color.
	yellow = "\033[1;33m%s\033[0m"
)

var (
	errNoShellService = errors.New("shell service is not enabled on this machine part")
	ftdcPath          = path.Join("~", ".viam", "diagnostics.data")
)

// viamClient wraps a cli.Context and provides all the CLI command functionality
// needed to talk to the app and data services but not directly to robot parts.
type viamClient struct {
	c                   *cli.Context
	conf                *Config
	client              apppb.AppServiceClient
	dataClient          datapb.DataServiceClient
	packageClient       packagepb.PackageServiceClient
	datasetClient       datasetpb.DatasetServiceClient
	datapipelinesClient datapipelinespb.DataPipelinesServiceClient
	mlTrainingClient    mltrainingpb.MLTrainingServiceClient
	mlInferenceClient   mlinferencepb.MLInferenceServiceClient
	buildClient         buildpb.BuildServiceClient
	baseURL             *url.URL
	authFlow            *authFlow

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
	printf(cCtx.App.Writer, "Country: %s", address.GetCountry())
	return nil
}

type getBillingConfigArgs struct {
	OrgID string
}

// GetBillingConfigAction corresponds to `organizations billing get`.
func GetBillingConfigAction(cCtx *cli.Context, args getBillingConfigArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to get billing config for")
	}
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.getBillingConfig(cCtx, args.OrgID)
}

func (c *viamClient) getBillingConfig(cCtx *cli.Context, orgID string) error {
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
	if resp.BillingAddress.GetCountry() != "" {
		printf(cCtx.App.Writer, "Country: %s", resp.BillingAddress.GetCountry())
	}

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
	if orgStr == "" { // if there's still not an orgStr, then we can fall back to the alphabetically first
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

func printMachinePartStatus(c *cli.Context, parts []*apppb.RobotPart) {
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
}

type machinesPartListArgs struct {
	Organization string
	Location     string
	Machine      string
}

// MachinesPartListAction is the corresponding Action for 'machines part list'.
func MachinesPartListAction(c *cli.Context, args machinesPartListArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	if err = client.ensureLoggedIn(); err != nil {
		return err
	}

	parts, err := client.robotParts(args.Organization, args.Location, args.Machine)
	if err != nil {
		return errors.Wrap(err, "could not get machine parts")
	}

	if len(parts) != 0 {
		printf(c.App.Writer, "Parts:")
	}
	printMachinePartStatus(c, parts)

	return nil
}

type listRobotsActionArgs struct {
	All          bool
	Organization string
	Location     string
}

func printOrgAndLocNames(ctx *cli.Context, orgName, locName string) {
	printf(ctx.App.Writer, "%s -> %s", orgName, locName)
}

func (c *viamClient) listAllRobotsInOrg(ctx *cli.Context, orgStr string) error {
	if err := c.selectOrganization(orgStr); err != nil {
		return err
	}
	locations, err := c.listLocations(c.selectedOrg.Id)
	if err != nil {
		return err
	}

	for _, loc := range locations {
		if loc.RobotCount == 0 {
			continue
		}
		c.selectedLoc = loc
		// when printing all robots in an org, we always want to include org and location
		// info to differentiate _where_ a particular robot is
		printOrgAndLocNames(ctx, c.selectedOrg.Name, loc.Name)
		if err = c.listLocationRobots(ctx, c.selectedOrg.Name, loc.Name); err != nil {
			return err
		}
		printf(ctx.App.Writer, "")
	}

	return nil
}

func (c *viamClient) listLocationRobots(ctx *cli.Context, orgStr, locStr string) error {
	robots, err := c.listRobots(orgStr, locStr)
	if err != nil {
		return errors.Wrap(err, "could not list machines")
	}

	if orgStr == "" || locStr == "" {
		printOrgAndLocNames(ctx, c.selectedOrg.Name, c.selectedLoc.Name)
	}

	for _, robot := range robots {
		parts, err := c.client.GetRobotParts(c.c.Context, &apppb.GetRobotPartsRequest{
			RobotId: robot.Id,
		})
		if err != nil {
			return err
		}
		mainPartID := "<unknown>"
		for _, part := range parts.Parts {
			if part.MainPart {
				mainPartID = part.Id
				break
			}
		}
		printf(ctx.App.Writer, "%s (id: %s) (main part id: %s)", robot.Name, robot.Id, mainPartID)
	}
	return nil
}

func (c *viamClient) lookupMachineByName(name, locStr, orgStr string) (*apppb.Robot, error) {
	if _, err := uuid.Parse(name); err == nil { // a robot ID was passed as the name
		req := apppb.GetRobotRequest{Id: name}
		resp, err := c.client.GetRobot(c.c.Context, &req)
		if err != nil {
			return nil, err
		}
		return resp.Robot, nil
	}
	orgs, err := c.listOrganizations()
	if err != nil {
		return nil, err
	}

	robots := map[string]*apppb.Robot{}

	for _, org := range orgs {
		if orgStr != "" && org.Id != orgStr && org.Name != orgStr {
			continue
		}
		locs, err := c.listLocations(org.Id)
		if err != nil {
			return nil, err
		}
		for _, loc := range locs {
			if locStr != "" && loc.Id != locStr && loc.Name != locStr {
				continue
			}
			if foundRobot, err := c.robot(org.Id, loc.Id, name); err == nil {
				robots[foundRobot.Id] = foundRobot
			}
		}
	}
	if len(robots) == 0 {
		return nil, fmt.Errorf("unable to find robot with name %s", name)
	} else if len(robots) != 1 {
		return nil, fmt.Errorf("multiple robots match %s: %v", name, robots)
	}

	var robot *apppb.Robot
	for _, bot := range robots {
		robot = bot
	}
	return robot, nil
}

func (c *viamClient) lookupLocationID(locStr, orgStr string) (string, error) {
	var err error
	foundLocs := []*apppb.Location{}
	orgs := []*apppb.Organization{}
	if orgStr != "" {
		org, err := c.getOrg(orgStr)
		if err != nil {
			return "", err
		}
		orgs = append(orgs, org)
	} else {
		orgs, err = c.listOrganizations()
		if err != nil {
			return "", err
		}
	}
	for _, org := range orgs {
		// an org has been specified and this isn't it
		if orgStr != "" && orgStr != org.Id && orgStr != org.Name {
			continue
		}
		locs, err := c.listLocations(org.Id)
		if err != nil {
			return "", err
		}

		for _, loc := range locs {
			if locStr == loc.Id || locStr == loc.Name {
				// don't add duplicates which can occur if a location is shared across a user's orgs
				if len(foundLocs) == 0 || loc.Id != foundLocs[0].Id {
					foundLocs = append(foundLocs, loc)
				}
			}
		}
	}

	if len(foundLocs) == 0 {
		var orgAddenda string
		if orgStr != "" {
			orgAddenda = fmt.Sprintf(" in organization %q", orgStr)
		}
		return "", errors.Errorf("no location found for %q%q", locStr, orgAddenda)
	}
	if len(foundLocs) != 1 {
		return "", errors.Errorf("multiple locations match %q: %v", locStr, foundLocs)
	}

	return foundLocs[0].Id, nil
}

type createMachineActionArgs struct {
	Name         string
	Location     string
	Organization string
}

// CreateMachineAction is the corresponding action for 'machines create'.
func CreateMachineAction(c *cli.Context, args createMachineActionArgs) error {
	if args.Location == "" {
		return errors.New("must provide a location to create a machine in")
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	locID, err := client.lookupLocationID(args.Location, args.Organization)
	if err != nil {
		return err
	}

	req := apppb.NewRobotRequest{Name: args.Name, Location: locID}

	resp, err := client.client.NewRobot(c.Context, &req)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "created new machine with id %s", resp.Id)
	return nil
}

type deleteMachineActionArgs struct {
	Machine      string
	Location     string
	Organization string
}

// DeleteMachineAction is the corresponding action for 'machines delete'.
func DeleteMachineAction(c *cli.Context, args deleteMachineActionArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	robot, err := client.lookupMachineByName(args.Machine, args.Location, args.Organization)
	robotID := robot.Id
	if err != nil {
		return err
	}

	req := apppb.DeleteRobotRequest{Id: robotID}
	if _, err = client.client.DeleteRobot(c.Context, &req); err != nil {
		return err
	}

	printf(c.App.Writer, "deleted machine %s", args.Machine)
	return nil
}

type updateMachineActionArgs struct {
	Machine      string
	NewName      string
	NewLocation  string
	Location     string
	Organization string
}

// UpdateMachineAction is the corresponding action for 'machines move'.
func UpdateMachineAction(c *cli.Context, args updateMachineActionArgs) error {
	if args.NewName == "" && args.Location == "" {
		return errors.New("must pass a new name or new location to update the machine")
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	robot, err := client.lookupMachineByName(args.Machine, args.Location, args.Organization)
	if err != nil {
		return err
	}

	id := robot.Id
	locStr := robot.GetLocation()
	currLocation, err := client.client.GetLocation(c.Context, &apppb.GetLocationRequest{LocationId: locStr})
	if err != nil {
		return err
	}

	var orgID string
	for _, org := range currLocation.GetLocation().GetOrganizations() {
		if org.Primary {
			orgID = org.OrganizationId
			break
		}
	}

	newLocID, err := client.lookupLocationID(args.NewLocation, orgID)
	if err != nil {
		return err
	}

	req := apppb.UpdateRobotRequest{Id: id, Location: newLocID, Name: args.NewName}

	if _, err = client.client.UpdateRobot(c.Context, &req); err != nil {
		return err
	}

	printf(c.App.Writer, "updated machine %s", args.Machine)
	return nil
}

// ListRobotsAction is the corresponding Action for 'machines list'.
func ListRobotsAction(c *cli.Context, args listRobotsActionArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	orgStr := args.Organization
	locStr := args.Location
	if args.All {
		return client.listAllRobotsInOrg(c, orgStr)
	}
	return client.listLocationRobots(c, orgStr, locStr)
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
		printOrgAndLocNames(c, orgName, locName)
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

	printMachinePartStatus(c, parts)

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

	if args.Start == "" {
		args.Start = time.Now().Add(defaultLogStartTime).UTC().Format(time.RFC3339)
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

type machinesPartCreateArgs struct {
	PartName     string
	Machine      string
	Location     string
	Organization string
}

func machinesPartCreateAction(c *cli.Context, args machinesPartCreateArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	robot, err := client.lookupMachineByName(args.Machine, args.Location, args.Organization)
	if err != nil {
		return err
	}

	req := apppb.NewRobotPartRequest{PartName: args.PartName, RobotId: robot.Id}

	resp, err := client.client.NewRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "created new machine part with ID %s", resp.PartId)
	return nil
}

type machinesPartDeleteArgs struct {
	Part         string
	Machine      string
	Location     string
	Organization string
}

func machinesPartDeleteAction(c *cli.Context, args machinesPartDeleteArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	req := apppb.DeleteRobotPartRequest{PartId: part.Id}

	_, err = client.client.DeleteRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "successfully deleted part %s (ID: %s)", part.Name, part.Id)
	return nil
}

func resourcesFromPartConfig(config map[string]any, resourceTypePlural string) ([]map[string]any, error) {
	var resources []any
	for k, v := range config {
		if k != resourceTypePlural {
			continue
		}
		r, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("config %s were improperly formatted", resourceTypePlural)
		}

		resources = r
		break
	}

	var typedResources []map[string]any

	for _, r := range resources {
		resource, ok := r.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s config was improperly formatted", resourceTypePlural)
		}
		typedResources = append(typedResources, resource)
	}

	return typedResources, nil
}

func resourceMap(c *cli.Context) map[string]string {
	resources := map[string]string{}

	for _, resource := range modulegen.Resources {
		r := strings.Split(resource, " ")
		if len(r) != 2 {
			printf(c.App.ErrWriter, "warning: resource type %s not properly formatted.", resource)
			continue
		}
		resources[r[0]] = r[1]
	}

	return resources
}

type robotsPartAddResourceArgs struct {
	Part            string
	Machine         string
	Location        string
	Organization    string
	ModelName       string
	Name            string
	ResourceSubtype string
	API             string
}

func robotsPartAddResourceAction(c *cli.Context, args robotsPartAddResourceArgs) error {
	if args.API == "" && args.ResourceSubtype == "" {
		return errors.New("cannot add a resource of unknown subtype; a subtype or fully qualified API triplet must be specified")
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	config := part.RobotConfig.AsMap()

	var resourceType string
	var api string
	if args.API != "" && len(strings.Split(args.API, ":")) == 3 {
		api = args.API
		resourceType = strings.Split(args.API, ":")[1]
	} else {
		if args.API != "" {
			warningf(
				c.App.ErrWriter, "the provided API '%s' is improperly formatted; attempting to infer API from the provided resource subtype %s",
				args.API, args.ResourceSubtype,
			)
		}

		subtype := strings.ReplaceAll(args.ResourceSubtype, "-", "_")
		subtype = strings.ReplaceAll(subtype, " ", "_")
		subtype = strings.ToLower(subtype)

		resourceMap := resourceMap(c)
		resourceType = resourceMap[subtype]
		if resourceType == "" {
			return fmt.Errorf(
				"resource subtype %s is unknown; if you're trying to add a custom resource type then a fully qualified API is necessary",
				subtype,
			)
		}
		api = fmt.Sprintf("rdk:%s:%s", resourceType, subtype)
	}

	// for a custom resource subtype, a user might not follow the format of namespace:type:subtype
	if resourceType != "component" && resourceType != "service" {
		warningf(c.App.ErrWriter, "unknown resource type '%s'. Resource type should be 'component' or 'service'; defaulting to component",
			resourceType,
		)
		resourceType = "component"
	}

	resourceTypePlural := resourceType + "s"
	resources, err := resourcesFromPartConfig(config, resourceTypePlural)
	if err != nil {
		return err
	}

	// ensure no component already exists with the given name
	for _, c := range resources {
		if c["name"] == args.Name {
			return fmt.Errorf("%s with name %s already exists", resourceType, args.Name)
		}
	}

	newResource := map[string]any{
		"name":  args.Name,
		"model": args.ModelName,
		"api":   api,
	}
	resources = append(resources, newResource)
	config[resourceTypePlural] = resources

	pbConfig, err := protoutils.StructToStructPb(config)
	if err != nil {
		return err
	}

	req := apppb.UpdateRobotPartRequest{Id: part.Id, Name: part.Name, RobotConfig: pbConfig}
	_, err = client.client.UpdateRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "successfully added resource %s to part %s", args.Name, args.Part)
	return nil
}

type robotsPartRemoveResourceArgs struct {
	Part         string
	Machine      string
	Location     string
	Organization string
	Name         string
}

func robotsPartRemoveResourceAction(c *cli.Context, args robotsPartRemoveResourceArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	config := part.RobotConfig.AsMap()
	resourceFound := false
	for _, resourceType := range []string{"components", "services"} {
		resources, err := resourcesFromPartConfig(config, resourceType)
		if err != nil {
			return err
		}
		var updatedResources []map[string]any
		for _, c := range resources {
			if c["name"] != args.Name {
				updatedResources = append(updatedResources, c)
			} else {
				resourceFound = true
			}
		}
		config[resourceType] = updatedResources
	}

	if !resourceFound {
		printf(c.App.Writer, "resource %s not found on part %s", args.Name, args.Part)
		return nil
	}

	pbConfig, err := protoutils.StructToStructPb(config)
	if err != nil {
		return err
	}

	req := apppb.UpdateRobotPartRequest{Id: part.Id, Name: part.Name, RobotConfig: pbConfig}
	_, err = client.client.UpdateRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "successfully removed resource %s from part %s", args.Name, args.Part)
	return nil
}

// parseJSONOrFile tries to read input as a file, falls back to parsing as inline JSON
func parseJSONOrFile(input string) (map[string]any, error) {
	var data []byte
	//nolint:gosec
	if fileData, err := os.ReadFile(input); err == nil {
		data = fileData
	} else {
		data = []byte(input)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// validateJobConfig validates the fields of a job config map. When isUpdate is true,
// this is an update-job so not all fields are required.
// partConfig is used to warn about unrecognized resource names.
func validateJobConfig(w io.Writer, jobConfig, partConfig map[string]any, isUpdate bool) error {
	// Validate schedule format if provided (or required for add).
	// Valid values: "continuous", a Go duration (e.g. "5s", "1h30m"), or a cron expression (5-6 fields).
	if schedule, ok := jobConfig["schedule"].(string); ok {
		if err := validateJobSchedule(schedule); err != nil {
			return err
		}
	} else if !isUpdate {
		return errors.New("job config must include 'schedule' field (string)")
	}

	// Validate resource is a non-empty string. Warn if not found in config
	// (could still be a built-in or remote resource).
	if resource, ok := jobConfig["resource"].(string); ok {
		if resource == "" {
			return errors.New("'resource' field must be a non-empty string")
		}
		if !resourceExistsInConfig(partConfig, resource) {
			warningf(w,
				"resource %q not found in part config; job will fail if this resource does not exist on the machine "+
					"(note: built-in and remote resources may not appear in config)",
				resource,
			)
		}
	} else if !isUpdate {
		return errors.New("job config must include 'resource' field (string)")
	}

	// Validate method is a non-empty string
	if method, ok := jobConfig["method"].(string); ok {
		if method == "" {
			return errors.New("'method' field must be a non-empty string")
		}
	} else if !isUpdate {
		return errors.New("job config must include 'method' field (string)")
	}

	// Validate command is a JSON object (map) if provided.
	if command, ok := jobConfig["command"]; ok {
		if _, ok := command.(map[string]any); !ok {
			return errors.New("'command' field must be a JSON object")
		}
	}

	// Validate log_configuration.level if provided.
	// Valid values: "debug", "info", "warn", "warning", "error".
	if logConfig, ok := jobConfig["log_configuration"].(map[string]any); ok {
		if level, ok := logConfig["level"].(string); ok {
			validLevels := map[string]bool{
				"debug": true, "info": true, "warn": true, "warning": true, "error": true,
			}
			if !validLevels[strings.ToLower(level)] {
				return fmt.Errorf("log_configuration level must be one of: debug, info, warn, warning, error; got %q", level)
			}
		}
	}

	return nil
}

// resourceExistsInConfig checks if a resource name exists in the part's components or services.
func resourceExistsInConfig(config map[string]any, name string) bool {
	for _, key := range []string{"components", "services"} {
		if arr, ok := config[key].([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if m["name"] == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// validateJobSchedule checks that schedule is "continuous", a valid Go duration, or a valid cron expression.
// This mirrors the parsing logic in robot/jobmanager/jobmanager.go scheduleJob().
func validateJobSchedule(schedule string) error {
	if strings.ToLower(schedule) == "continuous" {
		return nil
	}

	intErr := validateInterval(schedule)
	if intErr == nil {
		return nil
	}

	cronErr := validateCronExpression(schedule)
	if cronErr == nil {
		return nil
	}

	return errors.Errorf(
		"invalid schedule %q: not a valid interval (%v) or cron expression (%v)",
		schedule, intErr, cronErr,
	)
}

func validateInterval(interval string) error {
	if _, err := time.ParseDuration(interval); err != nil {
		return err
	}
	return nil
}

func validateCronExpression(schedule string) error {
	// Try parsing as cron. Use 6-field (with seconds) parser if there are 6+ fields,
	// otherwise use standard 5-field parser. This matches the jobmanager's behavior.
	withSeconds := len(strings.Fields(schedule)) >= 6
	if withSeconds {
		p := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := p.Parse(schedule); err != nil {
			return err
		}
	} else {
		if _, err := cron.ParseStandard(schedule); err != nil {
			return err
		}
	}
	return nil
}

type machinesPartAddJobArgs struct {
	Part         string
	Machine      string
	Location     string
	Organization string
	Attributes   string
}

func machinesPartAddJobAction(c *cli.Context, args machinesPartAddJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	var jobConfig map[string]any
	var part *apppb.RobotPart

	// If no attributes are provided, run the interactive huh flow.
	if args.Attributes == "" {
		// first, get part id through flag or prompt
		if args.Part == "" {
			var partID string
			partForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Part ID:").
						Description("Run 'viam machines list --all --organization=<org-id>' to see all machines with their part-ids").
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return errors.New("part ID cannot be empty")
							}
							return nil
						}).
						Value(&partID),
				),
			)
			if err := partForm.Run(); err != nil {
				return err
			}
			partID = strings.TrimSpace(partID)
			if partID == "" {
				return errors.New("part ID cannot be empty")
			}

			// Look up the part by ID and store it so we can use its config below.
			resp, err := client.getRobotPart(partID)
			if err != nil {
				return errors.Wrapf(err, "part ID %q not found", partID)
			}
			part = resp.Part
			args.Part = partID
		} else {
			var err error
			part, err = client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
			if err != nil {
				return err
			}
		}

		// 2. Build interactive form from the part config.
		confMap := part.RobotConfig.AsMap()
		var resourceOpts []huh.Option[string]
		for _, key := range []string{"components", "services"} {
			resources, err := resourcesFromPartConfig(confMap, key)
			if err != nil {
				return err
			}
			for _, r := range resources {
				if n, ok := r["name"].(string); ok && n != "" {
					resourceOpts = append(resourceOpts, huh.NewOption(n, n))
				}
			}
		}
		if len(resourceOpts) == 0 {
			return errors.New("This machine contains no components or services")
		}

		// 3. Create the form and run it
		var name, resource, method, commandStr, logLevel, scheduleType string
		form := huh.NewForm(huh.NewGroup(
			huh.NewNote().Title("Add a job to a part"),
			huh.NewInput().Title("Set a job name:").Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("job name cannot be empty")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Select a resource:").Options(resourceOpts...).Value(&resource),
			huh.NewInput().Title("Set a method:").Value(&method).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("method cannot be empty")
					}
					return nil
				}),
			huh.NewInput().
				Title("If using DoCommand, set a command in JSON format (leave empty otherwise):").
				Placeholder("{}").
				Value(&commandStr).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return nil
					}
					var cmd map[string]any
					if err := json.Unmarshal([]byte(s), &cmd); err != nil {
						return errors.Wrap(err, "invalid JSON object")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Set the log threshold:").
				Options(
					huh.NewOption("debug", "debug"),
					huh.NewOption("info", "info"),
					huh.NewOption("warn", "warn"),
					huh.NewOption("error", "error"),
				).
				Value(&logLevel),
			huh.NewSelect[string]().
				Title("Set the schedule type:").
				Options(
					huh.NewOption("Interval", "interval"),
					huh.NewOption("Cron", "cron"),
					huh.NewOption("Continuous", "continuous"),
				).
				Value(&scheduleType),
		))
		if err := form.Run(); err != nil {
			return err
		}

		// 4. last page form loads based on what type of schedule is selected
		var schedule string
		switch scheduleType {
		case "interval":
			var intervalStr string
			form2 := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Set the interval:").
						Description("Valid intervals look like 10s, 1m, 1h1m, etc. (Go duration format).").
						Validate(func(s string) error {
							return validateInterval(s)
						}).
						Value(&intervalStr),
				),
			)
			if err := form2.Run(); err != nil {
				return err
			}
			schedule = intervalStr
		case "cron":
			var cronExpr string
			form2 := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Cron expression:").
						Description("Valid cron expressions look like 0 0 * * * for daily, */5 * * * * * for every 5 seconds, etc...").
						Validate(func(s string) error {
							return validateCronExpression(s)
						}).
						Value(&cronExpr),
				),
			)
			if err := form2.Run(); err != nil {
				return err
			}
			schedule = cronExpr
		default:
			schedule = "continuous"
		}

		// 5. Build the jobConfig map from the interactive inputs.
		jobConfig = map[string]any{
			"name": name, "schedule": schedule, "resource": resource, "method": method,
		}

		if method == "DoCommand" {
			if strings.TrimSpace(commandStr) == "" {
				jobConfig["command"] = map[string]any{}
			} else {
				var cmd map[string]any
				if err := json.Unmarshal([]byte(commandStr), &cmd); err != nil {
					return errors.Wrapf(err, "invalid command JSON")
				}
				jobConfig["command"] = cmd
			}
		}
		if logLevel != "" {
			jobConfig["log_configuration"] = map[string]any{"level": logLevel}
		}
	} else {
		// Non-interactive path: attributes and part are required flags.
		jobConfig, err = parseJSONOrFile(args.Attributes)
		if err != nil {
			return errors.Wrap(err, "failed to parse job config")
		}

		partStr := strings.TrimSpace(args.Part)
		if partStr == "" {
			return errors.New("part is required when using --attributes; specify --part (or --part-id/--part-name)")
		}
		part, err = client.robotPart(args.Organization, args.Location, args.Machine, partStr)
		if err != nil {
			return err
		}
	}

	// Validate required fields and format
	name, ok := jobConfig["name"].(string)
	if !ok || name == "" {
		return errors.New("job config must include 'name' field (string)")
	}

	config := part.RobotConfig.AsMap()
	if err := validateJobConfig(c.App.ErrWriter, jobConfig, config, false); err != nil {
		return err
	}

	// Get existing jobs array or create new one
	var jobs []any
	if existingJobs, ok := config["jobs"]; ok {
		if arr, ok := existingJobs.([]any); ok {
			jobs = arr
		}
	}

	// Check if job with same name exists
	for _, j := range jobs {
		if jobMap, ok := j.(map[string]any); ok {
			if jobMap["name"] == name {
				return fmt.Errorf("job with name %s already exists on part %s", name, part.Name)
			}
		}
	}

	jobs = append(jobs, jobConfig)
	config["jobs"] = jobs

	if err := client.updateRobotPart(part, config); err != nil {
		return err
	}

	printf(c.App.Writer, "successfully added job %s to part %s", name, part.Name)
	return nil
}

type machinesPartUpdateJobArgs struct {
	Part         string
	Machine      string
	Location     string
	Organization string
	Name         string
	Attributes   string
}

func machinesPartUpdateJobAction(c *cli.Context, args machinesPartUpdateJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	newJobConfig, err := parseJSONOrFile(args.Attributes)
	if err != nil {
		return errors.Wrap(err, "failed to parse job config")
	}

	config := part.RobotConfig.AsMap()
	if err := validateJobConfig(c.App.ErrWriter, newJobConfig, config, true); err != nil {
		return err
	}

	var jobs []any
	if existingJobs, ok := config["jobs"]; ok {
		if arr, ok := existingJobs.([]any); ok {
			jobs = arr
		}
	}

	// Find and update the job
	found := false
	for i, j := range jobs {
		if jobMap, ok := j.(map[string]any); ok {
			if jobMap["name"] == args.Name {
				found = true
				// Merge the new config into existing job, keeping the name
				for k, v := range newJobConfig {
					jobMap[k] = v
				}
				jobMap["name"] = args.Name // Ensure name doesn't change
				jobs[i] = jobMap
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("job %s not found on part %s", args.Name, part.Name)
	}

	config["jobs"] = jobs

	if err := client.updateRobotPart(part, config); err != nil {
		return err
	}

	printf(c.App.Writer, "successfully updated job %s on part %s", args.Name, part.Name)
	return nil
}

type machinesPartDeleteJobArgs struct {
	Part         string
	Machine      string
	Location     string
	Organization string
	Name         string
}

func machinesPartDeleteJobAction(c *cli.Context, args machinesPartDeleteJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	config := part.RobotConfig.AsMap()

	var jobs []any
	if existingJobs, ok := config["jobs"]; ok {
		if arr, ok := existingJobs.([]any); ok {
			jobs = arr
		}
	}

	// Filter out the job
	var newJobs []any
	found := false
	for _, j := range jobs {
		if jobMap, ok := j.(map[string]any); ok {
			if jobMap["name"] != args.Name {
				newJobs = append(newJobs, j)
			} else {
				found = true
			}
		} else {
			newJobs = append(newJobs, j)
		}
	}

	if !found {
		return fmt.Errorf("job %s not found on part %s", args.Name, part.Name)
	}

	config["jobs"] = newJobs

	if err := client.updateRobotPart(part, config); err != nil {
		return err
	}

	printf(c.App.Writer, "successfully deleted job %s from part %s", args.Name, part.Name)
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

	printMachinePartStatus(c, []*apppb.RobotPart{part})

	return nil
}

type robotsPartAddFragmentArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Fragment     string
}

// RobotsPartAddFragmentAction is the corresponding action for 'machines part fragments add'
func RobotsPartAddFragmentAction(c *cli.Context, args robotsPartAddFragmentArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	fragmentResp, err := client.client.ListFragments(c.Context, &apppb.ListFragmentsRequest{})
	if err != nil {
		return err
	}

	pbFragments := fragmentResp.Fragments

	var idToAdd, nameToAdd string

	if args.Fragment != "" {
		// Fragment specified, find it by name or ID
		found := false
		for _, fragment := range pbFragments {
			if fragment.Name == args.Fragment || fragment.Id == args.Fragment {
				idToAdd = fragment.Id
				nameToAdd = fragment.Name
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("fragment %s not found", args.Fragment)
		}
	} else {
		// No fragment specified, use fuzzyfinder
		idx, err := fuzzyfinder.Find(pbFragments, func(i int) string { return pbFragments[i].Name })
		if err != nil {
			return err
		}
		idToAdd = pbFragments[idx].Id
		nameToAdd = pbFragments[idx].Name
	}

	conf := part.RobotConfig.AsMap()
	fragments, ok := conf["fragments"].([]any)
	if !ok || fragments == nil {
		fragments = []any{}
	}

	for _, fragment := range fragments {
		fragment := fragment.(map[string]any)
		for k, v := range fragment {
			if k == "id" && v.(string) == idToAdd {
				return fmt.Errorf("fragment %s already exists on part %s", nameToAdd, part.Name)
			}
		}
	}

	newFragment := map[string]any{"id": idToAdd}
	fragments = append(fragments, newFragment)
	conf["fragments"] = fragments
	pbConf, err := protoutils.StructToStructPb(conf)
	if err != nil {
		return err
	}

	req := apppb.UpdateRobotPartRequest{Id: part.Id, Name: part.Name, RobotConfig: pbConf}
	_, err = client.client.UpdateRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "successfully added fragment %s to part %s", nameToAdd, part.Name)
	return nil
}

type robotsPartRemoveFragmentArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Fragment     string
}

// given a map of fragment names to IDs, allows the user to select one and returns the chosen name/ID
func (c *viamClient) selectFragment(fragmentNamesToIDs map[string]string) (string, string, error) {
	huhOptions := []huh.Option[string]{}

	for name := range fragmentNamesToIDs {
		huhOptions = append(huhOptions, huh.NewOption(name, name))
	}
	var selectedFragmentName string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Select a fragment to remove"),
			huh.NewSelect[string]().
				Title("Select a fragment:").
				Options(
					huhOptions...,
				).
				Value(&selectedFragmentName),
		),
	)
	err := form.Run()
	if err != nil {
		return "", "", errors.Wrap(err, "encountered an error in selecting fragment")
	}
	return selectedFragmentName, fragmentNamesToIDs[selectedFragmentName], nil
}

// getFragmentMap returns a map of the given part's fragment names to IDs
func (c *viamClient) getFragmentMap(cCtx *cli.Context, part *apppb.RobotPart) (map[string]string, error) {
	conf := part.GetRobotConfig().AsMap()

	fragments, ok := conf["fragments"].([]any)
	if !ok || len(fragments) == 0 { // there are no fragments on the machine part
		warningf(cCtx.App.ErrWriter, "no fragments found on part %s", part.Name)
		return nil, nil
	}
	fragmentNamesToIDs := map[string]string{}
	for _, fragment := range fragments {
		f := fragment.(map[string]any)
		for _, fragmentID := range f {
			fragmentID := fragmentID.(string)
			req := apppb.GetFragmentRequest{Id: fragmentID}
			fragmentPb, err := c.client.GetFragment(cCtx.Context, &req)
			if err != nil {
				return nil, err
			}
			fragmentNamesToIDs[fragmentPb.Fragment.Name] = fragmentID
		}
	}

	return fragmentNamesToIDs, nil
}

// RobotsPartRemoveFragmentAction is the corresponding action for `machines part fragments remove`
func RobotsPartRemoveFragmentAction(c *cli.Context, args robotsPartRemoveFragmentArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.robotPart(args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}

	fragmentNamesToIDs, err := client.getFragmentMap(c, part)
	if err != nil || fragmentNamesToIDs == nil {
		return err
	}

	var whichFragment, whichID string
	if args.Fragment != "" {
		// Fragment name or ID provided, bypass selection
		var ok bool
		whichID, ok = fragmentNamesToIDs[args.Fragment]
		if ok {
			// Found by name
			whichFragment = args.Fragment
		} else {
			// Check if it's an ID
			for name, id := range fragmentNamesToIDs {
				if id == args.Fragment {
					whichFragment = name
					whichID = args.Fragment
					ok = true
					break
				}
			}
			if !ok {
				return errors.Errorf("fragment %s not found on part %s", args.Fragment, part.Name)
			}
		}
	} else {
		// No fragment provided, prompt user to select
		whichFragment, whichID, err = client.selectFragment(fragmentNamesToIDs)
		if err != nil {
			return err
		}
	}

	conf := part.GetRobotConfig().AsMap()
	oldFragments := conf["fragments"].([]any)
	newFragments := []any{}
	for _, oldFragment := range oldFragments {
		oldF := oldFragment.(map[string]any)
		for _, id := range oldF {
			if id.(string) != whichID {
				newFragments = append(newFragments, oldFragment)
			}
		}
	}

	conf["fragments"] = newFragments
	pbConf, err := protoutils.StructToStructPb(conf)
	if err != nil {
		return err
	}

	req := apppb.UpdateRobotPartRequest{Id: part.Id, Name: part.Name, RobotConfig: pbConf}
	_, err = client.client.UpdateRobotPart(c.Context, &req)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "successfully removed fragment %s from part %s", whichFragment, part.Name)
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
	Component    string
}

// apiToGRPCServiceName converts a resource API to its gRPC service name.
// For example: rdk:component:camera -> viam.component.camera.v1.CameraService
func apiToGRPCServiceName(api resource.API) string {
	// Convert subtype from snake_case to lowercase (remove underscores)
	subtypeLower := strings.ReplaceAll(api.SubtypeName, "_", "")

	// Convert subtype to PascalCase for service name
	// e.g., "movement_sensor" -> "MovementSensor"
	parts := strings.Split(api.SubtypeName, "_")
	var pascalParts []string
	for _, p := range parts {
		if len(p) > 0 {
			pascalParts = append(pascalParts, strings.ToUpper(p[:1])+p[1:])
		}
	}
	subtypePascal := strings.Join(pascalParts, "")

	// Build the full service name
	// Format: viam.<type>.<subtype_no_underscore>.v1.<SubtypePascal>Service
	return fmt.Sprintf("viam.%s.%s.v1.%sService", api.Type.Name, subtypeLower, subtypePascal)
}

// isShortMethodName returns true if the method name is a short form (no dots or slashes).
func isShortMethodName(method string) bool {
	return !strings.Contains(method, ".") && !strings.Contains(method, "/")
}

// mergeComponentNameIntoData merges the component name into the data JSON.
// If data is empty, it creates a new JSON object with just the name.
// If data already has a "name" field, it is preserved (not overwritten).
func mergeComponentNameIntoData(data, componentName string) (string, error) {
	var dataMap map[string]interface{}
	if data == "" {
		dataMap = make(map[string]interface{})
	} else {
		if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
			return "", errors.Wrap(err, "failed to parse --data as JSON")
		}
	}

	// Only set name if not already present
	if _, exists := dataMap["name"]; !exists {
		dataMap["name"] = componentName
	}

	result, err := json.Marshal(dataMap)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize data JSON")
	}
	return string(result), nil
}

// MachinesPartRunAction is the corresponding Action for 'machines part run'.
func MachinesPartRunAction(c *cli.Context, args machinesPartRunArgs) error {
	svcMethod := args.Method
	if svcMethod == "" {
		svcMethod = c.Args().First()
	}
	if svcMethod == "" && args.Component == "" {
		return errors.New("service method required")
	}

	viamClient, err := newViamClient(c)
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

	data := args.Data

	// If component is specified, resolve the method and merge name into data
	if args.Component != "" {
		// Connect to the robot to get resource information
		dialCtx, fqdn, rpcOpts, err := viamClient.prepareDial(
			args.Organization, args.Location, args.Machine, args.Part, globalArgs.Debug)
		if err != nil {
			return err
		}

		robotClient, err := viamClient.connectToRobot(dialCtx, fqdn, rpcOpts, globalArgs.Debug, logger)
		if err != nil {
			return err
		}
		defer func() {
			utils.UncheckedError(robotClient.Close(c.Context))
		}()

		// Find the component by name
		var foundAPI *resource.API
		for _, name := range robotClient.ResourceNames() {
			if name.Name == args.Component {
				apiCopy := name.API
				foundAPI = &apiCopy
				break
			}
		}
		if foundAPI == nil {
			return errors.Errorf("component %q not found on machine", args.Component)
		}

		// If method is a short name, expand it
		if svcMethod == "" {
			return errors.New("method is required when using --component")
		}
		if isShortMethodName(svcMethod) {
			serviceName := apiToGRPCServiceName(*foundAPI)
			svcMethod = fmt.Sprintf("%s.%s", serviceName, svcMethod)
		}

		// Merge component name into data
		data, err = mergeComponentNameIntoData(data, args.Component)
		if err != nil {
			return err
		}
	}

	return viamClient.runRobotPartCommand(
		args.Organization,
		args.Location,
		args.Machine,
		args.Part,
		svcMethod,
		data,
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
	NoProgress   bool
}

type wrongNumArgsError struct {
	have int
	min  int
	max  int
}

func (err wrongNumArgsError) Error() string {
	if err.min != err.max && err.max == 0 {
		noun := "arguments"
		if err.min == 1 {
			noun = "argument"
		}
		return fmt.Sprintf("expected %d %s but got %d", err.min, noun, err.have)
	}
	return fmt.Sprintf("expected %d-%d arguments but got %d", err.min, err.max, err.have)
}

type machinesPartGetFTDCArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

// MachinesPartGetFTDCAction is the corresponding Action for 'machines part get-ftdc'.
func MachinesPartGetFTDCAction(c *cli.Context, args machinesPartGetFTDCArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	logger := globalArgs.createLogger()

	return client.machinesPartGetFTDCAction(c, args, globalArgs.Debug, logger)
}

// MachinesPartCopyFilesAction is the corresponding Action for 'machines part cp'.
func MachinesPartCopyFilesAction(c *cli.Context, args machinesPartCopyFilesArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	logger := globalArgs.createLogger()

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
			// If the user passes a `~` for their machine destination, we should treat
			// it as root rather than trying to create a file named `~` at the root
			if destination == "~" {
				destination = ""
			}
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
	var pm *ProgressManager
	doCopy := func() (int, error) {
		var copyFunc func() error
		if isFrom {
			copyFunc = func() error {
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
		} else {
			copyFunc = func() error {
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
					flagArgs.NoProgress,
				)
			}
		}
		if !flagArgs.NoProgress {
			pm = NewProgressManager([]*Step{
				{ID: "copy", Message: "Copying files...", CompletedMsg: "Files copied", IndentLevel: 0},
			})
			if err := pm.Start("copy"); err != nil {
				return 0, err
			}
			defer pm.Stop()
		}
		attemptCount, err := c.retryableCopy(
			ctx,
			pm,
			copyFunc,
			isFrom,
		)
		return attemptCount, err
	}
	attemptCount, err := doCopy()
	if err != nil {
		defer pm.Fail("copy", err) //nolint:errcheck
		if statusErr := status.Convert(err); statusErr != nil &&
			statusErr.Code() == codes.InvalidArgument &&
			statusErr.Message() == shell.ErrMsgDirectoryCopyRequestNoRecursion {
			return errDirectoryCopyRequestNoRecursion
		}
		return fmt.Errorf("all %d copy attempts failed, try again later", attemptCount)
	}
	if err := pm.Complete("copy"); err != nil {
		return err
	}
	return nil
}

func (c *viamClient) machinesPartGetFTDCAction(
	ctx *cli.Context,
	flagArgs machinesPartGetFTDCArgs,
	debug bool,
	logger logging.Logger,
) error {
	args := ctx.Args().Slice()
	var targetPath string
	switch numArgs := len(args); numArgs {
	case 0:
		var err error
		targetPath, err = os.Getwd()
		if err != nil {
			return err
		}
	case 1:
		targetPath = args[0]
	default:
		return wrongNumArgsError{numArgs, 0, 1}
	}

	part, err := c.robotPart(flagArgs.Organization, flagArgs.Location, flagArgs.Machine, flagArgs.Part)
	if err != nil {
		return err
	}
	// Intentional use of path instead of filepath: Windows understands both / and
	// \ as path separators, and we don't want a cli running on Windows to send
	// a path using \ to a *NIX machine.
	src := path.Join(ftdcPath, part.Id)
	gArgs, err := getGlobalArgs(ctx)
	quiet := err == nil && gArgs != nil && gArgs.Quiet
	var startTime time.Time
	if !quiet {
		startTime = time.Now()
		printf(ctx.App.Writer, "Saving to %s ...", path.Join(targetPath, part.GetId()))
	}
	if err := c.copyFilesFromMachine(
		flagArgs.Organization,
		flagArgs.Location,
		flagArgs.Machine,
		flagArgs.Part,
		debug,
		true,
		false,
		[]string{src},
		targetPath,
		logger,
	); err != nil {
		if statusErr := status.Convert(err); statusErr != nil &&
			statusErr.Code() == codes.InvalidArgument &&
			statusErr.Message() == shell.ErrMsgDirectoryCopyRequestNoRecursion {
			return errDirectoryCopyRequestNoRecursion
		}
		return err
	}
	if !quiet {
		printf(ctx.App.Writer, "Done in %s.", time.Since(startTime))
	}
	return nil
}

type robotsPartTunnelArgs struct {
	Organization    string
	Location        string
	Machine         string
	Part            string
	LocalPort       int
	DestinationPort int
}

// RobotsPartTunnelAction is the corresponding Action for 'machines part tunnel'.
func RobotsPartTunnelAction(c *cli.Context, args robotsPartTunnelArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.robotPartTunnel(c, args)
}

func tunnelTraffic(ctx *cli.Context, robotClient *client.RobotClient, local, dest int) error {
	// don't block tunnel attempt if ListTunnels fails in any way - it may be unimplemented.
	// TODO: early return if ListTunnels fails.
	if tunnels, err := robotClient.ListTunnels(ctx.Context); err == nil {
		allowed := false
		for _, t := range tunnels {
			if t.Port == dest {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.Errorf(
				"tunneling to destination port %v not allowed. "+
					"Please ensure the traffic_tunnel_endpoints configuration is set correctly on the machine.",
				dest,
			)
		}
	}

	li, err := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(local)))
	if err != nil {
		return fmt.Errorf("failed to create listener %w", err)
	}
	infof(ctx.App.Writer, "tunneling connections from local port %v to destination port %v on machine part...", local, dest)
	defer func() {
		if err := li.Close(); err != nil {
			warningf(ctx.App.ErrWriter, "error closing listener: %s", err)
		}
	}()

	var wg sync.WaitGroup
	for {
		if ctx.Err() != nil {
			break
		}
		conn, err := li.Accept()
		if err != nil {
			warningf(ctx.App.ErrWriter, "failed to accept connection: %s", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			// call tunnel once per connection, the connection passed in will be closed
			// by Tunnel.
			if err := robotClient.Tunnel(ctx.Context, conn, dest); err != nil {
				printf(ctx.App.Writer, "error while tunneling connection: %s", err)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (c *viamClient) robotPartTunnel(cCtx *cli.Context, args robotsPartTunnelArgs) error {
	orgStr := args.Organization
	locStr := args.Location
	robotStr := args.Machine
	partStr := args.Part

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs, err := getGlobalArgs(cCtx)
	if err != nil {
		return err
	}
	if globalArgs.Debug {
		logger = logging.NewDebugLogger("cli")
	}

	dialCtx, fqdn, rpcOpts, err := c.prepareDial(orgStr, locStr, robotStr, partStr, globalArgs.Debug)
	if err != nil {
		return err
	}

	robotClient, err := c.connectToRobot(dialCtx, fqdn, rpcOpts, globalArgs.Debug, logger)
	if err != nil {
		return err
	}
	return tunnelTraffic(cCtx, robotClient, args.LocalPort, args.DestinationPort)
}

// checkUpdateResponse holds the values used to hold release information.
type getLatestReleaseResponse struct {
	Name       string `json:"name"`
	TagName    string `json:"tag_name"`
	TarballURL string `json:"tarball_url"`
}

func getLatestRelease() (getLatestReleaseResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp := getLatestReleaseResponse{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rdkReleaseURL, nil)
	if err != nil {
		return getLatestReleaseResponse{}, err
	}

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return getLatestReleaseResponse{}, err
	}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return getLatestReleaseResponse{}, err
	}

	defer utils.UncheckedError(res.Body.Close())
	return resp, nil
}

// getLatestReleaseVersionFunc can be overridden in tests to mock GitHub API calls
var getLatestReleaseVersionFunc = func() (string, error) {
	resp, err := getLatestRelease()
	if err != nil {
		return "", err
	}
	return resp.TagName, err
}

func localVersion() (*semver.Version, error) {
	appVersion := rconfig.Version
	localVersion, err := semver.NewVersion(appVersion)
	if err != nil {
		return nil, errors.New("failed to parse local build version")
	}
	return localVersion, nil
}

func latestVersion() (*semver.Version, error) {
	latestRelease, err := getLatestReleaseVersionFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release information: %w", err)
	}
	latestVersion, err := semver.NewVersion(latestRelease)
	if err != nil {
		return nil, errors.New("failed to parse latest release version")
	}
	return latestVersion, nil
}

func (conf *Config) checkUpdate(c *cli.Context) error {
	var shouldCheckUpdate bool

	// if there has never been a last update check, then we should definitely alert as necessary
	if conf.LastUpdateCheck == "" {
		shouldCheckUpdate = true
	} else {
		lastUpdateCheck, err := time.Parse(time.RFC3339, conf.LastUpdateCheck)
		if err != nil {
			warningf(c.App.ErrWriter, "CLI Update Check: failed to parse last update check: %w", err)
			return nil
		}
		// if we've warned people within the last hour, don't do so again
		shouldCheckUpdate = time.Since(lastUpdateCheck) > time.Hour
	}

	if !shouldCheckUpdate {
		return nil
	}

	// indicate that the most recent check happened now
	if err := conf.updateLastUpdateCheck(); err != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to update config update time: %w", err)
		return nil
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	if globalArgs.Quiet {
		return nil
	}

	latestVersion, err := latestVersion()
	// failure to parse `latestRelease` is expected for local builds; we don't want overly
	// noisy warnings here so only alert in these cases if debug flag is on
	if err != nil && globalArgs.Debug {
		warningf(c.App.ErrWriter, "CLI Update Check: %w", err)
	}
	localVersion, err := localVersion()
	if err != nil && globalArgs.Debug {
		warningf(c.App.ErrWriter, "CLI Update Check: %w", err)
	}
	// we know both the local version and the latest version so we can make a determination
	// from that alone on whether or not to alert users to update
	if localVersion != nil && latestVersion != nil {
		// the local version is out of date, so we know to warn
		if localVersion.LessThan(latestVersion) {
			warningf(c.App.ErrWriter, "CLI Update Check: Your CLI (%s) is out of date. Consider updating to version %s. "+
				"Run 'viam update' or see https://docs.viam.com/cli/#install", localVersion.Original(), latestVersion.Original())
		}
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

	// the local build is more than a week old, so we should warn
	if time.Since(dateCompiled) > time.Hour*24*7 {
		var updateInstructions string
		if latestVersion != nil {
			updateInstructions = fmt.Sprintf(" to version: %s", latestVersion.Original())
		}
		warningf(c.App.ErrWriter, "CLI Update Check: Your CLI is more than a week old. "+
			"New CLI releases happen weekly; consider updating%s. Run 'viam update' or see https://docs.viam.com/cli/#install", updateInstructions)
	}
	return nil
}

// UpdateCLIAction updates the CLI to the latest version.
func UpdateCLIAction(c *cli.Context, args emptyArgs) error {
	// 1. check CLI to see if update needed, if this fails then try update anyways
	latestVersion, latestVersionErr := latestVersion()
	if latestVersionErr != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to get latest release information: %w", latestVersionErr)
	}
	localVersion, localVersionErr := localVersion()
	if localVersionErr != nil {
		warningf(c.App.ErrWriter, "CLI Update Check: failed to get local release information: %w", localVersionErr)
	}
	if localVersion != nil && latestVersion != nil {
		if localVersion.GreaterThanEqual(latestVersion) {
			infof(c.App.Writer, "Your CLI is already up to date (version %s)", localVersion.Original())
			return nil
		}
	}
	// 2. check if cli managed by brew, if so attempt update. If it fails
	// dont continue with binary replacement to avoid putting brew out of sync
	managedByBrew, err := checkAndTryBrewUpdate()
	if err != nil {
		return errors.Errorf("CLI update failed: %v", err)
	}
	// try the binary replacement process because not managed by brew
	if !managedByBrew {
		// 3. get the local version binary path (use full path if no symlinks)
		execPath, err := os.Executable()
		if err != nil {
			return errors.Errorf("CLI update failed: failed to get executable path: %v", err)
		}
		localBinaryPath, err := filepath.EvalSymlinks(execPath)
		if err != nil {
			localBinaryPath = execPath
		}
		directoryPath := filepath.Dir(localBinaryPath)

		// 4. get the latest binary (from storage.googleapis.com) and write it into a temp file
		binaryURL := binaryURL()
		latestBinaryPath, err := downloadBinaryIntoDir(binaryURL, directoryPath)
		defer os.Remove(latestBinaryPath) //nolint:errcheck
		if err != nil {
			return errors.Errorf("CLI update failed: failed to download binary: %v", err)
		}

		// 5. replace the old binary with the new one
		if err := replaceBinary(localBinaryPath, latestBinaryPath); err != nil {
			return errors.Errorf("CLI update failed: failed to replace binary: %v", err)
		}
	}
	infof(c.App.Writer, "Your CLI has been successfully updated")
	return nil
}

func checkAndTryBrewUpdate() (bool, error) {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			// Check if viam is actually managed by brew
			err := exec.Command("brew", "list", "viam").Run()
			if err == nil {
				// viam is managed by brew - try upgrade
				out, err := exec.Command("brew", "upgrade", "viam").CombinedOutput()
				if err == nil {
					if strings.Contains(string(out), "already installed") {
						// edge case: latest version released but brew has not updated yet
						return false, errors.New("the latest version is not on brew yet")
					}
					return true, nil
				}
				return false, errors.Errorf("failed to upgrade CLI via brew: %v", err)
			}
		}
	}
	return false, nil
}

func binaryURL() string {
	// Determine binary URL based on OS and architecture
	binaryURL := "https://storage.googleapis.com/packages.viam.com/apps/viam-cli/viam-cli-stable-" +
		runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == osWindows {
		binaryURL += ".exe" //nolint:goconst
	}
	return binaryURL
}

func downloadBinaryIntoDir(binaryURL, directoryPath string) (string, error) {
	// Download the binary
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, binaryURL, nil)
	if err != nil {
		return "", errors.Errorf("binary download failed: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Errorf("binary download failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		utils.UncheckedError(resp.Body.Close())
		return "", errors.Errorf("binary download failed: server returned status %d", resp.StatusCode)
	}

	// create a temp file that we write the downloaded binary into
	goos := runtime.GOOS
	tempFileName := "viam-cli-update"
	if goos == osWindows {
		tempFileName += ".exe"
	}
	tempFileName += ".new"

	// Create the temp file in the same directory as the binary
	latestBinaryPath := filepath.Join(directoryPath, tempFileName)
	latestBinaryFile, err := os.Create(latestBinaryPath) //nolint:gosec
	if err != nil {
		utils.UncheckedError(resp.Body.Close())
		if os.IsPermission(err) {
			if goos == osWindows {
				return "", errors.New("permission denied: run PowerShell as Administrator")
			}
			return "", errors.New("permission denied: run 'sudo viam update'")
		}
		return "", errors.Errorf("failed to create temp file: %v", err)
	}

	// Write downloaded content to temp file
	_, err = io.Copy(latestBinaryFile, resp.Body)
	utils.UncheckedError(resp.Body.Close())
	utils.UncheckedError(latestBinaryFile.Close())
	if err != nil {
		return "", errors.Errorf("failed to write downloaded binary: %v", err)
	}

	// Make executable on Unix-like systems, if permissions are improperly set then no
	// change will be made and user has to run sudo
	if goos != osWindows {
		if err := os.Chmod(latestBinaryPath, 0o755); err != nil { //nolint:gosec
			if os.IsPermission(err) {
				return "", errors.New("permission denied: run 'sudo viam update'")
			}
			return "", errors.Errorf("failed to make binary executable: %v", err)
		}
	}
	return latestBinaryPath, nil
}

func replaceBinary(localBinaryPath, latestBinaryPath string) error {
	if err := os.Rename(latestBinaryPath, localBinaryPath); err != nil {
		if os.IsPermission(err) {
			if runtime.GOOS == osWindows {
				return errors.New("permission denied: run PowerShell as Administrator")
			}
			return errors.New("permission denied: run 'sudo viam update'")
		}
		return errors.Errorf("failed to replace binary: %v", err)
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

func isProdBaseURL(baseURL *url.URL) bool {
	return strings.HasSuffix(baseURL.Hostname(), "viam.com")
}

// Creates a new viam client, defaulting to _not_ passing the `disableBrowerOpen` arg (which
// users don't even have an option of setting for any CLI method currently except `Login`).
func newViamClient(c *cli.Context) (*viamClient, error) {
	client, err := newViamClientInner(c, false)
	if err != nil {
		return nil, err
	}
	if err := client.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return client, nil
}

func newViamClientInner(c *cli.Context, disableBrowserOpen bool) (*viamClient, error) {
	baseURL, conf, err := getBaseURL(c)
	if err != nil {
		return nil, err
	}

	if err = conf.checkUpdate(c); err != nil {
		warningf(c.App.ErrWriter, "Failed to check for CLI updates: %w", err)
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

func getBaseURL(c *cli.Context) (*url.URL, *Config, error) {
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return nil, nil, err
	}
	conf, err := ConfigFromCache(c)
	if err != nil {
		if !os.IsNotExist(err) {
			debugf(c.App.Writer, globalArgs.Debug, "Cached config parse error: %v", err)
			return nil, nil, errors.New("failed to parse cached config. Please log in again")
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
		return nil, nil, fmt.Errorf("cached base URL for this session is %q. "+
			"Please logout and login again to use provided base URL %q", conf.BaseURL, baseURLArg)
	}

	if conf.BaseURL != defaultBaseURL {
		infof(c.App.ErrWriter, "Using %q as base URL value", conf.BaseURL)
	}
	baseURL, _, err := rutils.ParseBaseURL(conf.BaseURL, true)
	if err != nil {
		return nil, nil, err
	}

	return baseURL, conf, nil
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
	if err := c.selectOrganization(orgID); err != nil {
		return nil, err
	}
	if err := c.loadLocations(); err != nil {
		return nil, err
	}
	return (*c.locs), nil
}

func (c *viamClient) listRobots(orgStr, locStr string) ([]*apppb.Robot, error) {
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
	return c.client.GetRobotPart(c.c.Context, &apppb.GetRobotPartRequest{Id: partID})
}

func (c *viamClient) updateRobotPart(part *apppb.RobotPart, confMap map[string]any) error {
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

func (c *viamClient) connectToRobot(
	dialCtx context.Context,
	fqdn string,
	rpcOpts []rpc.DialOption,
	debug bool,
	logger logging.Logger,
) (*client.RobotClient, error) {
	if debug {
		printf(c.c.App.Writer, "Establishing connection...")
	}
	robotClient, err := client.New(dialCtx, fqdn, logger, client.WithDialOptions(rpcOpts...))
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to machine part")
	}
	return robotClient, nil
}

func (c *viamClient) connectToShellServiceInner(
	dialCtx context.Context,
	fqdn string,
	rpcOpts []rpc.DialOption,
	debug bool,
	logger logging.Logger,
) (shell.Service, func(ctx context.Context) error, error) {
	robotClient, err := c.connectToRobot(dialCtx, fqdn, rpcOpts, debug, logger)
	if err != nil {
		return nil, nil, err
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

// maxCopyAttempts is the number of times to retry copying files to a part before giving up.
const maxCopyAttempts = 6

// retryableCopy attempts to copy files to a part using the shell service with retries.
// It handles progress manager updates for each attempt and provides helpful error messages.
// The copyFunc parameter allows for mocking in tests.
// returns number of attempts made in case it terminates early due to nonretryable error
func (c *viamClient) retryableCopy(
	ctx *cli.Context,
	pm *ProgressManager,
	copyFunc func() error,
	isFrom bool,
) (int, error) {
	var hadPreviousFailure bool
	var copyErr error

	for attempt := 1; attempt <= maxCopyAttempts; attempt++ {
		// If we had a previous failure, create a nested step for this retry
		var attemptStepID string
		if hadPreviousFailure {
			attemptStepID = fmt.Sprintf("Attempt-%d", attempt)
			attemptStep := &Step{
				ID:           attemptStepID,
				Message:      fmt.Sprintf("Attempt %d/%d...", attempt, maxCopyAttempts),
				CompletedMsg: fmt.Sprintf("Attempt %d succeeded", attempt),
				Status:       StepPending,
				IndentLevel:  2, // Nested under "copy" which is at level 1
			}
			pm.steps = append(pm.steps, attemptStep)
			pm.stepMap[attemptStepID] = attemptStep

			if err := pm.Start(attemptStepID); err != nil {
				return attempt, err
			}
		}

		copyErr = copyFunc()

		if copyErr == nil {
			// Success! Complete the step if this was a retry
			if attemptStepID != "" {
				if err := pm.Complete(attemptStepID); err != nil {
					return attempt, err
				}
			}
			return attempt, nil
		}

		// Handle error
		hadPreviousFailure = true

		// Print special warning for invalid argument and permission denied errors (in addition to regular error)
		if s, ok := status.FromError(copyErr); ok {
			if s.Code() == codes.PermissionDenied {
				if isFrom {
					warningf(ctx.App.ErrWriter, "RDK couldn't read the source files on the machine. "+
						"Try copying from a path the RDK user can read (e.g., $HOME, /tmp), "+
						"temporarily changing file permissions with 'chmod'.")
				} else {
					warningf(ctx.App.ErrWriter, "RDK couldn't write to the default file copy destination. "+
						"If you're running as non-root, try adding --home $HOME or --home /user/username to your CLI command. "+
						"Alternatively, run the RDK as root.")
				}
				_ = pm.Fail(attemptStepID, copyErr) //nolint:errcheck
				return attempt, copyErr
			} else if s.Code() == codes.InvalidArgument {
				warningf(ctx.App.ErrWriter, "Copy failed with invalid argument: %s", copyErr.Error())
				_ = pm.Fail(attemptStepID, copyErr) //nolint:errcheck
				return attempt, copyErr
			}
		}

		// Create a step for this failed attempt (so it shows in the output)
		if attemptStepID == "" {
			// First attempt - create its step retroactively
			attemptStepID = "Attempt-1"
			attemptStep := &Step{
				ID:           attemptStepID,
				Message:      fmt.Sprintf("Attempt 1/%d...", maxCopyAttempts),
				CompletedMsg: "Attempt 1 succeeded",
				Status:       StepPending,
				IndentLevel:  2,
			}
			pm.steps = append(pm.steps, attemptStep)
			pm.stepMap[attemptStepID] = attemptStep
			if err := pm.Start(attemptStepID); err != nil {
				return attempt, err
			}
		}

		// Mark this attempt as failed (this will print the error on next line)
		_ = pm.Fail(attemptStepID, copyErr) //nolint:errcheck
	}

	// All attempts failed - return the error from the copy function
	return maxCopyAttempts, copyErr
}

func (c *viamClient) copyFilesToMachine(
	orgStr, locStr, robotStr, partStr string,
	debug bool,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
	logger logging.Logger,
	noProgress bool,
) error {
	shellSvc, closeClient, err := c.connectToShellService(orgStr, locStr, robotStr, partStr, debug, logger)
	if err != nil {
		return err
	}
	return c.copyFilesToMachineInner(shellSvc, closeClient, allowRecursion, preserve, paths, destination, noProgress)
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
	noProgress bool,
) error {
	shellSvc, closeClient, err := c.connectToShellServiceFqdn(fqdn, debug, logger)
	if err != nil {
		return err
	}
	return c.copyFilesToMachineInner(shellSvc, closeClient, allowRecursion, preserve, paths, destination, noProgress)
}

// copyFilesToMachineInner is the common logic for both copyFiles variants.
func (c *viamClient) copyFilesToMachineInner(
	shellSvc shell.Service,
	closeClient func(ctx context.Context) error,
	allowRecursion bool,
	preserve bool,
	paths []string,
	destination string,
	noProgress bool,
) error {
	defer func() {
		utils.UncheckedError(closeClient(c.c.Context))
	}()

	if noProgress {
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

	// Calculate total size of all files to be copied
	var totalSize int64
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() && allowRecursion {
			err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					totalSize += info.Size()
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if !info.IsDir() {
			totalSize += info.Size()
		}
	}

	// Create a progress tracking function
	var currentFile string
	progressFunc := func(bytes int64, file string, fileSize int64) {
		if file != currentFile {
			if currentFile != "" {
				//nolint:errcheck // progress display is non-critical
				_, _ = os.Stdout.WriteString("\n")
			}
			currentFile = file
			//nolint:errcheck // progress display is non-critical
			_, _ = os.Stdout.WriteString(fmt.Sprintf("Copying %s...\n", file))
		}
		uploadPercent := int(math.Ceil(100 * float64(bytes) / float64(fileSize)))
		//nolint:errcheck // progress display is non-critical
		_, _ = os.Stdout.WriteString(fmt.Sprintf("\rProgress: %d%% (%d/%d bytes)", uploadPercent, bytes, fileSize))
	}

	// Wrap the copy factory to track progress
	progressFactory := &progressTrackingFactory{
		factory:    shell.NewCopyFileToMachineFactory(destination, preserve, shellSvc),
		onProgress: progressFunc,
	}

	// Create a new read copier with the progress tracking factory
	readCopier, err := shell.NewLocalFileReadCopier(paths, allowRecursion, false, progressFactory)
	if err != nil {
		return err
	}
	defer func() {
		if err := readCopier.Close(c.c.Context); err != nil {
			utils.UncheckedError(err)
		}
	}()

	// ReadAll the files into the copier.
	err = readCopier.ReadAll(c.c.Context)
	return err
}

// progressTrackingFactory wraps a copy factory to track progress.
type progressTrackingFactory struct {
	factory    shell.FileCopyFactory
	onProgress func(int64, string, int64)
}

func (ptf *progressTrackingFactory) MakeFileCopier(ctx context.Context, sourceType shell.CopyFilesSourceType) (shell.FileCopier, error) {
	copier, err := ptf.factory.MakeFileCopier(ctx, sourceType)
	if err != nil {
		return nil, err
	}
	return &progressTrackingCopier{
		copier:     copier,
		onProgress: ptf.onProgress,
	}, nil
}

// progressTrackingCopier wraps a file copier to track progress.
type progressTrackingCopier struct {
	copier     shell.FileCopier
	onProgress func(int64, string, int64)
}

func (ptc *progressTrackingCopier) Copy(ctx context.Context, file shell.File) error {
	// Get file size
	info, err := file.Data.Stat()
	if err != nil {
		return err
	}
	fileSize := info.Size()

	// Create a progress tracking reader
	progressReader := &progressReader{
		reader:     file.Data,
		onProgress: ptc.onProgress,
		fileName:   file.RelativeName,
		fileSize:   fileSize,
	}

	// Create a new file with the progress tracking reader
	progressFile := shell.File{
		RelativeName: file.RelativeName,
		Data:         progressReader,
	}

	return ptc.copier.Copy(ctx, progressFile)
}

func (ptc *progressTrackingCopier) Close(ctx context.Context) error {
	//nolint:errcheck // progress display is non-critical
	_, _ = os.Stdout.WriteString("\n")
	return ptc.copier.Close(ctx)
}

// progressReader wraps a reader to track progress.
type progressReader struct {
	reader     fs.File
	onProgress func(int64, string, int64)
	copied     int64
	fileName   string
	fileSize   int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.copied += int64(n)
		pr.onProgress(pr.copied, pr.fileName, pr.fileSize)
	}
	return n, err
}

func (pr *progressReader) Stat() (fs.FileInfo, error) {
	return pr.reader.Stat()
}

func (pr *progressReader) Close() error {
	return pr.reader.Close()
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

	if args.OrgID == "" {
		return errors.New("must provide an organization ID to read OAuth app")
	}

	return client.readOAuthAppAction(c, args.OrgID, args.ClientID)
}

func (c *viamClient) readOAuthAppAction(cCtx *cli.Context, orgID, clientID string) error {
	req := &apppb.ReadOAuthAppRequest{OrgId: orgID, ClientId: clientID}
	resp, err := c.client.ReadOAuthApp(c.c.Context, req)
	if err != nil {
		return err
	}

	config := resp.OauthConfig
	printf(cCtx.App.Writer, "OAuth config for client ID %s:", clientID)
	printf(cCtx.App.Writer, "")
	printf(cCtx.App.Writer, "Client Name: %s", resp.ClientName)
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

	if args.OrgID == "" {
		return errors.New("must provide an organization ID to create an OAuth app")
	}

	return client.createOAuthAppAction(c, args)
}

func (c *viamClient) createOAuthAppAction(cCtx *cli.Context, args createOAuthAppArgs) error {
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
	if args.OrgID == "" {
		return nil, errors.New("must provide an organization ID to update OAuth app")
	}
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
