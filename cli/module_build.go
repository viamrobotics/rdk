package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	v1 "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

type moduleBuildStartArgs struct {
	Module    string
	Version   string
	Ref       string
	Token     string
	Workdir   string
	Platforms []string
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(cCtx *cli.Context, args moduleBuildStartArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	_, err = c.moduleBuildStartAction(cCtx, args)
	return err
}

func (c *viamClient) moduleBuildStartForRepo(
	cCtx *cli.Context, args moduleBuildStartArgs, manifest *moduleManifest, repo string,
) (string, error) {
	version := args.Version
	if manifest.Build == nil || manifest.Build.Build == "" {
		return "", errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}

	// Clean the version argument to ensure compatibility with github tag standards
	version = strings.TrimPrefix(version, "v")

	var platforms []string
	if len(args.Platforms) > 0 {
		platforms = args.Platforms
	} else if len(manifest.Build.Arch) > 0 {
		platforms = manifest.Build.Arch
	} else {
		platforms = defaultBuildInfo.Arch
	}

	gitRef := args.Ref
	token := args.Token
	workdir := args.Workdir
	req := buildpb.StartBuildRequest{
		Repo:          repo,
		Ref:           &gitRef,
		Platforms:     platforms,
		ModuleId:      manifest.ModuleID,
		ModuleVersion: version,
		Token:         &token,
		Workdir:       &workdir,
	}
	res, err := c.buildClient.StartBuild(c.c.Context, &req)
	if err != nil {
		return "", err
	}
	// Print to stderr so that the buildID is the only thing in stdout
	printf(cCtx.App.ErrWriter, "Started build:")
	printf(cCtx.App.Writer, res.BuildId)
	return res.BuildId, nil
}

func (c *viamClient) moduleBuildStartAction(cCtx *cli.Context, args moduleBuildStartArgs) (string, error) {
	manifest, err := loadManifest(args.Module)
	if err != nil {
		return "", err
	}

	if manifest.URL == "" {
		return "", errors.New("meta.json must have a url field set in order to start a cloud build. " +
			"Ex: 'https://github.com/your-username/your-repo'")
	}

	return c.moduleBuildStartForRepo(cCtx, args, &manifest, manifest.URL)
}

type moduleBuildLocalArgs struct {
	Module string
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(cCtx *cli.Context, args moduleBuildLocalArgs) error {
	manifestPath := args.Module
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	return moduleBuildLocalAction(cCtx, &manifest, nil)
}

func moduleBuildLocalAction(cCtx *cli.Context, manifest *moduleManifest, environment map[string]string) error {
	if manifest.Build == nil || manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}
	infof(cCtx.App.Writer, "Starting build")
	processConfig := pexec.ProcessConfig{
		Environment: environment,
		Name:        "bash",
		OneShot:     true,
		Log:         true,
		LogWriter:   cCtx.App.Writer,
	}
	// Required logger for the ManagedProcess. Not used
	logger := logging.NewLogger("x")
	if manifest.Build.Setup != "" {
		infof(cCtx.App.Writer, "Starting setup step: %q", manifest.Build.Setup)
		processConfig.Args = []string{"-c", manifest.Build.Setup}
		proc := pexec.NewManagedProcess(processConfig, logger)
		if err := proc.Start(cCtx.Context); err != nil {
			return err
		}
	}
	infof(cCtx.App.Writer, "Starting build step: %q", manifest.Build.Build)
	processConfig.Args = []string{"-c", manifest.Build.Build}
	proc := pexec.NewManagedProcess(processConfig, logger)
	if err := proc.Start(cCtx.Context); err != nil {
		return err
	}
	infof(cCtx.App.Writer, "Completed build")
	return nil
}

type moduleBuildListArgs struct {
	Module string
	Count  int
	ID     string
}

// ModuleBuildListAction lists the module's build jobs.
func ModuleBuildListAction(cCtx *cli.Context, args moduleBuildListArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.moduleBuildListAction(cCtx, args)
}

func (c *viamClient) moduleBuildListAction(cCtx *cli.Context, args moduleBuildListArgs) error {
	buildIDFilter := args.ID
	var moduleIDFilter string
	// Fall back on the module manifest if build id is not present.
	if buildIDFilter == "" {
		manifestPath := args.Module
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
	if args.Count != 0 {
		count := int32(args.Count)
		numberOfJobsToReturn = &count
	}

	var buildID *string
	if buildIDFilter != "" {
		buildID = &buildIDFilter
	}

	jobs, err := c.listModuleBuildJobs(moduleIDFilter, numberOfJobsToReturn, buildID)
	if err != nil {
		return err
	}
	// table format rules:
	// minwidth, tabwidth, padding int, padchar byte, flags uint
	w := tabwriter.NewWriter(cCtx.App.Writer, 5, 4, 1, ' ', 0)
	tableFormat := "%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, tableFormat, "ID", "PLATFORM", "STATUS", "VERSION", "TIME") //nolint:errcheck
	for _, job := range jobs.Jobs {
		fmt.Fprintf(w, //nolint:errcheck
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

type moduleBuildLogsArgs struct {
	ID        string
	Platform  string
	Wait      bool
	GroupLogs bool
}

// ModuleBuildLogsAction retrieves the logs for a specific build step.
func ModuleBuildLogsAction(c *cli.Context, args moduleBuildLogsArgs) error {
	buildID := args.ID
	platform := args.Platform
	shouldWait := args.Wait
	groupLogs := args.GroupLogs

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	var statuses map[string]jobStatus
	if shouldWait {
		statuses, err = client.waitForBuildToFinish(buildID, platform, nil)
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

type moduleBuildLinkRepoArgs struct {
	OAuthLink string
	Module    string
	Repo      string
}

// ModuleBuildLinkRepoAction links a github repo to your module.
func ModuleBuildLinkRepoAction(c *cli.Context, args moduleBuildLinkRepoArgs) error {
	linkID := args.OAuthLink
	moduleID := args.Module
	repo := args.Repo

	if moduleID == "" {
		manifest, err := loadManifestOrNil(defaultManifestFilename)
		if err != nil {
			return fmt.Errorf("this command needs a module ID from either %s flag or valid %s", moduleFlagPath, defaultManifestFilename)
		}
		moduleID = manifest.ModuleID
		infof(c.App.ErrWriter, "using module ID %s from %s", moduleID, defaultManifestFilename)
	}

	if repo == "" {
		remoteURL, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
		if err != nil {
			return fmt.Errorf("no %s provided and unable to get git remote from current directory", moduleBuildFlagRepo)
		}
		parsed, err := url.Parse(strings.Trim(string(remoteURL), "\n "))
		if err != nil {
			return errors.Wrapf(err, "couldn't parse git remote %s; fix or use %s flag", remoteURL, moduleBuildFlagRepo)
		}
		if parsed.Host != "github.com" {
			return fmt.Errorf("can't use non-github git remote %s. To force this, use the %s flag", parsed.Host, moduleBuildFlagRepo)
		}
		repo = strings.Trim(parsed.Path, "/")
		infof(c.App.ErrWriter, "using repo %s from current folder", repo)
	}

	req := buildpb.LinkRepoRequest{
		Link: &buildpb.RepoLink{
			OauthAppLinkId: linkID,
			Repo:           repo,
		},
	}
	var found bool
	req.Link.OrgId, req.Link.ModuleName, found = strings.Cut(moduleID, ":")
	if !found {
		return fmt.Errorf("the given module ID '%s' isn't of the form org:name", moduleID)
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	res, err := client.buildClient.LinkRepo(c.Context, &req)
	if err != nil {
		return err
	}
	infof(c.App.Writer, "Successfully created link with ID %s", res.RepoLinkId)
	return nil
}

func (c *viamClient) printModuleBuildLogs(buildID, platform string) error {
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
		fmt.Fprint(c.c.App.Writer, log.Data) //nolint:errcheck // data is already formatted with newlines
	}

	return nil
}

func (c *viamClient) listModuleBuildJobs(moduleIDFilter string, count *int32, buildIDFilter *string) (*buildpb.ListJobsResponse, error) {
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
// If pm is not nil, it will show progress spinners for each build step.
func (c *viamClient) waitForBuildToFinish(
	buildID string,
	platform string,
	pm *ProgressManager,
) (map[string]jobStatus, error) {
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

	// Track the last build step to detect changes
	var lastBuildStep string
	var currentStepID string
	var buildStartCompleted bool

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

					// Handle progress spinner for build steps
					if pm != nil && job.GetBuildStep() != "" && job.GetBuildStep() != lastBuildStep {
						// On first build step, complete "build-start" with the build ID message
						if !buildStartCompleted {
							_ = pm.CompleteWithMessage("build-start", fmt.Sprintf("Build started (ID: %s)", buildID)) //nolint:errcheck
							buildStartCompleted = true
						}

						// Complete the previous step if it exists
						if currentStepID != "" {
							_ = pm.Complete(currentStepID) //nolint:errcheck
						}

						// Start a new step with IndentLevel = 1 (child of "Building...")
						currentStepID = "build-step-" + job.GetBuildStep()
						newStep := &Step{
							ID:          currentStepID,
							Message:     job.GetBuildStep(),
							Status:      StepPending,
							IndentLevel: 1,
						}
						pm.steps = append(pm.steps, newStep)
						pm.stepMap[currentStepID] = newStep

						_ = pm.Start(currentStepID) //nolint:errcheck
						lastBuildStep = job.GetBuildStep()
					}

					if status != jobStatusDone && status != jobStatusFailed {
						allDone = false
						break
					}
				}
			}
			// If all jobs are done, complete the last step and return
			if allDone {
				if pm != nil {
					// If build-start was never completed (no build steps received), complete it now
					if !buildStartCompleted {
						_ = pm.CompleteWithMessage("build-start", fmt.Sprintf("Build started (ID: %s)", buildID)) //nolint:errcheck
					}
					// Complete the last build step
					if currentStepID != "" {
						_ = pm.Complete(currentStepID) //nolint:errcheck
					}
				}
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

type reloadModuleArgs struct {
	PartID     string
	Module     string
	NoBuild    bool
	Local      bool
	NoProgress bool
	CloudBuild bool

	// CloudConfig is a path to the `viam.json`, or the config containing the robot ID.
	CloudConfig  string
	ModelName    string
	Workdir      string
	ResourceName string
	Path         string
}

func (c *viamClient) createGitArchive(repoPath string) (string, error) {
	var err error
	repoPath, err = filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	viamReloadArchive := ".VIAM_RELOAD_ARCHIVE.tar.gz"
	archivePath := filepath.Join(repoPath, viamReloadArchive)

	// Remove existing archive if it exists
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// Load gitignore patterns
	matcher, err := c.loadGitignorePatterns(repoPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to load gitignore patterns")
	}

	// Create the tar.gz archive
	//nolint:gosec // archivePath is constructed from validated repoPath and constant filename
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to create archive file")
	}
	defer func() {
		if closeErr := archiveFile.Close(); closeErr != nil {
			err = multierr.Append(err, errors.Wrap(closeErr, "failed to close archive file"))
		}
	}()

	gzWriter := gzip.NewWriter(archiveFile)
	defer func() {
		if closeErr := gzWriter.Close(); closeErr != nil {
			err = multierr.Append(err, errors.Wrap(closeErr, "failed to close gzip writer"))
		}
	}()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil {
			err = multierr.Append(err, errors.Wrap(closeErr, "failed to close tar writer"))
		}
	}()

	// Walk the filesystem and add files that are not ignored
	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the archive file itself
		if path == archivePath {
			return nil
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		// Skip directories and check if file should be ignored
		if info.IsDir() {
			// Skip .git directory
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches gitignore patterns
		if c.shouldIgnoreFile(relPath, matcher) {
			return nil
		}

		// Read file content
		//nolint:gosec // path is validated through filepath.Walk and relative path checks
		content, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", relPath)
		}

		// Create tar header
		header := &tar.Header{
			Name:    filepath.ToSlash(relPath), // Use forward slashes in tar
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return errors.Wrapf(err, "failed to write header for file %s", relPath)
		}

		// Write file content
		if _, err := tarWriter.Write(content); err != nil {
			return errors.Wrapf(err, "failed to write content for file %s", relPath)
		}

		return nil
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to process filesystem files")
	}

	return archivePath, nil
}

func (c *viamClient) loadGitignorePatterns(repoPath string) (gitignore.Matcher, error) {
	var patterns []gitignore.Pattern

	// Add default patterns to ignore common files
	defaultIgnores := []string{
		".git/",
		".DS_Store",
		"Thumbs.db",
		".VIAM_RELOAD_ARCHIVE.tar.gz", // Ignore the archive file itself
	}

	for _, pattern := range defaultIgnores {
		patterns = append(patterns, gitignore.ParsePattern(pattern, nil))
	}

	// Load .gitignore file if it exists
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		//nolint:gosec // gitignorePath is constructed from validated repoPath and constant filename
		file, err := os.Open(gitignorePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open .gitignore file")
		}
		defer func() {
			//nolint:errcheck,gosec // Ignore close error for read-only file
			file.Close()
		}()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, gitignore.ParsePattern(line, nil))
		}

		if err := scanner.Err(); err != nil {
			return nil, errors.Wrap(err, "failed to read .gitignore file")
		}
	}

	return gitignore.NewMatcher(patterns), nil
}

func (c *viamClient) shouldIgnoreFile(relPath string, matcher gitignore.Matcher) bool {
	// Convert to forward slashes for gitignore matching
	normalizedPath := filepath.ToSlash(relPath)
	return matcher.Match(strings.Split(normalizedPath, "/"), false)
}

func (c *viamClient) ensureModuleRegisteredInCloud(
	ctx *cli.Context, moduleID moduleID, manifest *moduleManifest, pm *ProgressManager,
) error {
	_, err := c.getModule(moduleID)
	if err != nil {
		// Module is not registered in the cloud, prompt user for confirmation
		// Stop the spinner before prompting for user input to avoid interference
		// with the interactive prompt.
		if pm != nil {
			pm.Stop()
		}

		red := "\033[1;31m%s\033[0m"
		printf(ctx.App.Writer, red, "Error: module not registered in cloud or you lack permissions to edit it.")

		yellow := "\033[1;33m%s\033[0m"
		printf(ctx.App.Writer, yellow, "Info: The reloading process requires the module to first be registered in the cloud. "+
			"Do you want to proceed with module registration?")
		printf(ctx.App.Writer, "Continue: y/n: ")
		if err := ctx.Err(); err != nil {
			return err
		}

		rawInput, err := bufio.NewReader(ctx.App.Reader).ReadString('\n')
		if err != nil {
			return err
		}

		input := strings.ToUpper(strings.TrimSpace(rawInput))
		if input != "Y" {
			return errors.New("module reload aborted - module not registered in cloud")
		}

		// If user confirmed, we'll proceed with the reload which will register the module
		// The registration happens implicitly through the cloud build process
		// Restart the spinner after user input
		if pm != nil {
			if err := pm.Start("register"); err != nil {
				return err
			}
		}

		org, err := getOrgByModuleIDPrefix(c, moduleID.prefix)
		if err != nil {
			return err
		}
		// Create the module in the cloud
		_, err = c.createModule(moduleID.name, org.GetId())
		if err != nil {
			return err
		}
	}

	// always update the cloud module before reloading
	_, err = c.updateModule(moduleID, *manifest)
	if err != nil {
		return err
	}

	return nil
}

func (c *viamClient) inferOrgIDFromManifest(manifest moduleManifest) (string, error) {
	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return "", err
	}
	org, err := getOrgByModuleIDPrefix(c, moduleID.prefix)
	if err != nil {
		return "", err
	}

	return org.GetId(), nil
}

func (c *viamClient) triggerCloudReloadBuild(
	ctx *cli.Context,
	args reloadModuleArgs,
	manifest moduleManifest,
	archivePath, partID string,
) (string, error) {
	stream, err := c.buildClient.StartReloadBuild(ctx.Context)
	if err != nil {
		return "", err
	}

	//nolint:gosec
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}

	orgID, err := c.inferOrgIDFromManifest(manifest)
	if err != nil {
		return "", err
	}

	part, err := c.getRobotPart(args.PartID)
	if err != nil {
		return "", err
	}

	if part.Part == nil {
		return "", fmt.Errorf("part with id=%s not found", args.PartID)
	}

	if part.Part.UserSuppliedInfo == nil {
		return "", errors.New("unable to determine platform for part")
	}

	// App expects `BuildInfo` as the first request
	platform := part.Part.UserSuppliedInfo.Fields["platform"].GetStringValue()
	req := &buildpb.StartReloadBuildRequest{
		CloudBuild: &buildpb.StartReloadBuildRequest_BuildInfo{
			BuildInfo: &buildpb.ReloadBuildInfo{
				Platform: platform,
				Workdir:  &args.Workdir,
				ModuleId: manifest.ModuleID,
			},
		},
	}
	if err := stream.Send(req); err != nil {
		return "", err
	}

	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return "", err
	}
	pkgInfo := v1.PackageInfo{
		OrganizationId: orgID,
		Name:           moduleID.name,
		Version:        getReloadVersion(reloadSourceVersionPrefix, partID),
		Type:           v1.PackageType_PACKAGE_TYPE_MODULE,
	}
	reqInner := &v1.CreatePackageRequest{
		Package: &v1.CreatePackageRequest_Info{
			Info: &pkgInfo,
		},
	}
	req = &buildpb.StartReloadBuildRequest{
		CloudBuild: &buildpb.StartReloadBuildRequest_Package{
			Package: reqInner,
		},
	}

	if err := stream.Send(req); err != nil {
		return "", err
	}

	var errs error
	// Suppress the "Uploading... X%" progress bar output since we have our own spinner
	if err := sendUploadRequests(
		ctx.Context, stream, file, io.Discard, getNextReloadBuildUploadRequest); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	if closeErr != nil && !errors.Is(closeErr, io.EOF) {
		errs = multierr.Combine(errs, closeErr)
	}
	return resp.GetBuildId(), errs
}

