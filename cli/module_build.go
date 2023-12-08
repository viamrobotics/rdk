package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	buildpb "go.viam.com/api/app/build/v1"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/logging"
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
	platforms := manifest.Build.Arch
	if len(platforms) == 0 {
		platforms = defaultBuildInfo.Arch
	}
	res, err := c.startBuild(manifest.URL, "main", manifest.ModuleID, platforms, version)
	if err != nil {
		return err
	}
	// Print to stderr so that the buildID is the only thing in stdout
	printf(cCtx.App.ErrWriter, "Started build:")
	printf(cCtx.App.Writer, res.BuildId)
	return nil
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Build == nil || manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}
	infof(c.App.Writer, "Starting build")
	processConfig := pexec.ProcessConfig{
		Name:      "bash",
		OneShot:   true,
		Log:       true,
		LogWriter: c.App.Writer,
	}
	// Required logger for the ManagedProcess. Not used
	logger := logging.NewLogger("x")
	if manifest.Build.Setup != "" {
		infof(c.App.Writer, "Starting setup step: %q", manifest.Build.Setup)
		processConfig.Args = []string{"-c", manifest.Build.Setup}
		proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
		if err = proc.Start(c.Context); err != nil {
			return err
		}
	}
	infof(c.App.Writer, "Starting build step: %q", manifest.Build.Build)
	processConfig.Args = []string{"-c", manifest.Build.Build}
	proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
	if err = proc.Start(c.Context); err != nil {
		return err
	}
	infof(c.App.Writer, "Completed build")
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
	manifestPath := cCtx.String(moduleBuildFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	var numberOfJobsToReturn *int32
	if cCtx.IsSet(moduleBuildFlagCount) {
		count := int32(cCtx.Int(moduleBuildFlagCount))
		numberOfJobsToReturn = &count
	}
	var buildIDFilter *string
	if cCtx.IsSet(moduleBuildFlagBuildID) {
		filter := cCtx.String(moduleBuildFlagBuildID)
		buildIDFilter = &filter
	}
	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return err
	}
	jobs, err := c.listModuleBuildJobs(moduleID, numberOfJobsToReturn, buildIDFilter)
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

// ModuleBuildLogsAction retrieves the logs for a specific build step.
func ModuleBuildLogsAction(c *cli.Context) error {
	buildID := c.String(moduleBuildFlagBuildID)
	platform := c.String(moduleBuildFlagPlatform)
	shouldWait := c.Bool(moduleBuildFlagWait)
	if shouldWait {
		return errors.New("wait not implemented")
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	err = client.printModuleBuildLogs(c.Context, buildID, platform)
	if err != nil {
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

func (c *viamClient) printModuleBuildLogs(ctx context.Context, buildID, platform string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	logsReq := &buildpb.GetLogsRequest{
		BuildId:  buildID,
		Platform: platform,
	}

	stream, err := c.buildClient.GetLogs(ctx, logsReq)
	if err != nil {
		return err
	}
	lastBuildStep := ""
	for {
		if ctx.Err() != nil {
			return ctx.Err()
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

func (c *viamClient) listModuleBuildJobs(moduleID moduleID, number *int32, buildIDFilter *string) (*buildpb.ListJobsResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.ListJobsRequest{
		ModuleId:      moduleID.String(),
		MaxJobsLength: number,
		BuildId:       buildIDFilter,
	}
	return c.buildClient.ListJobs(c.c.Context, &req)
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
