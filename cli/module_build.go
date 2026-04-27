package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	v1 "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
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

// githubRefExists calls GitHub's REST commits API to check whether a ref
// (branch, tag, or commit SHA) exists in a repo
var githubRefExists = func(ctx context.Context, owner, repo, ref, token string) (bool, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status %d from github api", resp.StatusCode)
	}
}

// parseGitHubRepo extracts owner and repo from an https://github.com/owner/repo
// URL. If it fails to parse, it might be formatted non-uniformly, so we try the build action anyways.
func parseGitHubRepo(repoURL string) (owner, repo string, ok bool, err error) {
	u, parseErr := url.Parse(repoURL)
	// not a github link: skip validation, the build action may still succeed on a non-github host
	if parseErr != nil || u.Host != "github.com" {
		return "", "", false, nil //nolint:nilerr
	}
	// strip leading / from path and then get first three parts (owner, repo, path)
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 3)
	// github url but missing owner/repo: the cloud build will definitely fail, so hard-fail early
	if len(parts) < 2 || parts[1] == "" {
		return "", "", false, fmt.Errorf(
			"meta.json url %q is missing the repo path (expected https://github.com/<owner>/<repo>)", repoURL)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true, nil
}

// validateRefExists checks that ref exists on the remote at repoURL before a
// cloud build is started, and only stops the build if the ref can be proven to not exist, or
// else it will go through to with the build attempt (like with non-github links)
func (c *viamClient) validateRefExists(ctx context.Context, cmd *cli.Command, repoURL, ref, token string) error {
	owner, repo, ok, err := parseGitHubRepo(repoURL)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	exists, err := githubRefExists(ctx, owner, repo, ref, token)
	if err != nil {
		gArgs := parseStructFromCtx[globalArgs](cmd)
		debugf(cmd.Root().ErrWriter, gArgs.Debug,
			"could not verify ref %q on %s: %v — proceeding anyway", ref, repoURL, err)
		return nil
	}
	if !exists {
		if token == "" {
			return fmt.Errorf("ref %q not found on %s (if this is a private repo, pass a token with --token)", ref, repoURL)
		}
		return fmt.Errorf("ref %q not found on %s", ref, repoURL)
	}
	return nil
}

type moduleBuildStartArgs struct {
	Module    string
	Version   string
	Ref       string
	Token     string
	Workdir   string
	Platforms []string
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(ctx context.Context, cmd *cli.Command, args moduleBuildStartArgs) error {
	c, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	_, err = c.moduleBuildStartAction(ctx, cmd, args)
	return err
}

func (c *viamClient) moduleBuildStartForRepo(
	ctx context.Context, cmd *cli.Command, args moduleBuildStartArgs, manifest *ModuleManifest, repo string,
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
		Distro:        &manifest.Build.Distro,
	}
	res, err := c.buildClient.StartBuild(ctx, &req)
	if err != nil {
		return "", err
	}
	// Print to stderr so that stdout only contains the buildID, which is parsed by the build-action.
	// See https://github.com/viamrobotics/build-action/blob/main/src/index.js
	printf(cmd.Root().ErrWriter, "Build started, follow the logs with:")
	printf(cmd.Root().ErrWriter, "	viam module build logs --id %s", res.BuildId)
	printf(cmd.Root().Writer, res.BuildId)
	return res.BuildId, nil
}

func (c *viamClient) moduleBuildStartAction(ctx context.Context, cmd *cli.Command, args moduleBuildStartArgs) (string, error) {
	manifest, err := loadManifest(args.Module)
	if err != nil {
		return "", err
	}

	// Check if this is a Windows Python module by looking for src/main.py
	if runtime.GOOS == osWindows && manifest.Build != nil {
		manifestDir := filepath.Dir(args.Module)
		mainPyPath := filepath.Join(manifestDir, "src", "main.py")
		if _, err := os.Stat(mainPyPath); err == nil {
			return "", errors.New("cloud build is not currently supported for Windows Python modules.\n" +
				"Build locally with 'viam module build local' and upload with 'viam module upload'")
		}
	}

	if manifest.URL == "" {
		return "", errors.New("meta.json must have a url field set in order to start a cloud build. " +
			"Ex: 'https://github.com/your-username/your-repo'")
	}

	if err := c.validateRefExists(ctx, cmd, manifest.URL, args.Ref, args.Token); err != nil {
		return "", err
	}

	return c.moduleBuildStartForRepo(ctx, cmd, args, &manifest, manifest.URL)
}