func getNextReloadBuildUploadRequest(file *os.File) (*buildpb.StartReloadBuildRequest, int, error) {
	packagesRequest, byteLen, err := getNextPackageUploadRequest(file)
	if err != nil {
		return nil, 0, err
	}

	return &buildpb.StartReloadBuildRequest{
		CloudBuild: &buildpb.StartReloadBuildRequest_Package{
			Package: packagesRequest,
		},
	}, byteLen, nil
}

// moduleCloudBuildInfo contains information needed to download a cloud build artifact.
type moduleCloudBuildInfo struct {
	ID          string
	Version     string
	Platform    string
	ArchivePath string // Path to the temporary archive that should be deleted after download
}

// moduleCloudReload triggers a cloud build and returns info needed to download the artifact.
func (c *viamClient) moduleCloudReload(
	ctx *cli.Context,
	args reloadModuleArgs,
	platform string,
	manifest moduleManifest,
	partID string,
	pm *ProgressManager,
) (*moduleCloudBuildInfo, error) {
	// Start the "Preparing for build..." parent step (prints as header)
	if err := pm.Start("prepare"); err != nil {
		return nil, err
	}

	// ensure that the module has been registered in the cloud
	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return nil, err
	}

	if err := pm.Start("register"); err != nil {
		return nil, err
	}
	err = c.ensureModuleRegisteredInCloud(ctx, moduleID, &manifest, pm)
	if err != nil {
		_ = pm.FailWithMessage("register", "Registration failed")   //nolint:errcheck
		_ = pm.FailWithMessage("prepare", "Preparing for build...") //nolint:errcheck
		return nil, err
	}
	if err := pm.Complete("register"); err != nil {
		return nil, err
	}

	id := ctx.String(generalFlagID)
	if id == "" {
		id = manifest.ModuleID
	}

	if err := pm.Start("archive"); err != nil {
		return nil, err
	}
	archivePath, err := c.createGitArchive(args.Path)
	if err != nil {
		_ = pm.FailWithMessage("archive", "Archive creation failed") //nolint:errcheck
		_ = pm.FailWithMessage("prepare", "Preparing for build...")  //nolint:errcheck
		return nil, err
	}
	if err := pm.Complete("archive"); err != nil {
		return nil, err
	}

	if err := pm.Start("upload-source"); err != nil {
		return nil, err
	}
	buildID, err := c.triggerCloudReloadBuild(ctx, args, manifest, archivePath, partID)
	if err != nil {
		_ = pm.FailWithMessage("upload-source", "Upload failed")    //nolint:errcheck
		_ = pm.FailWithMessage("prepare", "Preparing for build...") //nolint:errcheck
		return nil, err
	}
	if err := pm.Complete("upload-source"); err != nil {
		return nil, err
	}

	// Complete the "Preparing for build..." parent step AFTER all its children
	if err := pm.Complete("prepare"); err != nil {
		return nil, err
	}

	// Start the "Building..." parent step (prints as header)
	if err := pm.Start("build"); err != nil {
		return nil, err
	}

	// Start "Starting build..." and keep it active until first actual build step
	if err := pm.Start("build-start"); err != nil {
		return nil, err
	}

	// ensure the build completes before we try to download and use it
	// waitForBuildToFinish will complete "build-start" when first build step is received
	statuses, err := c.waitForBuildToFinish(buildID, platform, pm)
	if err != nil {
		_ = pm.FailWithMessage("build", "Building...") //nolint:errcheck
		return nil, err
	}

	// if the build failed, print the logs and return an error
	if statuses[platform] == jobStatusFailed {
		_ = pm.FailWithMessage("build", "Building...") //nolint:errcheck

		// Print error message without exiting (don't use Errorf since it calls os.Exit(1))
		errorf(c.c.App.Writer, "Build %q failed to complete. Please check the logs below for more information.", buildID)

		if err = c.printModuleBuildLogs(buildID, platform); err != nil {
			return nil, err
		}

		return nil, errors.Errorf("Reloading module failed")
	}
	// Note: The "build" parent step will be completed by the caller after downloading artifacts

	// Return build info so the caller can download the artifact with a spinner
	return &moduleCloudBuildInfo{
		ID:          id,
		Version:     getReloadVersion(reloadVersionPrefix, partID),
		Platform:    platform,
		ArchivePath: archivePath,
	}, nil
}

