package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	buildpb "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils/pexec"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/structpb"

	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
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

var reloadCommand = cli.Command{
	Name:  "reload",
	Usage: "run this module on a robot (only works on local robot for now)",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: moduleFlagPath, Value: "meta.json"},
		&cli.StringFlag{Name: generalFlagAliasRobotID},
		&cli.StringFlag{Name: configFlag},
	},
	Action: ModuleReloadAction,
}

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
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.moduleBuildLocalAction(cCtx)
}

func (c *viamClient) moduleBuildLocalAction(cCtx *cli.Context) error {
	manifestPath := cCtx.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
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
		if err = proc.Start(cCtx.Context); err != nil {
			return err
		}
	}
	infof(cCtx.App.Writer, "Starting build step: %q", manifest.Build.Build)
	processConfig.Args = []string{"-c", manifest.Build.Build}
	proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
	if err = proc.Start(cCtx.Context); err != nil {
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
			infof(c.App.Writer, "Logs for %q", platform)
			err := client.printModuleBuildLogs(buildID, platform)
			if err != nil {
				combinedErr = multierr.Combine(combinedErr, client.printModuleBuildLogs(buildID, platform))
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

// mapOver applies fn() to a slice of items and returns a slice of the return values.
func mapOver[T, U any](items []T, fn func(T) (U, error)) ([]U, error) {
	ret := make([]U, 0, len(items))
	for _, item := range items {
		newItem, err := fn(item)
		if err != nil {
			return nil, err
		}
		ret = append(ret, newItem)
	}
	return ret, nil
}

// mapToStructJson converts a map to a struct via json. The `mapstructure` package doesn't use json tags.
func mapToStructJson(raw map[string]interface{}, target interface{}) error {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

// structToMapJson does json ser/des to convert a struct to a map.
func structToMapJson(orig interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(orig)
	if err != nil {
		return nil, err
	}
	println("encoded", string(encoded))
	var ret map[string]interface{}
	err = json.Unmarshal(encoded, &ret)
	return ret, err
}

func getPartId(ctx context.Context, configPath string) (string, error) {
	conf, err := rdkConfig.ReadLocalConfig(ctx, configPath, logging.Global())
	if err != nil {
		return "", err
	}
	return conf.Cloud.ID, nil
}

func ModuleReloadAction(cCtx *cli.Context) error {
	logger := logging.Global()
	robotId := cCtx.String(generalFlagAliasRobotID)
	configPath := cCtx.String(configFlag)
	// todo: switch to MutuallyExclusiveFlags when available
	if (len(robotId) == 0) && (len(configPath) == 0) {
		return fmt.Errorf("provide exactly one of --%s or --%s", generalFlagAliasRobotID, configFlag)
	}
	manifest, err := loadManifest(cCtx.String(moduleFlagPath))
	if err != nil {
		return err
	}
	var conf *rdkConfig.Config
	if len(robotId) != 0 {
		return errors.New("robot-id not implemented yet")
	} else {
		conf, err = rdkConfig.Read(cCtx.Context, configPath, logging.Global())
		if err != nil {
			return err
		}
	}
	localName := "hr_" + strings.ReplaceAll(manifest.ModuleID, ":", "_")
	var foundMod *rdkConfig.Module
	dirty := false
	for _, mod := range conf.Modules {
		if mod.ModuleID == manifest.ModuleID {
			foundMod = &mod
			break
		} else if mod.Name == localName {
			foundMod = &mod
			break
		}
	}
	absEntrypoint, err := filepath.Abs(manifest.Entrypoint)
	if err != nil {
		return err
	}
	if foundMod == nil {
		logger.Debug("module not found, inserting")
		dirty = true
		newMod := rdkConfig.Module{
			Name:    localName,
			ExePath: absEntrypoint,
			Type:    rdkConfig.ModuleTypeLocal,
			// todo: let user pass through LogLevel and Environment
		}
		conf.Modules = append(conf.Modules, newMod)
	} else {
		if same, err := compareExePath(foundMod, &manifest); err != nil {
			return err
		} else if !same {
			dirty = true
			logger.Debug("replacing entrypoint")
			foundMod.ExePath = absEntrypoint
		}
	}
	if dirty {
		logger.Debug("writing back config changes")
		vc, err := newViamClient(cCtx)
		if err != nil {
			return err
		}
		err = vc.updateRobotPart(conf.Cloud.ID, conf)
		if err != nil {
			return err
		}
	}
	return nil
}

// compareExePath returns true if mod.ExePath and manifest.Entrypoint are the same path.
func compareExePath(mod *rdkConfig.Module, manifest *moduleManifest) (bool, error) {
	exePath, err := filepath.Abs(mod.ExePath)
	if err != nil {
		return false, err
	}
	entrypoint, err := filepath.Abs(manifest.Entrypoint)
	if err != nil {
		return false, err
	}
	return exePath == entrypoint, nil
}

// marshalToMap does json ser/des to convert a struct to a map.
func marshalToMap(orig interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(orig)
	if err != nil {
		return nil, err
	}
	var ret map[string]interface{}
	json.Unmarshal(encoded, &err)
	return ret, nil
}

func (c *viamClient) updateRobotPart(partId string, conf *rdkConfig.Config) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	confMap, err := marshalToMap(conf)
	if err != nil {
		return err
	}
	confStruct, err := structpb.NewStruct(confMap)
	if err != nil {
		return err
	}
	logging.Global().Warn("pls get actual name pls")
	req := apppb.UpdateRobotPartRequest{
		Id:          partId,
		Name:        "TODO DONT OVERWRITE NAME PLS",
		RobotConfig: confStruct,
	}
	_, err = c.client.UpdateRobotPart(c.c.Context, &req)
	return err
}
