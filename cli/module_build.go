package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	buildpb "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/utils"
)

type jobStatus string

const (
	// In other places in the codebase, "unspecified" fits the established code patterns,
	// however, "Unknown" is more obvious to the user that their build is in an error / strange state.
	jobStatusUnspecified jobStatus = "Unknown"
	// In other places in the codebase, "in progress" fits the established code patterns,
	// however, in the cli, we want this to be a single word so that it is easier
	// to use unix tools on the output.
	jobStatusInProgress jobStatus = "Building"
	jobStatusFailed     jobStatus = "Failed"
	jobStatusDone       jobStatus = "Done"
)

var moduleBuildPollingInterval = 2 * time.Second

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.moduleBuildStartAction(cCtx)
}

func (c *viamClient) moduleBuildStartAction(cCtx *cli.Context) error {
	manifest, err := loadManifest(cCtx.String(moduleFlagPath))
	if err != nil {
		return err
	}
	version := cCtx.String(moduleBuildFlagVersion)
	if manifest.Build == nil || manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}

	// Clean the version argument to ensure compatibility with github tag standards
	version = strings.TrimPrefix(version, "v")

	platforms := manifest.Build.Arch
	if len(platforms) == 0 {
		platforms = defaultBuildInfo.Arch
	}

	gitRef := cCtx.String(moduleBuildFlagRef)
	res, err := c.startBuild(manifest.URL, gitRef, manifest.ModuleID, platforms, version)
	if err != nil {
		return err
	}
	// Print to stderr so that the buildID is the only thing in stdout
	printf(cCtx.App.ErrWriter, "Started build:")
	printf(cCtx.App.Writer, res.BuildId)
	return nil
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(cCtx *cli.Context) error {
	manifestPath := cCtx.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	return moduleBuildLocalAction(cCtx, &manifest)
}

func moduleBuildLocalAction(cCtx *cli.Context, manifest *moduleManifest) error {
	if manifest.Build == nil || manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}
	infof(cCtx.App.Writer, "Starting build")
	processConfig := pexec.ProcessConfig{
		Name:      "bash",
		OneShot:   true,
		Log:       true,
		LogWriter: cCtx.App.Writer,
	}
	// Required logger for the ManagedProcess. Not used
	logger := logging.NewLogger("x")
	if manifest.Build.Setup != "" {
		infof(cCtx.App.Writer, "Starting setup step: %q", manifest.Build.Setup)
		processConfig.Args = []string{"-c", manifest.Build.Setup}
		proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
		if err := proc.Start(cCtx.Context); err != nil {
			return err
		}
	}
	infof(cCtx.App.Writer, "Starting build step: %q", manifest.Build.Build)
	processConfig.Args = []string{"-c", manifest.Build.Build}
	proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
	if err := proc.Start(cCtx.Context); err != nil {
		return err
	}
	infof(cCtx.App.Writer, "Completed build")
	return nil
}

// ModuleBuildListAction lists the module's build jobs.
func ModuleBuildListAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.moduleBuildListAction(cCtx)
}

func (c *viamClient) moduleBuildListAction(cCtx *cli.Context) error {
	var buildIDFilter *string
	var moduleIDFilter string
	// This will use the build id if present and fall back on the module manifest if not
	if cCtx.IsSet(moduleBuildFlagBuildID) {
		filter := cCtx.String(moduleBuildFlagBuildID)
		buildIDFilter = &filter
	} else {
		manifestPath := cCtx.String(moduleBuildFlagPath)
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return err
		}
		moduleID, err := parseModuleID(manifest.ModuleID)
		if err != nil {
			return err
		}
		moduleIDFilter = moduleID.String()
	}
	var numberOfJobsToReturn *int32
	if cCtx.IsSet(moduleBuildFlagCount) {
		count := int32(cCtx.Int(moduleBuildFlagCount))
		numberOfJobsToReturn = &count
	}
	jobs, err := c.listModuleBuildJobs(moduleIDFilter, numberOfJobsToReturn, buildIDFilter)
	if err != nil {
		return err
	}
	// table format rules:
	// minwidth, tabwidth, padding int, padchar byte, flags uint
	w := tabwriter.NewWriter(cCtx.App.Writer, 5, 4, 1, ' ', 0)
	tableFormat := "%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, tableFormat, "ID", "PLATFORM", "STATUS", "VERSION", "TIME")
	for _, job := range jobs.Jobs {
		fmt.Fprintf(w,
			tableFormat,
			job.BuildId,
			job.Platform,
			jobStatusFromProto(job.Status),
			job.Version,
			job.StartTime.AsTime().Format(time.RFC3339))
	}
	// the table is not printed to stdout until the tabwriter is flushed
	//nolint: errcheck,gosec
	w.Flush()
	return nil
}