// ReloadModuleLocalAction builds a module locally, configures it on a robot, and starts or restarts it.
func ReloadModuleLocalAction(c *cli.Context, args reloadModuleArgs) error {
	return reloadModuleAction(c, args, false)
}

// ReloadModuleAction builds a module, configures it on a robot, and starts or restarts it.
func ReloadModuleAction(c *cli.Context, args reloadModuleArgs) error {
	return reloadModuleAction(c, args, true)
}

func reloadModuleAction(c *cli.Context, args reloadModuleArgs, cloudBuild bool) error {
	vc, err := newViamClient(c)
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

	return reloadModuleActionInner(c, vc, args, logger, cloudBuild)
}

func getReloadVersion(versionPrefix, partID string) string {
	return versionPrefix + "-" + partID
}

// reload with cloudbuild was supported starting in 0.90.0
// there are older versions of viam-servet that don't support ~/ file prefix, so lets avoid using them.
var reloadVersionSupported = semver.MustParse("0.90.0")

// reloadModuleActionInner is the testable inner reload logic.
func reloadModuleActionInner(
	c *cli.Context,
	vc *viamClient,
	args reloadModuleArgs,
	logger logging.Logger,
	cloudBuild bool,
) error {
	// TODO(RSDK-9727) it'd be nice for this to be a method on a viam client rather than taking one as an arg
	partID, err := resolvePartID(args.PartID, args.CloudConfig)
	if err != nil {
		return err
	}
	manifest, err := loadManifestOrNil(args.Module)
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

	var partOs string
	var partArch string
	var platform string
	if part.Part.UserSuppliedInfo != nil {
		// Check if the viam-server version is supported for hot reloading
		if part.Part.UserSuppliedInfo.Fields["version"] != nil {
			// Note: developer instances of viam-server will not have a semver version (instead it is a git commit)
			// so we can safely ignore the error here, assuming that all real instances of viam-server will have a semver version
			version, err := semver.NewVersion(part.Part.UserSuppliedInfo.Fields["version"].GetStringValue())
			if err == nil && version.LessThan(reloadVersionSupported) {
				return fmt.Errorf("viam-server version %s is not supported for hot reloading,"+
					"please update to at least %s", version.Original(), reloadVersionSupported.Original())
			}
		}

		platform = part.Part.UserSuppliedInfo.Fields["platform"].GetStringValue()
		if partInfo := strings.SplitN(platform, "/", 2); len(partInfo) == 2 {
			partOs = partInfo[0]
			partArch = partInfo[1]
		}
	}

	// Create environment map with platform info
	environment := map[string]string{
		"VIAM_BUILD_OS":   partOs,
		"VIAM_BUILD_ARCH": partArch,
	}

	// Add all environment variables with VIAM_ prefix
	for _, envVar := range os.Environ() {
		if parts := strings.SplitN(envVar, "=", 2); len(parts) == 2 && strings.HasPrefix(parts[0], "VIAM_") {
			environment[parts[0]] = parts[1]
		}
	}

	// note: configureModule and restartModule signal the robot via different channels.
	// Running this command in rapid succession can cause an extra restart because the
	// CLI will see configuration changes before the robot, and skip to the needsRestart
	// case on the second call. Because these are triggered by user actions, we're okay
	// with this behavior, and the robot will eventually converge to what is in config.

	// Define all steps upfront (build + reload) with clear parent/child relationships
	allSteps := []*Step{
		{ID: "prepare", Message: "Preparing for build...", CompletedMsg: "Prepared for build", IndentLevel: 0},
		{ID: "register", Message: "Ensuring module is registered...", CompletedMsg: "Module is registered", IndentLevel: 1},
		{ID: "archive", Message: "Creating source code archive...", CompletedMsg: "Source code archive created", IndentLevel: 1},
		{ID: "upload-source", Message: "Uploading source code...", CompletedMsg: "Source code uploaded", IndentLevel: 1},
		{ID: "build", Message: "Building...", CompletedMsg: "Built", IndentLevel: 0},
		{ID: "build-start", Message: "Starting build...", IndentLevel: 1},
		// Dynamic build steps (e.g., "Spin up environment", "Install dependencies") are added at runtime with IndentLevel: 1
		{ID: "reload", Message: "Reloading to part...", CompletedMsg: "Reloaded to part", IndentLevel: 0},
		{ID: "download", Message: "Downloading build artifact...", CompletedMsg: "Build artifact downloaded", IndentLevel: 1},
		{ID: "shell", Message: "Setting up shell service...", CompletedMsg: "Shell service ready", IndentLevel: 1},
		{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 1},
		{ID: "configure", Message: "Configuring module...", CompletedMsg: "Module configured", IndentLevel: 1},
		{ID: "restart", Message: "Restarting module...", CompletedMsg: "Module restarted successfully", IndentLevel: 1},
		{ID: "resource", Message: "Adding resource...", CompletedMsg: "Resource added", IndentLevel: 1},
	}

	pm := NewProgressManager(allSteps, WithProgressOutput(!args.NoProgress))
	defer pm.Stop()

	var needsRestart bool
	var buildPath string
	var buildInfo *moduleCloudBuildInfo
	if !args.NoBuild {
		if manifest == nil {
			return fmt.Errorf(`manifest not found at "%s". manifest required for build`, moduleFlagPath)
		}
		if !cloudBuild {
			err = moduleBuildLocalAction(c, manifest, environment)
			buildPath = manifest.Build.Path
		} else {
			buildInfo, err = vc.moduleCloudReload(c, args, platform, *manifest, partID, pm)
			if err != nil {
				return err
			}

			// Complete the build phase before starting reload
			if err := pm.Complete("build"); err != nil {
				return err
			}

			// Download the build artifact with a spinner
			if err := pm.Start("reload"); err != nil {
				return err
			}
			if err := pm.Start("download"); err != nil {
				return err
			}
			downloadArgs := downloadModuleFlags{
				ID:          buildInfo.ID,
				Version:     buildInfo.Version,
				Platform:    buildInfo.Platform,
				Destination: ".",
			}
			downloadedPath, err := vc.downloadModuleAction(c, downloadArgs)
			if err != nil {
				_ = pm.Fail("download", err)                             //nolint:errcheck
				_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
				return err
			}

			// Move the downloaded artifact to reload-dist/{platform}.tar.gz
			platformFile := strings.ReplaceAll(buildInfo.Platform, "/", "-") + ".tar.gz"
			reloadDistPath := filepath.Join("reload-dist", platformFile)

			// Ensure reload-dist directory exists
			if err := os.MkdirAll("reload-dist", 0o750); err != nil {
				_ = pm.Fail("download", err)                             //nolint:errcheck
				_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
				return fmt.Errorf("failed to create reload-dist directory: %w", err)
			}

			// Move the file to the new location
			if err := os.Rename(downloadedPath, reloadDistPath); err != nil {
				_ = pm.Fail("download", err)                             //nolint:errcheck
				_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
				return fmt.Errorf("failed to move artifact to reload-dist: %w", err)
			}

			buildPath = reloadDistPath

			// Clean up the version directory that was created
			downloadDir := filepath.Dir(downloadedPath)
			if downloadDir != "." && downloadDir != "" {
				// Try to remove the version directory - if it fails, it's not critical
				_ = os.RemoveAll(downloadDir) //nolint:errcheck
			}

			if err := pm.Complete("download"); err != nil {
				return err
			}

			// Delete the archive we created
			if err := os.Remove(buildInfo.ArchivePath); err != nil {
				warningf(c.App.Writer, "failed to delete archive at %s", buildInfo.ArchivePath)
			}
		}
		if err != nil {
			return err
		}
	} else {
		// --no-build flag is set, look for existing artifact
		if !cloudBuild {
			// For local builds, use manifest build path if available
			if manifest == nil || manifest.Build == nil {
				return fmt.Errorf(`manifest not found at "%s". manifest required for reload`, moduleFlagPath)
			}
			buildPath = manifest.Build.Path
		} else {
			// For cloud builds, look for artifact in reload-dist directory
			if platform == "" {
				return errors.New("unable to determine platform for part")
			}
			platformFile := strings.ReplaceAll(platform, "/", "-") + ".tar.gz"
			artifactPath := filepath.Join("reload-dist", platformFile)

			// Check if file exists
			if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
				// Show command to run without --no-build
				errorf(c.App.ErrWriter, "No existing artifact found for platform %s at %s", platform, artifactPath)
				infof(c.App.ErrWriter, "To build and reload, run: viam module reload --part-id %s", partID)
				return fmt.Errorf("no existing artifact found for platform %s", platform)
			} else if err != nil {
				return fmt.Errorf("error checking for artifact: %w", err)
			}

			buildPath = artifactPath
			infof(c.App.ErrWriter, "Starting reload onto part with existing artifact at: %s...", artifactPath)
		}
	}

	if !args.Local {
		if manifest == nil || manifest.Build == nil || buildPath == "" {
			return errors.New(
				"remote reloading requires a meta.json with the 'build.path' field set. " +
					"try --local if you are testing on the same machine.",
			)
		}
		if err := validateReloadableArchive(c, manifest.Build); err != nil {
			// if it is a cloud build then it makes sense that we might not have a reloadable
			// archive locally, so we can safely ignore the error
			if !cloudBuild {
				return err
			}
		}

		// Start the "Reloading to part..." parent step if not already started (for local builds with cloud-built artifacts)
		if !cloudBuild {
			if err := pm.Start("reload"); err != nil {
				return err
			}
		}
		if err := pm.Start("shell"); err != nil {
			return err
		}
		shellAdded, err := addShellService(c, vc, logger, part.Part, true)
		if err != nil {
			_ = pm.Fail("shell", err)                                //nolint:errcheck
			_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
			return err
		}
		if shellAdded {
			if err := pm.CompleteWithMessage("shell", "Shell service installed"); err != nil {
				return err
			}
		} else {
			if err := pm.CompleteWithMessage("shell", "Shell service already exists"); err != nil {
				return err
			}
		}

		globalArgs, err := getGlobalArgs(c)
		if err != nil {
			return err
		}
		dest := reloadingDestination(c, manifest)

		if err := pm.Start("upload"); err != nil {
			return err
		}
		err = vc.retryableCopyToPart(
			c,
			part.Part.Fqdn,
			globalArgs.Debug,
			[]string{buildPath},
			dest,
			logger,
			partID,
			pm,
			vc.copyFilesToFqdn,
		)
		if err != nil {
			_ = pm.Fail("upload", err)                               //nolint:errcheck
			_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
			return err
		}
		if err := pm.Complete("upload"); err != nil {
			return err
		}
	} else {
		// For local builds, start the "Reloading to part..." parent step right before configure
		if err := pm.Start("reload"); err != nil {
			return err
		}
	}

	if err := pm.Start("configure"); err != nil {
		return err
	}
	var newPart *apppb.RobotPart
	newPart, needsRestart, err = configureModule(c, vc, manifest, part.Part, args.Local)
	// if the module has been configured, the cached response we have may no longer accurately reflect
	// the update, so we set the updated `part.Part`
	if newPart != nil {
		part.Part = newPart
	}

	if err != nil {
		_ = pm.Fail("configure", err)                            //nolint:errcheck
		_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
		return err
	}

	if !needsRestart {
		if err := pm.CompleteWithMessage("configure", "Module added to part"); err != nil {
			return err
		}
	} else {
		if err := pm.CompleteWithMessage("configure", "Module already exists on part"); err != nil {
			return err
		}
	}

	if needsRestart {
		if err := pm.Start("restart"); err != nil {
			return err
		}
		if err = restartModule(c, vc, part.Part, manifest, logger); err != nil {
			_ = pm.Fail("restart", err)                              //nolint:errcheck
			_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
			return err
		}
		if err := pm.Complete("restart"); err != nil {
			return err
		}
	}

	if args.ModelName != "" {
		if err := pm.Start("resource"); err != nil {
			return err
		}
		if err = vc.addResourceFromModule(c, part.Part, manifest, args.ModelName, args.ResourceName); err != nil {
			_ = pm.FailWithMessage("resource", fmt.Sprintf("Failed to add resource: %v", err)) //nolint:errcheck
			warningf(c.App.ErrWriter, "unable to add requested resource to robot config: %s", err)
		} else {
			resourceName := args.ResourceName
			if resourceName == "" {
				resourceName = args.ModelName
			}
			if err := pm.CompleteWithMessage("resource", fmt.Sprintf("Added %s", resourceName)); err != nil {
				return err
			}
		}
	}

	// Complete the parent "Reloading to part..." step
	if err := pm.Complete("reload"); err != nil {
		return err
	}

	return nil
}

