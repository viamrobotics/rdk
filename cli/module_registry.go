package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	apppb "go.viam.com/api/app/v1"
	vutils "go.viam.com/utils"

	modconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/modmanager"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/utils"
)

// moduleUploadChunkSize sets the number of bytes included in each chunk of the upload stream.
var moduleUploadChunkSize = 32 * 1024

// moduleVisibility determines whether modules are public or private.
type moduleVisibility string

// Permissions enumeration.
const (
	moduleVisibilityPrivate moduleVisibility = "private"
	moduleVisibilityPublic  moduleVisibility = "public"
)

// ModuleComponent represents an api - model pair.
type ModuleComponent struct {
	API   string `json:"api"`
	Model string `json:"model"`
}

// moduleID represents a prefix:name pair where prefix can be either an org id or a namespace.
type moduleID struct {
	prefix string
	name   string
}

// manifestBuildInfo is the "build" section of meta.json.
type manifestBuildInfo struct {
	Build string   `json:"build"`
	Setup string   `json:"setup"`
	Path  string   `json:"path"`
	Arch  []string `json:"arch"`
}

// defaultBuildInfo has defaults for unset fields in "build".
//

var defaultBuildInfo = manifestBuildInfo{
	Build: "make module.tar.gz",
	Path:  "module.tar.gz",
	Arch:  []string{"linux/amd64", "linux/arm64"},
}