// anyFailed returns a useful error based on which platforms failed, or nil if all good.
func buildError(statuses map[string]jobStatus) error {
	failedPlatforms := utils.FilterMap(
		statuses,
		func(_ string, s jobStatus) bool { return s != jobStatusDone },
	)
	if len(failedPlatforms) == 0 {
		return nil
	}
	return fmt.Errorf("some platforms failed to build: %s", strings.Join(maps.Keys(failedPlatforms), ", "))
}

// ModuleBuildLogsAction retrieves the logs for a specific build step.
func ModuleBuildLogsAction(c *cli.Context) error {
	buildID := c.String(moduleBuildFlagBuildID)
	platform := c.String(moduleBuildFlagPlatform)
	shouldWait := c.Bool(moduleBuildFlagWait)
	groupLogs := c.Bool(moduleBuildFlagGroupLogs)

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	var statuses map[string]jobStatus
	if shouldWait {
		statuses, err = client.waitForBuildToFinish(buildID, platform)
		if err != nil {
			return err
		}
	}
	if platform != "" {
		if err := client.printModuleBuildLogs(buildID, platform); err != nil {
			return err
		}
	} else {
		platforms, err := client.getPlatformsForModuleBuild(buildID)
		if err != nil {
			return err
		}
		var combinedErr error
		for _, platform := range platforms {
			if groupLogs {
				statusEmoji := "❓"
				switch statuses[platform] { //nolint: exhaustive
				case jobStatusDone:
					statusEmoji = "✅"
				case jobStatusFailed:
					statusEmoji = "❌"
				}
				printf(os.Stdout, "::group::{%s %s}", statusEmoji, platform)
			}
			infof(c.App.Writer, "Logs for %q", platform)
			err := client.printModuleBuildLogs(buildID, platform)
			if err != nil {
				combinedErr = multierr.Combine(combinedErr, client.printModuleBuildLogs(buildID, platform))
			}
			if groupLogs {
				printf(os.Stdout, "::endgroup::")
			}
		}
		if combinedErr != nil {
			return combinedErr
		}
	}

	if err := buildError(statuses); err != nil {
		return err
	}
	return nil
}

func (c *viamClient) startBuild(repo, ref, moduleID string, platforms []string, version string) (*buildpb.StartBuildResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.StartBuildRequest{
		Repo:          repo,
		Ref:           &ref,
		Platforms:     platforms,
		ModuleId:      moduleID,
		ModuleVersion: version,
	}
	return c.buildClient.StartBuild(c.c.Context, &req)
}

func (c *viamClient) printModuleBuildLogs(buildID, platform string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	logsReq := &buildpb.GetLogsRequest{
		BuildId:  buildID,
		Platform: platform,
	}

	stream, err := c.buildClient.GetLogs(c.c.Context, logsReq)
	if err != nil {
		return err
	}
	lastBuildStep := ""
	for {
		if c.c.Context.Err() != nil {
			return c.c.Context.Err()
		}
		log, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if lastBuildStep != log.BuildStep {
			infof(c.c.App.Writer, log.BuildStep)
			lastBuildStep = log.BuildStep
		}
		fmt.Fprint(c.c.App.Writer, log.Data) // data is already formatted with newlines
	}

	return nil
}

func (c *viamClient) listModuleBuildJobs(moduleIDFilter string, count *int32, buildIDFilter *string) (*buildpb.ListJobsResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.ListJobsRequest{
		ModuleId:      moduleIDFilter,
		MaxJobsLength: count,
		BuildId:       buildIDFilter,
	}
	return c.buildClient.ListJobs(c.c.Context, &req)
}