type reloadingDestinationArgs struct {
	Home string
}

// this chooses a destination path for the module archive.
func reloadingDestination(c *cli.Context, manifest *moduleManifest) string {
	args := parseStructFromCtx[reloadingDestinationArgs](c)
	return filepath.Join(args.Home,
		".viam", config.PackagesDirName+config.LocalPackagesSuffix,
		utils.SanitizePath(localizeModuleID(manifest.ModuleID)+"-"+manifest.Build.Path))
}

// validateReloadableArchive returns an error if there is a fatal issue (for now just file not found).
// It also logs warnings for likely problems.
func validateReloadableArchive(c *cli.Context, build *manifestBuildInfo) error {
	reader, err := os.Open(build.Path)
	if err != nil {
		return errors.Wrap(err, "error opening the build.path field in your meta.json")
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
func resolvePartID(partIDFromFlag, cloudJSON string) (string, error) {
	if len(partIDFromFlag) > 0 {
		return partIDFromFlag, nil
	}
	if len(cloudJSON) == 0 {
		return "", errors.New("no --part and no default json")
	}
	conf, err := config.ReadLocalConfig(cloudJSON, logging.NewLogger("config"))
	if err != nil {
		return "", err
	}
	if conf.Cloud == nil {
		return "", fmt.Errorf("unknown failure opening viam.json at: %s", cloudJSON)
	}
	return conf.Cloud.ID, nil
}

// maxCopyAttempts is the number of times to retry copying files to a part before giving up.
const maxCopyAttempts = 6

// retryableCopyToPart attempts to copy files to a part using the shell service with retries.
// It handles progress manager updates for each attempt and provides helpful error messages.
// The copyFunc parameter allows for mocking in tests.
func (c *viamClient) retryableCopyToPart(
	ctx *cli.Context,
	fqdn string,
	debug bool,
	paths []string,
	dest string,
	logger logging.Logger,
	partID string,
	pm *ProgressManager,
	copyFunc func(fqdn string, debug, allowRecursion, preserve bool,
		paths []string, destination string, logger logging.Logger, noProgress bool) error,
) error {
	var hadPreviousFailure bool

	for attempt := 1; attempt <= maxCopyAttempts; attempt++ {
		// If we had a previous failure, create a nested step for this retry
		var attemptStepID string
		if hadPreviousFailure {
			attemptStepID = fmt.Sprintf("upload-attempt-%d", attempt)
			attemptStep := &Step{
				ID:           attemptStepID,
				Message:      fmt.Sprintf("Upload attempt %d/%d...", attempt, maxCopyAttempts),
				CompletedMsg: fmt.Sprintf("Upload attempt %d succeeded", attempt),
				Status:       StepPending,
				IndentLevel:  2, // Nested under "upload" which is at level 1
			}
			pm.steps = append(pm.steps, attemptStep)
			pm.stepMap[attemptStepID] = attemptStep

			if err := pm.Start(attemptStepID); err != nil {
				return err
			}
		}

		err := copyFunc(fqdn, debug, false, false, paths, dest, logger, true)

		if err == nil {
			// Success! Complete the step if this was a retry
			if attemptStepID != "" {
				if err := pm.Complete(attemptStepID); err != nil {
					return err
				}
			}
			return nil
		}

		// Handle error
		hadPreviousFailure = true

		// Print special warning for permission denied errors (in addition to regular error)
		if s, ok := status.FromError(err); ok && s.Code() == codes.PermissionDenied {
			warningf(ctx.App.ErrWriter, "RDK couldn't write to the default file copy destination. "+
				"If you're running as non-root, try adding --home $HOME or --home /user/username to your CLI command. "+
				"Alternatively, run the RDK as root.")
		}

		// Create a step for this failed attempt (so it shows in the output)
		if attemptStepID == "" {
			// First attempt - create its step retroactively
			attemptStepID = "upload-attempt-1"
			attemptStep := &Step{
				ID:           attemptStepID,
				Message:      fmt.Sprintf("Upload attempt 1/%d...", maxCopyAttempts),
				CompletedMsg: "Upload attempt 1 succeeded",
				Status:       StepPending,
				IndentLevel:  2,
			}
			pm.steps = append(pm.steps, attemptStep)
			pm.stepMap[attemptStepID] = attemptStep
			if err := pm.Start(attemptStepID); err != nil {
				return err
			}
		}

		// Mark this attempt as failed (this will print the error on next line)
		_ = pm.Fail(attemptStepID, err) //nolint:errcheck
	}

	// All attempts failed - return a comprehensive error message
	return fmt.Errorf("all %d upload attempts failed. You can retry the copy later, "+
		"skipping the build step with: viam module reload --no-build --part-id %s", maxCopyAttempts, partID)
}

type resolveTargetModuleArgs struct {
	Name string
	ID   string
}

// resolveTargetModule looks at name / id flags and packs a RestartModuleRequest.
func resolveTargetModule(c *cli.Context, manifest *moduleManifest) (*robot.RestartModuleRequest, error) {
	args := parseStructFromCtx[resolveTargetModuleArgs](c)
	modName := args.Name
	modID := args.ID
	// todo: use MutuallyExclusiveFlags for this when urfave/cli 3.x is stable
	if (len(modName) > 0) && (len(modID) > 0) {
		return nil, fmt.Errorf("provide at most one of --%s and --%s", generalFlagName, generalFlagID)
	}
	request := &robot.RestartModuleRequest{}

	if len(modName) > 0 {
		request.ModuleName = modName
	} else if len(modID) > 0 {
		request.ModuleID = modID
	} else if manifest != nil {
		request.ModuleID = manifest.ModuleID
	} else {
		return nil, fmt.Errorf("if there is no meta.json, provide one of --%s or --%s", generalFlagName, generalFlagID)
	}
	return request, nil
}

type moduleRestartArgs struct {
	PartID      string
	Module      string
	CloudConfig string
}

// ModuleRestartAction triggers a restart of the requested module.
func ModuleRestartAction(c *cli.Context, args moduleRestartArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	part, err := client.getRobotPart(args.PartID)
	if err != nil {
		return err
	}

	manifest, err := loadManifestOrNil(args.Module)
	if err != nil {
		return err
	}
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())

	return restartModule(c, client, part.Part, manifest, logger)
}

// restartModule restarts a module on a robot.
func restartModule(
	c *cli.Context,
	vc *viamClient,
	part *apppb.RobotPart,
	manifest *moduleManifest,
	logger logging.Logger,
) error {
	// TODO(RSDK-9727) it'd be nice for this to be a method on a viam client rather than taking one as an arg
	restartReq, err := resolveTargetModule(c, manifest)
	if err != nil {
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
	args, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	debugf(c.App.Writer, args.Debug, "using API key: %s %s", key.ApiKey.Id, key.ApiKey.Name)
	creds := rpc.WithEntityCredentials(key.ApiKey.Id, rpc.Credentials{
		Type:    rpc.CredentialsTypeAPIKey,
		Payload: key.ApiKey.Key,
	})
	robotClient, err := client.New(c.Context, part.Fqdn, logger, client.WithDialOptions(creds))
	if err != nil {
		return err
	}
	defer robotClient.Close(c.Context) //nolint: errcheck
	debugf(c.App.Writer, args.Debug, "restarting module %v", restartReq)
	// todo: make this a stream so '--wait' can tell user what's happening
	return robotClient.RestartModule(c.Context, *restartReq)
}