type moduleBuildLocalArgs struct {
	Module string
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(ctx context.Context, cmd *cli.Command, args moduleBuildLocalArgs) error {
	manifestPath := args.Module
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	return moduleBuildLocalAction(ctx, cmd, &manifest, nil)
}

func moduleBuildLocalAction(ctx context.Context, cmd *cli.Command, manifest *ModuleManifest, environment map[string]string) error {
	if manifest.Build == nil || manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}
	infof(cmd.Root().Writer, "Starting build")

	// Use cmd.exe on Windows, bash on Unix-like systems
	shellName := "bash"
	shellFlag := "-c"
	if runtime.GOOS == osWindows {
		shellName = "cmd.exe"
		shellFlag = "/C"
	}

	// Build environment slice from map, inheriting current environment
	env := os.Environ()
	for k, v := range environment {
		env = append(env, k+"="+v)
	}

	if manifest.Build.Setup != "" {
		infof(cmd.Root().Writer, "Starting setup step: %q", manifest.Build.Setup)
		//nolint:gosec // user-provided build commands from meta.json are intentionally executed
		setupCmd := exec.CommandContext(ctx, shellName, shellFlag, manifest.Build.Setup)
		setupCmd.Env = env
		setupCmd.Stdout = cmd.Root().Writer
		setupCmd.Stderr = cmd.Root().Writer
		if err := setupCmd.Run(); err != nil {
			return err
		}
	}
	infof(cmd.Root().Writer, "Starting build step: %q", manifest.Build.Build)
	//nolint:gosec // user-provided build commands from meta.json are intentionally executed
	buildCmd := exec.CommandContext(ctx, shellName, shellFlag, manifest.Build.Build)
	buildCmd.Env = env
	buildCmd.Stdout = cmd.Root().Writer
	buildCmd.Stderr = cmd.Root().Writer
	if err := buildCmd.Run(); err != nil {
		return err
	}
	infof(cmd.Root().Writer, "Completed build")
	return nil
}

type moduleBuildListArgs struct {
	Module string
	Count  int
	ID     string
}

// ModuleBuildListAction lists the module's build jobs.
func ModuleBuildListAction(ctx context.Context, cmd *cli.Command, args moduleBuildListArgs) error {
	c, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return c.moduleBuildListAction(ctx, cmd, args)
}

