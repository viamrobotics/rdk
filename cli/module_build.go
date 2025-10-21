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
	"github.com/chelnak/ysmrr"
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

// ErrReloadFailed is returned when module reload fails.
var ErrReloadFailed = errors.Errorf("Reloading module failed")

type moduleBuildStartArgs struct {
	Module    string
	Version   string
	Ref       string
	Token     string
	Workdir   string
	Platforms []string
}

// moduleDownloadInfo holds the information needed to download a built module.
type moduleDownloadInfo struct {
	ID       string
	Version  string
	Platform string
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(cCtx *cli.Context, args moduleBuildStartArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	_, err = c.moduleBuildStartAction(args)
	return err
}

func (c *viamClient) moduleBuildStartForRepo(
	args moduleBuildStartArgs, manifest *moduleManifest, repo string,
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

	pm := GetProgressManager(c.c.Context)
	childSpinner := pm.AddSpinner("Starting cloud build...")
	childSpinner.UpdatePrefix("  → ")
	pm.Start()

	res, err := c.buildClient.StartBuild(c.c.Context, &req)
	if err != nil {
		childSpinner.ErrorWithMessage(fmt.Sprintf("Failed to start build: %s", err.Error()))
		return "", err
	}

	childSpinner.CompleteWithMessage(fmt.Sprintf("Build started (ID: %s)", res.BuildId))
	return res.BuildId, nil
}

func (c *viamClient) moduleBuildStartAction(args moduleBuildStartArgs) (string, error) {
	manifest, err := loadManifest(args.Module)
	if err != nil {
		return "", err
	}

	if manifest.URL == "" {
		return "", errors.New("meta.json must have a url field set in order to start a cloud build. " +
			"Ex: 'https://github.com/your-username/your-repo'")
	}

	// Create progress manager for build progress (but don't start yet)
	pm := NewProgressManager()
	defer func() {
		pm.Stop()
		pm.StopSignalHandler()
	}()

	// Add to context for sub-functions
	c.c.Context = WithProgressManager(c.c.Context, pm)

	return c.moduleBuildStartForRepo(args, &manifest, manifest.URL)
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
	ctx *cli.Context, moduleID moduleID, manifest *moduleManifest,
) error {
	pm := GetProgressManager(ctx.Context)
	_, err := c.getModule(moduleID)
	if err != nil {
		// Module is not registered in the cloud, prompt user for confirmation
		// Stop spinners before showing interactive prompt
		pm.Stop()

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
	spinner *ysmrr.Spinner,
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
	// Upload is handled by the calling function's spinner
	if err := sendUploadRequests(ctx.Context, stream, file, spinner, getNextReloadBuildUploadRequest); err != nil && !errors.Is(err, io.EOF) {
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

// moduleCloudReload triggers a cloud build and returns the download info for the built module.
func (c *viamClient) moduleCloudReload(
	ctx *cli.Context,
	args reloadModuleArgs,
	platform string,
	manifest moduleManifest,
	partID string,
) (*moduleDownloadInfo, error) {
	pm := GetProgressManager(ctx.Context)

	// ensure that the module has been registered in the cloud
	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return nil, err
	}

	// Preparing for Build (parent spinner)
	sPrepare := pm.AddSpinner("Preparing for build...")
	s1 := pm.AddSpinner("Ensuring module is registered...")
	s1.UpdatePrefix("  → ")
	err = c.ensureModuleRegisteredInCloud(ctx, moduleID, &manifest)
	if err != nil {
		s1.ErrorWithMessage(fmt.Sprintf("Registration failed: %s", err.Error()))
		sPrepare.Error()
		return nil, err
	}
	s1.Complete()

	id := ctx.String(generalFlagID)
	if id == "" {
		id = manifest.ModuleID
	}

	s2 := pm.AddSpinner("Creating source code archive...")
	s2.UpdatePrefix("  → ")
	archivePath, err := c.createGitArchive(args.Path)
	if err != nil {
		s2.ErrorWithMessage(fmt.Sprintf("Archive creation failed: %s", err.Error()))
		sPrepare.Error()
		return nil, err
	}
	s2.CompleteWithMessage(fmt.Sprintf("Source code archive created at %s", archivePath))

	infof(c.c.App.Writer, "Creating a new cloud build and swapping it onto the requested machine part. This may take a few minutes...")

	s3 := pm.AddSpinner("Triggering cloud reload build...")
	s3.UpdatePrefix("  → ")
	buildID, err := c.triggerCloudReloadBuild(ctx, args, manifest, archivePath, partID, s3)
	if err != nil {
		s3.ErrorWithMessage(fmt.Sprintf("Build trigger failed: %s", err.Error()))
		sPrepare.Error()
		return nil, err
	}
	s3.CompleteWithMessage(fmt.Sprintf("Cloud build %s started", buildID))

	// ensure the build completes before we try to dowload and use it
	s4 := pm.AddSpinner(fmt.Sprintf("Waiting for build %s to finish...", buildID))
	s4.UpdatePrefix("  → ")
	statuses, err := c.waitForBuildToFinish(buildID, platform)
	if err != nil {
		s4.ErrorWithMessage(fmt.Sprintf("Build wait failed: %s", err.Error()))
		sPrepare.Error()
		return nil, err
	}

	// if the build failed, print the logs and return an error
	if statuses[platform] == jobStatusFailed {
		s4.ErrorWithMessage(fmt.Sprintf("Build %s failed", buildID))
		sPrepare.Error()

		// Print error message without exiting (don't use Errorf since it calls os.Exit(1))
		errorf(c.c.App.Writer, "Build %q failed to complete. Please check the logs below for more information.", buildID)

		if err = c.printModuleBuildLogs(buildID, platform); err != nil {
			return nil, err
		}

		return nil, ErrReloadFailed
	}
	s4.CompleteWithMessage(fmt.Sprintf("Build %s completed successfully", buildID))
	sPrepare.Complete()

	// delete the archive we created
	if err := os.Remove(archivePath); err != nil {
		warningf(ctx.App.Writer, "failed to delete archive at %s", archivePath)
	}

	// Return the module download info so the download can happen in reloadModuleAction
	// This allows us to structure the download under "Reloading to Part..."
	return &moduleDownloadInfo{
		ID:       id,
		Version:  getReloadVersion(reloadVersionPrefix, partID),
		Platform: platform,
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

// reloadModuleAction is the testable inner reload logic.
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

	// Initialize progress manager (but don't start yet)
	pm := NewProgressManager()
	defer pm.Stop()

	// Set custom cancellation message for reload
	pm.SetCancellationMessage("Module reloading aborted by user." +
		" The current step will be completed if it was running in the cloud, and then the rest of the steps will be skipped.")

	// Add to context for sub-functions
	c.Context = WithProgressManager(c.Context, pm)

	// TODO(RSDK-9727) it'd be nice for this to be a method on a viam client rather than taking one as an arg
	partID, err := resolvePartID(args.PartID, args.CloudConfig)
	if err != nil {
		return err
	}

	// Add first spinner and start manager
	s2 := pm.AddSpinner("Loading and validating meta.json...")
	pm.Start()
	manifest, err := loadManifestOrNil(args.Module)
	if err != nil {
		return err
	}
	if !args.NoBuild {
		if manifest == nil {
			if args.Module != "meta.json" {
				s2.ErrorWithMessage(fmt.Sprintf("Failed to load meta.json: file not found at %s.", args.Module))
				return ErrReloadFailed
			}
			s2.ErrorWithMessage(fmt.Sprintf("Failed to load meta.json:" +
				"file not found in current directory. Please ensure you are within the directory of a module."))
			return ErrReloadFailed
		}
		s2.Complete()

		// Get robot part
		s3 := pm.AddSpinner("Fetching robot part...")
		part, err := vc.getRobotPart(partID)
		if err != nil {
			s3.ErrorWithMessage(fmt.Sprintf("Failed to fetch robot part: %s", err.Error()))
			return err
		}
		if part.Part == nil {
			s3.ErrorWithMessage(fmt.Sprintf("Part with id=%s not found", partID))
			return fmt.Errorf("part with id=%s not found", partID)
		}
		s3.Complete()

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
		var buildPath string
		if !args.NoBuild {
			if !cloudBuild {
				// Local build - stop spinner to show build logs cleanly
				sBuild := pm.AddSpinner("Building module locally...")
				sBuild.Complete()
				pm.Stop()

				// Run build (logs will print to stdout cleanly)
				err = moduleBuildLocalAction(c, manifest, environment)
				buildPath = manifest.Build.Path

				// Add build result spinner and start (Start automatically creates fresh manager after Stop)
				sBuildResult := pm.AddSpinner("Build result...")
				pm.Start()
				if err != nil {
					sBuildResult.ErrorWithMessage(fmt.Sprintf("Build failed: %s", err.Error()))
					return err
				}
				sBuildResult.CompleteWithMessage("Build complete")
			} else {
				// Cloud build - structured with parent spinners
				downloadInfo, err := vc.moduleCloudReload(c, args, platform, *manifest, partID)
				if err != nil {
					return err
				}

				// Reloading to Part (parent spinner - combines download, shell, copy, config, restart)
				sReload := pm.AddSpinner("Reloading to part...")

				sDownload := pm.AddSpinner("Downloading build artifact...")
				sDownload.UpdatePrefix("  → ")
				downloadArgs := downloadModuleFlags{
					ID:       downloadInfo.ID,
					Version:  downloadInfo.Version,
					Platform: downloadInfo.Platform,
				}
				buildPath, err := vc.downloadModuleAction(c, downloadArgs)
				if err != nil {
					sDownload.ErrorWithMessage(fmt.Sprintf("Download failed: %s", err.Error()))
					sReload.Error()
					return err
				}
				sDownload.Complete()

				sShell := pm.AddSpinner("Setting up shell service...")
				sShell.UpdatePrefix("  → ")
				shellAdded, err := addShellService(c, vc, part.Part, true)
				if err != nil {
					sShell.ErrorWithMessage(fmt.Sprintf("Failed to add shell service: %s", err.Error()))
					sReload.Error()
					return err
				}
				if shellAdded {
					sShell.CompleteWithMessage("Shell service added to part config")
				} else {
					sShell.CompleteWithMessage("Shell service already exists, skipped")
				}

				globalArgs, err := getGlobalArgs(c)
				if err != nil {
					sReload.Error()
					return err
				}
				dest := reloadingDestination(c, manifest)
				err = vc.copyFilesToFqdn(
					part.Part.Fqdn, globalArgs.Debug, false, false, []string{buildPath},
					dest, logger, args.NoProgress, "  → ")
				if err != nil {
					if s, ok := status.FromError(err); ok && s.Code() == codes.PermissionDenied {
						warningf(c.App.ErrWriter, "RDK couldn't write to the default file copy destination. "+
							"If you're running as non-root, try adding --home $HOME or --home /user/username to your CLI command. "+
							"Alternatively, run the RDK as root.")
					}
					sReload.Error()
					return fmt.Errorf("failed copying to part (%v): %w", dest, err)
				}

				// Continue with configuration under same parent spinner
				sConfigModule := pm.AddSpinner("Configuring module...")
				sConfigModule.UpdatePrefix("  → ")
				var newPart *apppb.RobotPart
				newPart, configUpdated, err := configureModule(c, vc, manifest, part.Part, args.Local)
				if newPart != nil {
					part.Part = newPart
				}
				if err != nil {
					sConfigModule.ErrorWithMessage(fmt.Sprintf("Configuration failed: %s", err.Error()))
					sReload.Error()
					return err
				}
				if configUpdated {
					sConfigModule.CompleteWithMessage("Module added to part config")
				} else {
					sConfigModule.CompleteWithMessage("Module already exists on part, skipped")
				}

				// If config was not updated, we need to manually restart the module
				// (if config was updated, RDK will auto-restart)
				if !configUpdated {
					sRestart := pm.AddSpinner("Restarting module...")
					sRestart.UpdatePrefix("  → ")
					if err = restartModule(c, vc, part.Part, manifest, logger); err != nil {
						sRestart.ErrorWithMessage(fmt.Sprintf("Restart failed: %s", err.Error()))
						sReload.Error()
						return err
					}
					sRestart.CompleteWithMessage("Module restarted successfully")
				}

				// Handle adding a resource/component if --model-name was specified
				if args.ModelName != "" {
					sResource := pm.AddSpinner(fmt.Sprintf("Adding resource/component %s...", args.ModelName))
					sResource.UpdatePrefix("  → ")
					if err = vc.addResourceFromModule(part.Part, manifest, args.ModelName, args.ResourceName); err != nil {
						sResource.ErrorWithMessage(fmt.Sprintf("Resource %s not added to part config", args.ModelName))
						warningf(c.App.ErrWriter, "unable to add requested resource to robot config: %s", err)
					} else {
						sResource.CompleteWithMessage(fmt.Sprintf("Resource %s added to part config", args.ModelName))
					}
				}
				sReload.Complete()
			}
		}
		if !args.Local && !cloudBuild {
			if manifest.Build == nil || buildPath == "" {
				return errors.New(
					"remote reloading requires a meta.json with the 'build.path' field set. " +
						"try --local if you are testing on the same machine.",
				)
			}
			if err := validateReloadableArchive(c, manifest.Build); err != nil {
				return err
			}

			// Reloading to Part (parent spinner) - for local builds
			sReload := pm.AddSpinner("Reloading to part...")

			sShell := pm.AddSpinner("Setting up shell service...")
			sShell.UpdatePrefix("  → ")
			shellAdded, err := addShellService(c, vc, part.Part, true)
			if err != nil {
				sShell.ErrorWithMessage(fmt.Sprintf("Failed to add shell service: %s", err.Error()))
				sReload.Error()
				return err
			}
			if shellAdded {
				sShell.CompleteWithMessage("Shell service added to part config")
			} else {
				sShell.CompleteWithMessage("Shell service already exists, skipped")
			}

			globalArgs, err := getGlobalArgs(c)
			if err != nil {
				sReload.Error()
				return err
			}
			dest := reloadingDestination(c, manifest)
			err = vc.copyFilesToFqdn(
				part.Part.Fqdn, globalArgs.Debug, false, false, []string{buildPath},
				dest, logger, args.NoProgress, "  → ")
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.PermissionDenied {
					warningf(c.App.ErrWriter, "RDK couldn't write to the default file copy destination. "+
						"If you're running as non-root, try adding --home $HOME or --home /user/username to your CLI command. "+
						"Alternatively, run the RDK as root.")
				}
				sReload.Error()
				return fmt.Errorf("failed copying to part (%v): %w", dest, err)
			}

			// Continue with configuration under same parent spinner
			sConfigModule := pm.AddSpinner("Configuring module...")
			sConfigModule.UpdatePrefix("  → ")
			var newPart *apppb.RobotPart
			newPart, configUpdated, err := configureModule(c, vc, manifest, part.Part, args.Local)
			if newPart != nil {
				part.Part = newPart
			}
			if err != nil {
				sConfigModule.ErrorWithMessage(fmt.Sprintf("Configuration failed: %s", err.Error()))
				sReload.Error()
				return err
			}
			if configUpdated {
				sConfigModule.CompleteWithMessage("Module added to part config")
			} else {
				sConfigModule.CompleteWithMessage("Module already exists on part, skipped")
			}

			// If config was not updated, we need to manually restart the module
			// (if config was updated, RDK will auto-restart)
			if !configUpdated {
				sRestart := pm.AddSpinner("Restarting module...")
				sRestart.UpdatePrefix("  → ")
				if err = restartModule(c, vc, part.Part, manifest, logger); err != nil {
					sRestart.ErrorWithMessage(fmt.Sprintf("Restart failed: %s", err.Error()))
					sReload.Error()
					return err
				}
				sRestart.CompleteWithMessage("Module restarted successfully")
			}

			// Handle adding a resource/component if --model-name was specified
			if args.ModelName != "" {
				sResource := pm.AddSpinner(fmt.Sprintf("Adding resource %s...", args.ModelName))
				sResource.UpdatePrefix("  → ")
				if err = vc.addResourceFromModule(part.Part, manifest, args.ModelName, args.ResourceName); err != nil {
					sResource.ErrorWithMessage(fmt.Sprintf("Resource %s not added to part config", args.ModelName))
					warningf(c.App.ErrWriter, "unable to add requested resource to robot config: %s", err)
				} else {
					sResource.CompleteWithMessage(fmt.Sprintf("Resource %s added to part config", args.ModelName))
				}
			}
			sReload.Complete()
		}
	} else {
		infof(c.App.Writer, "Reload complete")
	}

	return nil
}

func getReloadVersion(versionPrefix, partID string) string {
	return versionPrefix + "-" + partID
}

// reload with cloudbuild was supported starting in 0.90.0
// there are older versions of viam-servet that don't support ~/ file prefix, so lets avoid using them.
var reloadVersionSupported = semver.MustParse("0.90.0")

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
	err = robotClient.RestartModule(c.Context, *restartReq)

	return err
}