// moduleManifest is used to create & parse manifest.json.
type moduleManifest struct {
	// for backward compatibility - DO NOT SET as will be deprecated
	Name        string            `json:"name,omitempty"`
	ModuleID    string            `json:"module_id"`
	Visibility  moduleVisibility  `json:"visibility"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Models      []ModuleComponent `json:"models"`
	Entrypoint  string            `json:"entrypoint"`
	Build       manifestBuildInfo `json:"build"`
}

const (
	defaultManifestFilename = "meta.json"
)

// CreateModuleAction is the corresponding Action for 'module create'. It runs
// the command to create a module. This includes both a gRPC call to register
// the module on app.viam.com and creating the manifest file.
func CreateModuleAction(c *cli.Context) error {
	moduleNameArg := c.String(moduleFlagName)
	publicNamespaceArg := c.String(moduleFlagPublicNamespace)
	orgIDArg := c.String(moduleFlagOrgID)

	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}
	// Check to make sure the user doesn't accidentally overwrite a module manifest
	if _, err := os.Stat(defaultManifestFilename); err == nil {
		return errors.New("another module's meta.json already exists in the current directory. Delete it and try again")
	}

	response, err := client.createModule(moduleNameArg, org.GetId())
	if err != nil {
		return errors.Wrap(err, "failed to register the module on app.viam.com")
	}

	returnedModuleID, err := parseModuleID(response.GetModuleId())
	if err != nil {
		return err
	}

	printf(c.App.Writer, "Successfully created '%s'", returnedModuleID.String())
	if response.GetUrl() != "" {
		printf(c.App.Writer, "You can view it here: %s", response.GetUrl())
	}
	emptyManifest := moduleManifest{
		ModuleID:   returnedModuleID.String(),
		Visibility: moduleVisibilityPrivate,
		// This is done so that the json has an empty example
		Models: []ModuleComponent{
			{},
		},
		// TODO(RSDK-5608) don't auto populate until we are ready to release the build subcommand
		// Build: defaultBuildInfo,
	}
	if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
		return err
	}
	printf(c.App.Writer, "Configuration for the module has been written to meta.json\n")
	return nil
}

// UpdateModuleAction is the corresponding Action for 'module update'. It runs
// the command to update a module. This includes updating the meta.json to
// include the public namespace (if set on the org).
func UpdateModuleAction(c *cli.Context) error {
	publicNamespaceArg := c.String(moduleFlagPublicNamespace)
	orgIDArg := c.String(moduleFlagOrgID)
	var moduleID moduleID

	manifestPath := c.String(moduleFlagPath)

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}

	// for backwards compatibility this could be empty
	if manifest.ModuleID != "" {
		moduleID, err = validateModuleID(c, client, manifest.ModuleID, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
	} else {
		moduleID, err = validateModuleID(c, client, manifest.Name, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
	}

	response, err := client.updateModule(moduleID, manifest)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Module successfully updated! You can view your changes online here: %s", response.GetUrl())

	// if we have gotten this far it means that moduleID will have a prefix in it
	// because the validate command resolves the orgId or namespace to the moduleID with the namespace as the priority

	// TODO: Will remove in a few week
	if manifest.Name != "" || manifest.ModuleID == "" {
		manifest.Name = ""
		manifest.ModuleID = moduleID.String()
		if err := writeManifest(manifestPath, manifest); err != nil {
			return errors.Wrap(err, "failed to update meta.json with new information from Viam")
		}
	}

	return nil
}

// UploadModuleAction is the corresponding action for 'module upload'.
func UploadModuleAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	publicNamespaceArg := c.String(moduleFlagPublicNamespace)
	orgIDArg := c.String(moduleFlagOrgID)
	nameArg := c.String(moduleFlagName)
	versionArg := c.String(moduleFlagVersion)
	platformArg := c.String(moduleFlagPlatform)
	forceUploadArg := c.Bool(moduleFlagForce)
	moduleUploadPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"Make sure to specify flag and optional arguments before the required positional package argument")
	}
	if moduleUploadPath == "" {
		return errors.New("nothing to upload -- please provide a path to your module. Use --help for more information")
	}

	// Clean the version argument to ensure compatibility with github tag standards
	versionArg = strings.TrimPrefix(versionArg, "v")

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	var moduleID moduleID
	// if the manifest cant be found
	if _, err := os.Stat(manifestPath); err != nil {
		// no manifest found.
		if nameArg == "" || (publicNamespaceArg == "" && orgIDArg == "") {
			return errors.New("unable to find the meta.json. " +
				"If you want to upload a version without a meta.json, you must supply a module name and namespace (or module name and org-id)",
			)
		}
	} else {
		// if we can find a manifest, use that
		manifest, err := loadManifest(manifestPath)
		var IDFromField string
		if err != nil {
			return err
		}

		if manifest.ModuleID != "" {
			IDFromField = manifest.ModuleID
		} else {
			IDFromField = manifest.Name
		}

		moduleID, err = parseModuleID(IDFromField)
		if err != nil {
			return err
		}
		if nameArg != "" && (nameArg != moduleID.name) {
			// This is almost certainly a mistake we want to catch
			return errors.Errorf("module name %q was supplied on the command line but the meta.json has a module ID of %q", nameArg,
				moduleID.name)
		}
		// set name arg from the manifest file rather than what is passed in
		nameArg = IDFromField
	}

	moduleID, err = validateModuleID(c, client, nameArg, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}
	tarballPath := moduleUploadPath
	if !isTarball(tarballPath) {
		tarballPath, err = createTarballForUpload(moduleUploadPath, c.App.Writer)
		if err != nil {
			return err
		}
		defer utils.RemoveFileNoError(tarballPath)
	}

	if !forceUploadArg {
		if err := validateModuleFile(client, moduleID, tarballPath, versionArg); err != nil {
			return fmt.Errorf(
				"error validating module: %w. For more details, please visit: https://docs.viam.com/manage/cli/#command-options-3 ",
				err)
		}
	}

	response, err := client.uploadModuleFile(moduleID, versionArg, platformArg, tarballPath)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "Version successfully uploaded! you can view your changes online here: %s", response.GetUrl())

	return nil
}

func (c *viamClient) createModule(moduleName, organizationID string) (*apppb.CreateModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := apppb.CreateModuleRequest{
		Name:           moduleName,
		OrganizationId: organizationID,
	}
	return c.client.CreateModule(c.c.Context, &req)
}

func (c *viamClient) getModule(moduleID moduleID) (*apppb.GetModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := apppb.GetModuleRequest{
		ModuleId: moduleID.String(),
	}
	return c.client.GetModule(c.c.Context, &req)
}

func (c *viamClient) updateModule(moduleID moduleID, manifest moduleManifest) (*apppb.UpdateModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	var models []*apppb.Model
	for _, moduleComponent := range manifest.Models {
		models = append(models, moduleComponentToProto(moduleComponent))
	}
	visibility, err := visibilityToProto(manifest.Visibility)
	if err != nil {
		return nil, err
	}
	req := apppb.UpdateModuleRequest{
		ModuleId:    moduleID.String(),
		Visibility:  visibility,
		Url:         manifest.URL,
		Description: manifest.Description,
		Models:      models,
		Entrypoint:  manifest.Entrypoint,
	}
	return c.client.UpdateModule(c.c.Context, &req)
}

func (c *viamClient) uploadModuleFile(
	moduleID moduleID,
	version,
	platform string,
	tarballPath string,
) (*apppb.UploadModuleFileResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return nil, err
	}
	ctx := c.c.Context

	stream, err := c.client.UploadModuleFile(ctx)
	if err != nil {
		return nil, err
	}
	moduleFileInfo := apppb.ModuleFileInfo{
		ModuleId: moduleID.String(),
		Version:  version,
		Platform: platform,
	}
	req := &apppb.UploadModuleFileRequest{
		ModuleFile: &apppb.UploadModuleFileRequest_ModuleFileInfo{ModuleFileInfo: &moduleFileInfo},
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}

	var errs error
	// We do not add the EOF as an error because all server-side errors trigger an EOF on the stream
	// This results in extra clutter to the error msg
	if err := sendModuleUploadRequests(ctx, stream, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	errs = multierr.Combine(errs, closeErr)
	return resp, errs
}

func sendModuleUploadRequests(ctx context.Context, stream apppb.AppService_UploadModuleFileClient, file *os.File, stdout io.Writer) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()
	uploadedBytes := 0
	// Close the line with the progress reading
	defer printf(stdout, "")

	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file or the context expires.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextModuleUploadRequest(file)

		// EOF means we've completed successfully.
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "could not read file")
		}

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
		uploadedBytes += len(uploadReq.GetFile())
		// Simple progress reading until we have a proper tui library
		uploadPercent := int(math.Ceil(100 * float64(uploadedBytes) / float64(fileSize)))
		fmt.Fprintf(stdout, "\rUploading... %d%% (%d/%d bytes)", uploadPercent, uploadedBytes, fileSize) // no newline
	}
}

func getNextModuleUploadRequest(file *os.File) (*apppb.UploadModuleFileRequest, error) {
	// get the next chunk of bytes from the file
	byteArr := make([]byte, moduleUploadChunkSize)
	numBytesRead, err := file.Read(byteArr)
	if err != nil {
		return nil, err
	}
	if numBytesRead < moduleUploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	return &apppb.UploadModuleFileRequest{
		ModuleFile: &apppb.UploadModuleFileRequest_File{
			File: byteArr,
		},
	}, nil
}

func validateModuleFile(client *viamClient, moduleID moduleID, tarballPath, version string) error {
	getModuleResp, err := client.getModule(moduleID)
	if err != nil {
		return err
	}
	entrypoint, err := getEntrypointForVersion(getModuleResp.Module, version)
	if err != nil {
		return err
	}
	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	archive, err := gzip.NewReader(file)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(archive)
	filesWithSameNameAsEntrypoint := []string{}
	for {
		if err := client.c.Context.Err(); err != nil {
			return err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "error reading %s", file.Name())
		}
		path := header.Name

		// if path == entrypoint, we have found the right file
		if filepath.Clean(path) == filepath.Clean(entrypoint) {
			info := header.FileInfo()
			if info.Mode().Perm()&0o100 == 0 {
				return errors.Errorf(
					"the archive contained a file at the entrypoint %q, but that file is not marked as executable",
					entrypoint)
			}
			// executable file at entrypoint. validation succeeded.
			return nil
		}
		if filepath.Base(path) == filepath.Base(entrypoint) {
			filesWithSameNameAsEntrypoint = append(filesWithSameNameAsEntrypoint, path)
		}
	}
	extraErrInfo := ""
	if len(filesWithSameNameAsEntrypoint) > 0 {
		extraErrInfo = fmt.Sprintf(". Did you mean to set your entrypoint to %v?", filesWithSameNameAsEntrypoint)
	}
	return errors.Errorf("the archive does not contain a file at the desired entrypoint %q%s",
		entrypoint, extraErrInfo)
}

func visibilityToProto(visibility moduleVisibility) (apppb.Visibility, error) {
	switch visibility {
	case moduleVisibilityPrivate:
		return apppb.Visibility_VISIBILITY_PRIVATE, nil
	case moduleVisibilityPublic:
		return apppb.Visibility_VISIBILITY_PUBLIC, nil
	default:
		return apppb.Visibility_VISIBILITY_UNSPECIFIED,
			errors.Errorf("invalid module visibility. must be either %q or %q", moduleVisibilityPublic, moduleVisibilityPrivate)
	}
}

func moduleComponentToProto(moduleComponent ModuleComponent) *apppb.Model {
	return &apppb.Model{
		Api:   moduleComponent.API,
		Model: moduleComponent.Model,
	}
}

func parseModuleID(id string) (moduleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
	// We also allow for empty prefixes here (unlike the backend) to simplify the flexible way to parse user input
	splitModuleName := strings.Split(id, ":")
	switch len(splitModuleName) {
	case 1:
		return moduleID{prefix: "", name: id}, nil
	case 2:
		return moduleID{prefix: splitModuleName[0], name: splitModuleName[1]}, nil
	default:
		return moduleID{}, errors.Errorf("invalid module name '%s'."+
			" Module name must be in the form 'prefix:module-name' for public modules"+
			" or just 'module-name' for private modules in organizations without a public namespace", id)
	}
}

func (m *moduleID) String() string {
	if m.prefix == "" {
		return m.name
	}
	return fmt.Sprintf("%s:%s", m.prefix, m.name)
}

// validateModuleID tries to parse the manifestModuleID to see if it is a valid moduleID with a prefix
// if it is not, it uses the publicNamespaceArg and orgIDArg to determine what the moduleID prefix should be.
func validateModuleID(
	c *cli.Context,
	client *viamClient,
	manifestModuleID,
	publicNamespaceArg,
	orgIDArg string,
) (moduleID, error) {
	mid, err := parseModuleID(manifestModuleID)
	if err != nil {
		return moduleID{}, err
	}

	if mid.prefix != "" {
		if publicNamespaceArg != "" || orgIDArg != "" {
			org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
			if err != nil {
				return moduleID{}, err
			}
			expectedOrg, err := getOrgByModuleIDPrefix(client, mid.prefix)
			if err != nil {
				return moduleID{}, err
			}
			if org.GetId() != expectedOrg.GetId() {
				// This is almost certainly a user mistake
				// Preferring org name rather than orgid here because the manifest probably has it specified in terms of
				// public_namespace so returning the ids would be frustrating
				return moduleID{}, errors.Errorf("the meta.json specifies a different org %q than the one provided via args %q",
					expectedOrg.GetName(), org.GetName())
			}
			printf(c.App.Writer, "the module's meta.json already specifies a full module id. Ignoring public-namespace and org-id arg")
		}
		return mid, nil
	}
	// moduleID.Prefix is empty. Need to use orgIDArg and publicNamespaceArg to figure out what it should be
	org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	if err != nil {
		return moduleID{}, err
	}
	if org.PublicNamespace != "" {
		mid.prefix = org.PublicNamespace
	} else {
		mid.prefix = org.Id
	}
	return mid, nil
}

// resolveOrg accepts either an orgID or a publicNamespace (one must be an empty string).
// If orgID is an empty string, it will use the publicNamespace to resolve it.
func resolveOrg(client *viamClient, publicNamespace, orgID string) (*apppb.Organization, error) {
	if orgID != "" {
		if publicNamespace != "" {
			return nil, errors.New("cannot specify both org-id and public-namespace")
		}
		if !isValidOrgID(orgID) {
			return nil, errors.Errorf("provided org-id %q is not a valid org-id", orgID)
		}
		org, err := client.getOrg(orgID)
		if err != nil {
			return nil, err
		}
		return org, nil
	}
	// Use publicNamespace to back-derive what the org is
	if publicNamespace == "" {
		return nil, errors.New("must provide either org-id or public-namespace")
	}
	org, err := client.getUserOrgByPublicNamespace(publicNamespace)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func getOrgByModuleIDPrefix(client *viamClient, moduleIDPrefix string) (*apppb.Organization, error) {
	if isValidOrgID(moduleIDPrefix) {
		return client.getOrg(moduleIDPrefix)
	}
	return client.getUserOrgByPublicNamespace(moduleIDPrefix)
}

// isValidOrgID checks if the str is a valid uuid.
func isValidOrgID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

func loadManifest(manifestPath string) (moduleManifest, error) {
	//nolint:gosec
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return moduleManifest{}, errors.Wrapf(err, "cannot find %s", manifestPath)
		}
		return moduleManifest{}, err
	}
	var manifest moduleManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return moduleManifest{}, err
	}
	return manifest, nil
}

func writeManifest(manifestPath string, manifest moduleManifest) error {
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", manifestPath)
	}
	if _, err := manifestFile.Write(manifestBytes); err != nil {
		return errors.Wrapf(err, "failed to write manifest to %s", manifestPath)
	}

	return nil
}

// getEntrypointForVersion returns the entrypoint associated with the provided version, or the last updated entrypoint if it doesnt exit.
func getEntrypointForVersion(mod *apppb.Module, version string) (string, error) {
	for _, ver := range mod.Versions {
		if ver.Version == version {
			return ver.Entrypoint, nil
		}
	}
	if mod.Entrypoint == "" {
		return "", errors.New("no entrypoint has been set for your module. add one to your meta.json and then update your module")
	}
	// if there is no entrypoint set yet, use the last uploaded entrypoint
	return mod.Entrypoint, nil
}

func isTarball(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".tar.gz") ||
		strings.HasSuffix(strings.ToLower(path), ".tgz")
}

func createTarballForUpload(moduleUploadPath string, stdout io.Writer) (string, error) {
	tmpFile, err := os.CreateTemp("", "module-upload-*.tar.gz")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary archive file")
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			Errorf(stdout, "failed to close temporary archive file %q", tmpFile.Name())
		}
	}()

	tmpFileWriter := bufio.NewWriter(tmpFile)
	archiveFiles, err := getArchiveFilePaths([]string{moduleUploadPath})
	if err != nil {
		return "", errors.Wrapf(err, "failed to find files to compress in %q", moduleUploadPath)
	}
	if len(archiveFiles) == 0 {
		return "", errors.Errorf("failed to find any files in %q", moduleUploadPath)
	}
	if err := createArchive(archiveFiles, tmpFileWriter, stdout); err != nil {
		return "", errors.Wrap(err, "failed to create temp archive")
	}
	if err := tmpFileWriter.Flush(); err != nil {
		return "", errors.Wrap(err, "failed to flush buffer while creating temp archive")
	}
	return tmpFile.Name(), nil
}

func readModels(path string, logger logging.Logger) ([]ModuleComponent, error) {
	parentAddr, err := os.MkdirTemp("", "viam-cli-test-*")
	if err != nil {
		return nil, err
	}
	defer vutils.UncheckedErrorFunc(func() error { return os.RemoveAll(parentAddr) })
	parentAddr += "/parent.sock"

	cfg := modconfig.Module{
		Name:    "xxxx",
		ExePath: path,
	}

	mgr := modmanager.NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	defer vutils.UncheckedErrorFunc(func() error { return mgr.Close(context.Background()) })

	err = mgr.Add(context.TODO(), cfg)
	if err != nil {
		return nil, err
	}

	res := []ModuleComponent{}

	h := mgr.Handles()
	for k, v := range h[cfg.Name] {
		for _, m := range v {
			res = append(res, ModuleComponent{k.API.String(), m.String()})
		}
	}

	return res, nil
}

func sameModels(a, b []ModuleComponent) bool {
	if len(a) != len(b) {
		return false
	}

	for _, x := range a {
		found := false

		for _, y := range b {
			if x.API == y.API && x.Model == y.Model {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// UpdateModelsAction figures out the models that a module supports and updates it's metadata file.
func UpdateModelsAction(c *cli.Context) error {
	logger := logging.NewLogger("x")
	newModels, err := readModels(c.String("binary"), logger)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(c.String(moduleFlagPath))
	if err != nil {
		return err
	}

	if sameModels(newModels, manifest.Models) {
		return nil
	}

	manifest.Models = newModels
	return writeManifest(c.String(moduleFlagPath), manifest)
}