// waitForBuildToFinish calls listModuleBuildJobs every moduleBuildPollingInterval
// Will wait until the status of the specified job is DONE or FAILED
// if platform is empty, it waits for all jobs associated with the ID.
func (c *viamClient) waitForBuildToFinish(buildID, platform string) (map[string]jobStatus, error) {
	// If the platform is not empty, we should check that the platform is actually present on the build
	// this is mostly to protect against users misspelling the platform
	if platform != "" {
		platformsForBuild, err := c.getPlatformsForModuleBuild(buildID)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(platformsForBuild, platform) {
			return nil, fmt.Errorf("platform %q is not present on build %q", platform, buildID)
		}
	}
	statuses := make(map[string]jobStatus)
	ticker := time.NewTicker(moduleBuildPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.c.Context.Done():
			return nil, c.c.Context.Err()
		case <-ticker.C:
			jobsResponse, err := c.listModuleBuildJobs("", nil, &buildID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to list module build jobs")
			}
			if len(jobsResponse.Jobs) == 0 {
				return nil, fmt.Errorf("build id %q returned no jobs", buildID)
			}
			// Loop through all the jobs and check if all the matching jobs are done
			allDone := true
			for _, job := range jobsResponse.Jobs {
				if platform == "" || job.Platform == platform {
					status := jobStatusFromProto(job.Status)
					statuses[job.Platform] = status
					if status != jobStatusDone && status != jobStatusFailed {
						allDone = false
						break
					}
				}
			}
			// If all jobs are done, return
			if allDone {
				return statuses, nil
			}
		}
	}
}

func (c *viamClient) getPlatformsForModuleBuild(buildID string) ([]string, error) {
	platforms := []string{}
	jobsResponse, err := c.listModuleBuildJobs("", nil, &buildID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list module build jobs")
	}
	for _, job := range jobsResponse.Jobs {
		platforms = append(platforms, job.Platform)
	}
	return platforms, nil
}

func jobStatusFromProto(s buildpb.JobStatus) jobStatus {
	switch s {
	case buildpb.JobStatus_JOB_STATUS_IN_PROGRESS:
		return jobStatusInProgress
	case buildpb.JobStatus_JOB_STATUS_FAILED:
		return jobStatusFailed
	case buildpb.JobStatus_JOB_STATUS_DONE:
		return jobStatusDone
	case buildpb.JobStatus_JOB_STATUS_UNSPECIFIED:
		fallthrough
	default:
		return jobStatusUnspecified
	}
}

// ReloadModuleAction builds a module, configures it on a robot, and starts or restarts it.
func ReloadModuleAction(c *cli.Context) error {
	vc, err := newViamClient(c)
	if err != nil {
		return err
	}
	return reloadModuleAction(c, vc)
}

// reloadModuleAction is the testable inner reload logic.
func reloadModuleAction(c *cli.Context, vc *viamClient) error {
	partID, err := resolvePartID(c.Context, c.String(partFlag), "/etc/viam.json")
	if err != nil {
		return err
	}
	manifest, err := loadManifestOrNil(c.String(moduleFlagPath))
	if err != nil {
		return err
	}
	part, err := vc.getRobotPart(partID)
	if err != nil {
		return err
	}
	if part.Part == nil {
		return fmt.Errorf("part with id=%s not found", partID)
	}
	// note: configureModule and restartModule signal the robot via different channels.
	// Running this command in rapid succession can cause an extra restart because the
	// CLI will see configuration changes before the robot, and skip to the needsRestart
	// case on the second call. Because these are triggered by user actions, we're okay
	// with this behavior, and the robot will eventually converge to what is in config.
	needsRestart := true
	if !c.Bool(moduleBuildRestartOnly) {
		if !c.Bool(moduleBuildFlagNoBuild) {
			if manifest == nil {
				return fmt.Errorf(`manifest not found at "%s". manifest required for build`, moduleFlagPath)
			}
			err = moduleBuildLocalAction(c, manifest)
			if err != nil {
				return err
			}
		}
		if !c.Bool(moduleFlagLocal) {
			if manifest == nil || manifest.Build == nil || manifest.Build.Path == "" {
				return errors.New(
					"remote reloading requires a meta.json with the 'build.path' field set. " +
						"try --local if you are testing on the same machine.",
				)
			}
			if err := validateReloadableArchive(c, manifest.Build); err != nil {
				return err
			}
			if err := addShellService(c, vc, part.Part, true); err != nil {
				return err
			}
			infof(c.App.Writer, "Copying %s to part %s", manifest.Build.Path, part.Part.Id)
			err = vc.copyFilesToFqdn(
				part.Part.Fqdn, c.Bool(debugFlag), false, false, []string{manifest.Build.Path},
				reloadingDestination(c, manifest), logging.NewLogger("reload"))
			if err != nil {
				return err
			}
		}
		needsRestart, err = configureModule(c, vc, manifest, part.Part)
		if err != nil {
			return err
		}
	}
	if needsRestart {
		return restartModule(c, vc, part.Part, manifest)
	}
	infof(c.App.Writer, "Reload complete")
	return nil
}

