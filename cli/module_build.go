package cli

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	buildpb "go.viam.com/api/app/build/v1"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/logging"
)

type jobStatus string

const (
	jobStatusUnknown    jobStatus = "unknown"
	jobStatusInProgress jobStatus = "in progress"
	jobStatusFailed     jobStatus = "failed"
	jobStatusDone       jobStatus = "done"
)

func (c *viamClient) startBuild(repo, ref, moduleID string, platform []string) (*buildpb.StartBuildResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.StartBuildRequest{
		Repo:     &repo,
		Ref:      &ref,
		Platform:     platform,
		ModuleId: moduleID,
	}
	return c.buildClient.StartBuild(c.c.Context, &req)
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(c *cli.Context) error {
	manifest, err := loadManifest(c.String(moduleFlagPath))
	if err != nil {
		return err
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	platforms := manifest.Build.Arch
	if len(platforms) == 0 {
		platforms = defaultBuildInfo.Arch
	}
	res, err := client.startBuild(manifest.URL, "main", manifest.ModuleID, platforms)
	if err != nil {
		return err
	}
	// org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	// if err != nil {
	// 	return err
	// }
	// // Check to make sure the user doesn't accidentally overwrite a module manifest
	// if _, err := os.Stat(defaultManifestFilename); err == nil {
	// 	return errors.New("another module's meta.json already exists in the current directory. Delete it and try again")
	// }

	// response, err := client.createModule(moduleNameArg, org.GetId())
	// if err != nil {
	// 	return errors.Wrap(err, "failed to register the module on app.viam.com")
	// }

	// returnedModuleID, err := parseModuleID(response.GetModuleId())
	// if err != nil {
	// 	return err
	// }

	// printf(c.App.Writer, "Successfully created '%s'", returnedModuleID.String())
	// if response.GetUrl() != "" {
	// 	printf(c.App.Writer, "You can view it here: %s", response.GetUrl())
	// }
	// emptyManifest := moduleManifest{
	// 	ModuleID:   returnedModuleID.String(),
	// 	Visibility: moduleVisibilityPrivate,
	// 	// This is done so that the json has an empty example
	// 	Models: []ModuleComponent{
	// 		{},
	// 	},
	// }
	// if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
	// 	return err
	// }

	// todo: change to VendorID
	printf(c.App.Writer, "got'em %s\n", res.BuildId)
	return nil
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Build.Build == "" {
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
func ModuleBuildListAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	var numberOfJobsToReturn *int32
	if c.IsSet(moduleBuildFlagNumber) {
		number := int32(c.Int(moduleBuildFlagNumber))
		numberOfJobsToReturn = &number
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return err
	}
	jobs, err := client.listModuleBuildJobs(moduleID, numberOfJobsToReturn)
	if err != nil {
		return err
	}
	firstN := func(str string, n int) string {
		v := []rune(str)
		if n >= len(v) {
			return str
		}
		return string(v[:n])
	}
	// table format rules:
	idLen := len("xyz123")
	statusLen := len("in progress")
	versionLen := len("1.2.34-rc0")
	platformLen := len("darwin/arm32v7")
	timeLen := len(time.RFC3339)
	//nolint:govet
	tableFormat := fmt.Sprintf("%-%dv %-%dv %-%dv %-%dv %-%dv ",
		idLen, platformLen, statusLen, versionLen, timeLen)
	printf(c.App.Writer, tableFormat, "ID", "PLATFORM", "STATUS", "VERSION", "TIME")
	for _, job := range jobs.Jobs {
		printf(c.App.Writer,
			tableFormat,
			firstN(job.BuildId, idLen),
			firstN(job.Platform, platformLen),
			firstN(string(jobStatusFromProto(job.Status)), statusLen),
			firstN(job.Version, versionLen),
			firstN(job.StartTime.AsTime().Format(time.RFC3339), timeLen),
		)
	}
	return nil
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
		return jobStatusUnknown
	}
}

func (c *viamClient) listModuleBuildJobs(moduleID moduleID, number *int32) (*buildpb.ListJobsResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.ListJobsRequest{
		ModuleId:      moduleID.String(),
		MaxJobsLength: number,
	}
	return c.buildClient.ListJobs(c.c.Context, &req)
}