func (c *viamClient) moduleBuildListAction(ctx context.Context, cmd *cli.Command, args moduleBuildListArgs) error {
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

	jobs, err := c.listModuleBuildJobs(ctx, moduleIDFilter, numberOfJobsToReturn, buildID)
	if err != nil {
		return err
	}
	// table format rules:
	// minwidth, tabwidth, padding int, padchar byte, flags uint
	w := tabwriter.NewWriter(cmd.Root().Writer, 5, 4, 1, ' ', 0)
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
	//nolint: errcheck
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
func ModuleBuildLogsAction(ctx context.Context, cmd *cli.Command, args moduleBuildLogsArgs) error {
	buildID := args.ID
	platform := args.Platform
	shouldWait := args.Wait
	groupLogs := args.GroupLogs

	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	var statuses map[string]jobStatus
	if shouldWait {
		statuses, err = client.waitForBuildToFinish(ctx, buildID, platform, nil)
		if err != nil {
			return err
		}
	}
	if platform != "" {
		if err := client.printModuleBuildLogs(ctx, buildID, platform); err != nil {
			return err
		}
	} else {
		platforms, err := client.getPlatformsForModuleBuild(ctx, buildID)
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
			infof(cmd.Root().Writer, "Logs for %q", platform)
			err := client.printModuleBuildLogs(ctx, buildID, platform)
			if err != nil {
				combinedErr = multierr.Combine(combinedErr, client.printModuleBuildLogs(ctx, buildID, platform))
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
func ModuleBuildLinkRepoAction(ctx context.Context, cmd *cli.Command, args moduleBuildLinkRepoArgs) error {
	linkID := args.OAuthLink
	moduleID := args.Module
	repo := args.Repo

	if moduleID == "" {
		manifest, err := loadManifestOrNil(defaultManifestFilename)
		if err != nil {
			return fmt.Errorf("this command needs a module ID from either %s flag or valid %s", moduleFlagPath, defaultManifestFilename)
		}
		moduleID = manifest.ModuleID
		infof(cmd.Root().ErrWriter, "using module ID %s from %s", moduleID, defaultManifestFilename)
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
		infof(cmd.Root().ErrWriter, "using repo %s from current folder", repo)
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

	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	res, err := client.buildClient.LinkRepo(ctx, &req)
	if err != nil {
		return err
	}
	infof(cmd.Root().Writer, "Successfully created link with ID %s", res.RepoLinkId)
	return nil
}

func (c *viamClient) printModuleBuildLogs(ctx context.Context, buildID, platform string) error {
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
			infof(c.c.Root().Writer, log.BuildStep)
			lastBuildStep = log.BuildStep
		}
		fmt.Fprint(c.c.Root().Writer, log.Data) //nolint:errcheck // data is already formatted with newlines
	}

	return nil
}

func (c *viamClient) listModuleBuildJobs(
	ctx context.Context, moduleIDFilter string, count *int32, buildIDFilter *string,
) (*buildpb.ListJobsResponse, error) {
	req := buildpb.ListJobsRequest{
		ModuleId:      moduleIDFilter,
		MaxJobsLength: count,
		BuildId:       buildIDFilter,
	}
	return c.buildClient.ListJobs(ctx, &req)
}

// waitForBuildToFinish calls listModuleBuildJobs every moduleBuildPollingInterval
// Will wait until the status of the specified job is DONE or FAILED
// if platform is empty, it waits for all jobs associated with the ID.
// If pm is not nil, it will show progress spinners for each build step.
func (c *viamClient) waitForBuildToFinish(
	ctx context.Context,
	buildID string,
	platform string,
	pm *ProgressManager,
) (map[string]jobStatus, error) {
	// If the platform is not empty, we should check that the platform is actually present on the build
	// this is mostly to protect against users misspelling the platform
	if platform != "" {
		platformsForBuild, err := c.getPlatformsForModuleBuild(ctx, buildID)
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
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			jobsResponse, err := c.listModuleBuildJobs(ctx, "", nil, &buildID)
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

func (c *viamClient) getPlatformsForModuleBuild(ctx context.Context, buildID string) ([]string, error) {
	platforms := []string{}
	jobsResponse, err := c.listModuleBuildJobs(ctx, "", nil, &buildID)
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

	// CloudConfig is a path to the `viam.json`, or the config containing the robot ID.
	CloudConfig  string
	ModelName    string
	Workdir      string
	ResourceName string
	Path         string
	Annotation   string
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
			if c.shouldIgnore(relPath, matcher, true) {
				return filepath.SkipDir
			}
			return nil
		}

		if c.shouldIgnore(relPath, matcher, false) {
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

	// Recursively read .gitignore files from the repo tree. ReadPatterns walks
	// subdirectories, respects already-matched ignore rules (so it won't descend
	// into e.g. node_modules), and attaches the correct domain to each pattern so
	// that nested .gitignore rules are scoped to their directory.
	// If no .git directory exists, ReadPatterns still reads .gitignore files.
	fs := osfs.New(repoPath)
	repoPatterns, err := gitignore.ReadPatterns(fs, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read gitignore patterns")
	}
	patterns = append(patterns, repoPatterns...)

	return gitignore.NewMatcher(patterns), nil
}

func (c *viamClient) shouldIgnore(relPath string, matcher gitignore.Matcher, isDir bool) bool {
	normalizedPath := filepath.ToSlash(relPath)
	return matcher.Match(strings.Split(normalizedPath, "/"), isDir)
}

//nolint:unused
func (c *viamClient) ensureModuleRegisteredInCloud(
	ctx context.Context, cmd *cli.Command, moduleID moduleID, pm *ProgressManager,
) error {
	_, err := c.getModule(ctx, moduleID)
	if err != nil {
		// Module is not registered in the cloud, prompt user for confirmation
		// Stop the spinner before prompting for user input to avoid interference
		// with the interactive prompt.
		if pm != nil {
			pm.Stop()
		}

		red := "\033[1;31m%s\033[0m"
		printf(cmd.Root().Writer, red, "Error: module not registered in cloud or you lack permissions to edit it.")

		yellow := "\033[1;33m%s\033[0m"
		printf(cmd.Root().Writer, yellow, "Info: The reloading process requires the module to first be registered in the cloud. "+
			"Do you want to proceed with module registration?")
		printf(cmd.Root().Writer, "Continue: y/n: ")
		if err := ctx.Err(); err != nil {
			return err
		}

		rawInput, err := bufio.NewReader(cmd.Root().Reader).ReadString('\n')
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

		org, err := getOrgByModuleIDPrefix(ctx, c, moduleID.prefix)
		if err != nil {
			return err
		}
		// Create the module in the cloud
		_, err = c.createModule(ctx, moduleID.name, org.GetId())
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *viamClient) getOrgIDForPart(ctx context.Context, part *apppb.RobotPart) (string, error) {
	robot, err := c.client.GetRobot(ctx, &apppb.GetRobotRequest{
		Id: part.GetRobot(),
	})
	if err != nil {
		return "", err
	}

	location, err := c.client.GetLocation(ctx, &apppb.GetLocationRequest{
		LocationId: robot.Robot.GetLocation(),
	})
	if err != nil {
		return "", err
	}

	orgID := location.Location.PrimaryOrgIdentity.GetId()

	if orgID == "" {
		return "", errors.New("no primary org id found for location")
	}

	return orgID, nil
}

func (c *viamClient) triggerCloudReloadBuild(
	ctx context.Context,
	cmd *cli.Command,
	args reloadModuleArgs,
	manifest ModuleManifest,
	archivePath, partID string,
	reloadUnixTS int64,
) (string, error) {
	stream, err := c.buildClient.StartReloadBuild(ctx)
	if err != nil {
		return "", err
	}

	//nolint:gosec
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}

	part, err := c.getRobotPart(ctx, partID)
	if err != nil {
		return "", err
	}
	if part.Part == nil {
		return "", fmt.Errorf("part with id=%s not found", partID)
	}

	if part.Part.UserSuppliedInfo == nil {
		return "", errors.New("unable to determine platform for part")
	}

	// use the primary org id for the machine as the reload
	// module org
	orgID, err := c.getOrgIDForPart(ctx, part.Part)
	if err != nil {
		return "", err
	}

	// App expects `BuildInfo` as the first request
	platform := part.Part.UserSuppliedInfo.Fields["platform"].GetStringValue()
	req := &buildpb.StartReloadBuildRequest{
		CloudBuild: &buildpb.StartReloadBuildRequest_BuildInfo{
			BuildInfo: &buildpb.ReloadBuildInfo{
				Platform: platform,
				Workdir:  &args.Workdir,
				ModuleId: manifest.ModuleID,
				Distro:   &manifest.Build.Distro,
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
		Version:        getReloadVersion(reloadSourceVersionPrefix, partID, reloadUnixTS),
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
		ctx, stream, file, io.Discard, getNextReloadBuildUploadRequest); err != nil && !errors.Is(err, io.EOF) {
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
	ModuleID    string
	Version     string
	Platform    string
	ArchivePath string // Path to the temporary archive that should be deleted after download
	OrgID       string
}

// moduleCloudReload triggers a cloud build and returns info needed to download the artifact.
func (c *viamClient) moduleCloudReload(
	ctx context.Context,
	cmd *cli.Command,
	args reloadModuleArgs,
	platform string,
	manifest ModuleManifest,
	partID string,
	pm *ProgressManager,
	reloadUnixTS int64,
) (*moduleCloudBuildInfo, error) {
	// Start the "Preparing for build..." parent step (prints as header)
	if err := pm.Start("prepare"); err != nil {
		return nil, err
	}

	part, err := c.getRobotPart(ctx, partID)
	if err != nil {
		return nil, err
	}
	if part.Part == nil {
		return nil, fmt.Errorf("part with id=%s not found", partID)
	}
	orgID, err := c.getOrgIDForPart(ctx, part.Part)
	if err != nil {
		return nil, err
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
	buildID, err := c.triggerCloudReloadBuild(ctx, cmd, args, manifest, archivePath, partID, reloadUnixTS)
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
	statuses, err := c.waitForBuildToFinish(ctx, buildID, platform, pm)
	if err != nil {
		_ = pm.FailWithMessage("build", "Building...") //nolint:errcheck
		return nil, err
	}

	// if the build failed, print the logs and return an error
	if statuses[platform] == jobStatusFailed {
		_ = pm.FailWithMessage("build", "Building...") //nolint:errcheck

		// Print error message without exiting (don't use Errorf since it calls os.Exit(1))
		errorf(c.c.Root().Writer, "Build %q failed to complete. Please check the logs below for more information.", buildID)

		if err = c.printModuleBuildLogs(ctx, buildID, platform); err != nil {
			return nil, err
		}

		return nil, errors.Errorf("Reloading module failed")
	}
	// Note: The "build" parent step will be completed by the caller after downloading artifacts

	// Return build info so the caller can download the artifact with a spinner
	return &moduleCloudBuildInfo{
		ModuleID:    manifest.ModuleID,
		OrgID:       orgID,
		Version:     getReloadVersion(reloadVersionPrefix, partID, reloadUnixTS),
		Platform:    platform,
		ArchivePath: archivePath,
	}, nil
}

// IsReloadVersion checks if the version is a reload version.
func IsReloadVersion(version string) bool {
	return strings.HasPrefix(version, reloadVersionPrefix)
}

// ReloadModuleLocalAction builds a module locally, configures it on a robot, and starts or restarts it.
func ReloadModuleLocalAction(ctx context.Context, cmd *cli.Command, args reloadModuleArgs) error {
	return reloadModuleAction(ctx, cmd, args, false)
}

// ReloadModuleAction builds a module, configures it on a robot, and starts or restarts it.
func ReloadModuleAction(ctx context.Context, cmd *cli.Command, args reloadModuleArgs) error {
	return reloadModuleAction(ctx, cmd, args, true)
}

func reloadModuleAction(ctx context.Context, cmd *cli.Command, args reloadModuleArgs, cloudBuild bool) error {
	vc, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	// Create logger based on presence of debugFlag.
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())
	globalArgs, err := getGlobalArgs(cmd)
	if err != nil {
		return err
	}
	if globalArgs.Debug {
		logger = logging.NewDebugLogger("cli")
	}

	return reloadModuleActionInner(ctx, cmd, vc, args, logger, cloudBuild)
}

func getReloadVersion(versionPrefix, partID string, unixTS int64) string {
	return fmt.Sprintf("%s-%s-%d", versionPrefix, partID, unixTS)
}

// reload with cloudbuild was supported starting in 0.90.0
// there are older versions of viam-servet that don't support ~/ file prefix, so lets avoid using them.
var reloadVersionSupported = semver.MustParse("0.90.0")

// reloadModuleActionInner is the testable inner reload logic.
func reloadModuleActionInner(
	ctx context.Context,
	cmd *cli.Command,
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
	part, err := vc.getRobotPart(ctx, partID)
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
	// Compute reload time once, used for both the package version and config reload_time
	reloadTime := time.Now().UTC()

	// note: configureModule and restartModule signal the robot via different channels.
	// Running this command in rapid succession can cause an extra restart because the
	// CLI will see configuration changes before the robot, and skip to the needsRestart
	// case on the second call. Because these are triggered by user actions, we're okay
	// with this behavior, and the robot will eventually converge to what is in config.

	// Define all steps upfront (build + reload) with clear parent/child relationships.
	// Cloud builds skip download/shell/upload since the machine downloads directly from cloud.
	var allSteps []*Step
	if cloudBuild {
		allSteps = []*Step{
			{ID: "prepare", Message: "Preparing for build...", CompletedMsg: "Prepared for build", IndentLevel: 0},
			{ID: "archive", Message: "Creating source code archive...", CompletedMsg: "Source code archive created", IndentLevel: 1},
			{ID: "upload-source", Message: "Uploading source code...", CompletedMsg: "Source code uploaded", IndentLevel: 1},
			{ID: "build", Message: "Building...", CompletedMsg: "Built", IndentLevel: 0},
			{ID: "build-start", Message: "Starting build...", IndentLevel: 1},
			// Dynamic build steps (e.g., "Spin up environment", "Install dependencies") are added at runtime with IndentLevel: 1
			{ID: "reload", Message: "Reloading to part...", CompletedMsg: "Reloaded to part", IndentLevel: 0},
			{ID: "configure", Message: "Configuring module...", CompletedMsg: "Module configured", IndentLevel: 1},
			{ID: "resource", Message: "Adding resource...", CompletedMsg: "Resource added", IndentLevel: 1},
		}
	} else {
		allSteps = []*Step{
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
	}

	pm := NewProgressManager(allSteps, WithProgressOutput(!args.NoProgress))
	defer pm.Stop()

	if len(args.PartID) == 0 && !args.NoProgress {
		printf(cmd.Root().ErrWriter, "Reloading to the machine configured at %s", args.CloudConfig)
	}

	var needsRestart bool
	var buildPath string
	var buildInfo *moduleCloudBuildInfo
	if !args.NoBuild {
		if manifest == nil {
			return fmt.Errorf(`manifest not found at "%s". manifest required for build`, moduleFlagPath)
		}
		if manifest.Build == nil || manifest.Build.Build == "" {
			return errors.New("your meta.json cannot have an empty build step. It is required for 'reload' and 'reload-local' commands")
		}
		if !cloudBuild {
			err = moduleBuildLocalAction(ctx, cmd, manifest, environment)
			buildPath = manifest.Build.Path
		} else {
			buildInfo, err = vc.moduleCloudReload(ctx, cmd, args, platform, *manifest, partID, pm, reloadTime.Unix())
			if err != nil {
				return err
			}

			// Complete the build phase before starting reload
			if err := pm.Complete("build"); err != nil {
				return err
			}

			// For cloud builds, the machine downloads the package directly from the cloud.
			// No need to download the artifact or copy it via shell service.

			// Delete the archive we created
			if err := os.Remove(buildInfo.ArchivePath); err != nil {
				warningf(cmd.Root().Writer, "failed to delete archive at %s", buildInfo.ArchivePath)
			}
		}
		if err != nil {
			return err
		}
	} else {
		// --no-build flag is set, look for existing artifact (only for reload-local)
		if manifest == nil || manifest.Build == nil {
			return fmt.Errorf(`manifest not found at "%s". manifest required for reload`, moduleFlagPath)
		}
		buildPath = manifest.Build.Path
	}

	// For cloud builds, the machine downloads the package directly from the cloud.
	// Skip the shell copy and go straight to configure.
	if cloudBuild {
		if err := pm.Start("reload"); err != nil {
			return err
		}
	} else if !args.Local {
		if manifest == nil || manifest.Build == nil || buildPath == "" {
			return errors.New(
				"remote reloading requires a meta.json with the 'build.path' field set. " +
					"try --local if you are testing on the same machine.",
			)
		}
		if err := validateReloadableArchive(cmd, manifest.Build); err != nil {
			return err
		}

		if err := pm.Start("reload"); err != nil {
			return err
		}
		if err := pm.Start("shell"); err != nil {
			return err
		}
		shellAdded, err := addShellService(ctx, cmd, vc, logger, part.Part, true)
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

		globalArgs, err := getGlobalArgs(cmd)
		if err != nil {
			return err
		}
		dest := reloadingDestination(cmd, manifest)

		if err := pm.Start("upload"); err != nil {
			return err
		}
		copyFunc := func() error {
			return vc.copyFilesToFqdn(
				ctx,
				part.Part.Fqdn,
				globalArgs.Debug,
				false, // allowRecursion
				false, // preserve
				[]string{buildPath},
				dest,
				logger,
				true, // noProgress
			)
		}
		attemptCount, err := vc.retryableCopy(
			cmd,
			pm,
			copyFunc,
			false,
		)
		if err != nil {
			_ = pm.Fail("upload", err)                               //nolint:errcheck
			_ = pm.FailWithMessage("reload", "Reloading to part...") //nolint:errcheck
			return fmt.Errorf("all %d copy attempts failed. You can retry the copy later, "+
				"skipping the build step with: viam module reload --no-build --part-id %s", attemptCount, partID)
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
	newPart, needsRestart, err = configureModule(
		ctx, cmd, vc, manifest, part.Part, args.Local, cloudBuild, reloadUser(vc.conf), args.Annotation, reloadTime.Unix())
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
		if err = restartModule(ctx, cmd, vc, part.Part, manifest, logger); err != nil {
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
		if err = vc.addResourceFromModule(cmd, part.Part, manifest, args.ModelName, args.ResourceName); err != nil {
			_ = pm.FailWithMessage("resource", fmt.Sprintf("Failed to add resource: %v", err)) //nolint:errcheck
			warningf(cmd.Root().ErrWriter, "unable to add requested resource to robot config: %s", err)
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
func reloadingDestination(cmd *cli.Command, manifest *ModuleManifest) string {
	args := parseStructFromCtx[reloadingDestinationArgs](cmd)
	return filepath.Join(args.Home,
		".viam", config.PackagesDirName+config.LocalPackagesSuffix,
		utils.SanitizePath(localizeModuleID(manifest.ModuleID)+"-"+manifest.Build.Path))
}

// validateReloadableArchive returns an error if there is a fatal issue (for now just file not found).
// It also logs warnings for likely problems.
func validateReloadableArchive(cmd *cli.Command, build *manifestBuildInfo) error {
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
		warningf(cmd.Root().ErrWriter, "archive at %s doesn't contain a meta.json, your module will probably fail to start", build.Path)
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
		return "", fmt.Errorf("did not receive part ID and no cloud config found at %s. "+
			"Provide --part-id or run this on a machine where viam-server has stored its cloud config", cloudJSON)
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
func resolveTargetModule(cmd *cli.Command, manifest *ModuleManifest) (*robot.RestartModuleRequest, error) {
	args := parseStructFromCtx[resolveTargetModuleArgs](cmd)
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
func ModuleRestartAction(ctx context.Context, cmd *cli.Command, args moduleRestartArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	partID, err := resolvePartID(args.PartID, args.CloudConfig)
	if err != nil {
		return err
	}

	part, err := client.getRobotPart(ctx, partID)
	if err != nil {
		return err
	}

	manifest, err := loadManifestOrNil(args.Module)
	if err != nil {
		return err
	}
	logger := logging.FromZapCompatible(zap.NewNop().Sugar())

	return restartModule(ctx, cmd, client, part.Part, manifest, logger)
}

// restartModule restarts a module on a robot.
func restartModule(
	ctx context.Context,
	cmd *cli.Command,
	vc *viamClient,
	part *apppb.RobotPart,
	manifest *ModuleManifest,
	logger logging.Logger,
) error {
	// TODO(RSDK-9727) it'd be nice for this to be a method on a viam client rather than taking one as an arg
	restartReq, err := resolveTargetModule(cmd, manifest)
	if err != nil {
		return err
	}
	apiRes, err := vc.client.GetRobotAPIKeys(ctx, &apppb.GetRobotAPIKeysRequest{RobotId: part.Robot})
	if err != nil {
		return err
	}
	if len(apiRes.ApiKeys) == 0 {
		return errors.New("API keys list for this machine is empty. You can create one with \"viam machine api-key create\"")
	}
	key := apiRes.ApiKeys[0]
	args, err := getGlobalArgs(cmd)
	if err != nil {
		return err
	}
	debugf(cmd.Root().Writer, args.Debug, "using API key: %s %s", key.ApiKey.Id, key.ApiKey.Name)
	creds := rpc.WithEntityCredentials(key.ApiKey.Id, rpc.Credentials{
		Type:    rpc.CredentialsTypeAPIKey,
		Payload: key.ApiKey.Key,
	})
	robotClient, err := client.New(ctx, part.Fqdn, logger, client.WithDialOptions(creds))
	if err != nil {
		return err
	}
	defer robotClient.Close(ctx) //nolint: errcheck
	debugf(cmd.Root().Writer, args.Debug, "restarting module %v", restartReq)
	// todo: make this a stream so '--wait' can tell user what's happening
	return robotClient.RestartModule(ctx, *restartReq)
}