// this chooses a destination path for the module archive.
func reloadingDestination(c *cli.Context, manifest *moduleManifest) string {
	return filepath.Join(c.String(moduleFlagHomeDir),
		".viam", config.PackagesDirName+config.LocalPackagesSuffix,
		utils.SanitizePath(localizeModuleID(manifest.ModuleID)+"-"+manifest.Build.Path))
}

// validateReloadableArchive returns an error if there is a fatal issue (for now just file not found).
// It also logs warnings for likely problems.
func validateReloadableArchive(c *cli.Context, build *manifestBuildInfo) error {
	reader, err := os.Open(build.Path)
	if err != nil {
		return err
	}
	decompressed, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	archive := tar.NewReader(decompressed)
	metaFound := false
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "reading tar at %s", build.Path)
		}
		if header.Name == "meta.json" {
			metaFound = true
			break
		}
	}
	if !metaFound {
		warningf(c.App.ErrWriter, "archive at %s doesn't contain a meta.json, your module will probably fail to start", build.Path)
	}
	return nil
}

// resolvePartID takes an optional provided part ID (from partFlag), and an optional default viam.json, and returns a part ID to use.
func resolvePartID(ctx context.Context, partIDFromFlag, cloudJSON string) (string, error) {
	if len(partIDFromFlag) > 0 {
		return partIDFromFlag, nil
	}
	if len(cloudJSON) == 0 {
		return "", errors.New("no --part and no default json")
	}
	conf, err := config.ReadLocalConfig(ctx, cloudJSON, logging.NewLogger("config"))
	if err != nil {
		return "", err
	}
	if conf.Cloud == nil {
		return "", fmt.Errorf("unknown failure opening viam.json at: %s", cloudJSON)
	}
	return conf.Cloud.ID, nil
}

// resolveTargetModule looks at name / id flags and packs a RestartModuleRequest.
func resolveTargetModule(c *cli.Context, manifest *moduleManifest) (*robot.RestartModuleRequest, error) {
	modName := c.String(moduleFlagName)
	modID := c.String(moduleBuildFlagBuildID)
	// todo: use MutuallyExclusiveFlags for this when urfave/cli 3.x is stable
	if (len(modName) > 0) && (len(modID) > 0) {
		return nil, fmt.Errorf("provide at most one of --%s and --%s", moduleFlagName, moduleBuildFlagBuildID)
	}
	request := &robot.RestartModuleRequest{}
	//nolint:gocritic
	if len(modName) > 0 {
		request.ModuleName = modName
	} else if len(modID) > 0 {
		request.ModuleID = modID
	} else if manifest != nil {
		// TODO(APP-4019): remove localize call
		request.ModuleName = localizeModuleID(manifest.ModuleID)
	} else {
		return nil, fmt.Errorf("if there is no meta.json, provide one of --%s or --%s", moduleFlagName, moduleBuildFlagBuildID)
	}
	return request, nil
}

// restartModule restarts a module on a robot.
func restartModule(c *cli.Context, vc *viamClient, part *apppb.RobotPart, manifest *moduleManifest) error {
	restartReq, err := resolveTargetModule(c, manifest)
	if err != nil {
		return err
	}
	if err := vc.ensureLoggedIn(); err != nil {
		return err
	}
	apiRes, err := vc.client.GetRobotAPIKeys(c.Context, &apppb.GetRobotAPIKeysRequest{RobotId: part.Robot})
	if err != nil {
		return err
	}
	if len(apiRes.ApiKeys) == 0 {
		return errors.New("API keys list for this machine is empty. You can create one with \"viam machine api-key create\"")
	}
	key := apiRes.ApiKeys[0]
	debugf(c.App.Writer, c.Bool(debugFlag), "using API key: %s %s", key.ApiKey.Id, key.ApiKey.Name)
	creds := rpc.WithEntityCredentials(key.ApiKey.Id, rpc.Credentials{
		Type:    rpc.CredentialsTypeAPIKey,
		Payload: key.ApiKey.Key,
	})
	robotClient, err := client.New(c.Context, part.Fqdn, logging.NewLogger("robot"), client.WithDialOptions(creds))
	if err != nil {
		return err
	}
	defer robotClient.Close(c.Context) //nolint: errcheck
	debugf(c.App.Writer, c.Bool(debugFlag), "restarting module %v", restartReq)
	// todo: make this a stream so '--wait' can tell user what's happening
	err = robotClient.RestartModule(c.Context, *restartReq)
	if err == nil {
		infof(c.App.Writer, "restarted module.")
	}
	return err
}
