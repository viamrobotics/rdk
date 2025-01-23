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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagespb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	vutils "go.viam.com/utils"

	modconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/module/modmanager"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// moduleUploadChunkSize sets the number of bytes included in each chunk of the upload stream.
var (
	moduleUploadChunkSize = 32 * 1024
	rdkAPITypes           = []string{resource.APITypeServiceName, resource.APITypeComponentName}
)

// moduleVisibility determines whether modules are public or private.
type moduleVisibility string

// Permissions enumeration.
const (
	moduleVisibilityPrivate        moduleVisibility = "private"
	moduleVisibilityPublic         moduleVisibility = "public"
	moduleVisibilityPublicUnlisted moduleVisibility = "public_unlisted"
)

type unknownRdkAPITypeError struct {
	APIType string
}

func (err unknownRdkAPITypeError) Error() string {
	return fmt.Sprintf("API with unknown type '%s', expected one of %s", err.APIType, strings.Join(rdkAPITypes, ", "))
}

// ModuleComponent represents an api - model pair.
type ModuleComponent struct {
	API          string  `json:"api"`
	Model        string  `json:"model"`
	Description  *string `json:"short_description,omitempty"`
	MarkdownLink *string `json:"markdown_link,omitempty"`
}

// moduleID represents a prefix:name pair where prefix can be either an org id or a namespace.
type moduleID struct {
	prefix string
	name   string
}

// manifestBuildInfo is the "build" section of meta.json.
type manifestBuildInfo struct {
	Build      string   `json:"build"`
	Setup      string   `json:"setup"`
	Path       string   `json:"path"`
	Arch       []string `json:"arch"`
	DarwinDeps []string `json:"darwin_deps,omitempty"`
}

// defaultBuildInfo has defaults for unset fields in "build".
var defaultBuildInfo = manifestBuildInfo{
	Build: "make module.tar.gz",
	Path:  "module.tar.gz",
	Arch:  []string{"linux/amd64", "linux/arm64"},
}

// moduleManifest is used to create & parse manifest.json.
// Detailed user-facing docs for this are in module.schema.json.
type moduleManifest struct {
	Schema      string            `json:"$schema"`
	ModuleID    string            `json:"module_id"`
	Visibility  moduleVisibility  `json:"visibility"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Models      []ModuleComponent `json:"models"`
	// JsonManifest provides fields shared with RDK proper.
	modconfig.JSONManifest
	Build *manifestBuildInfo `json:"build,omitempty"`
}

const (
	defaultManifestFilename = "meta.json"
)

type createModuleActionArgs struct {
	Name            string
	PublicNamespace string
	OrgID           string
	LocalOnly       bool
}

// CreateModuleAction is the corresponding Action for 'module create'. It runs
// the command to create a module. This includes both a gRPC call to register
// the module on app.viam.com and creating the manifest file.
func CreateModuleAction(c *cli.Context, args createModuleActionArgs) error {
	moduleNameArg := args.Name
	publicNamespaceArg := args.PublicNamespace
	orgIDArg := args.OrgID
	localOnly := args.LocalOnly

	var client *viamClient
	var err error
	var org *apppb.Organization
	if localOnly {
		org = &apppb.Organization{Id: orgIDArg, PublicNamespace: publicNamespaceArg}
	} else {
		client, err = newViamClient(c)
		if err != nil {
			return err
		}
		org, err = resolveOrg(client, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
	}

	shouldWriteNewEmptyManifest := true

	// If a meta.json exists in the current directory, we have a slightly different creation flow
	// in order to minimize user frustration. We will continue the creation if the args passed to create
	// match the values in the meta.json
	if _, err := os.Stat(defaultManifestFilename); err == nil {
		modManifest, err := loadManifest(defaultManifestFilename)
		if err != nil {
			return errors.New("another meta.json already exists in the current directory. Delete it and try again")
		}
		manifestModuleID, err := parseModuleID(modManifest.ModuleID)
		if err != nil ||
			manifestModuleID.name != moduleNameArg ||
			(manifestModuleID.prefix != org.GetId() && manifestModuleID.prefix != org.GetPublicNamespace()) {
			return errors.Errorf("a different module's meta.json already exists in the current directory. "+
				"Either delete that meta.json, or edit its module_id (%q) to match the args passed to this command",
				modManifest.ModuleID)
		}
		shouldWriteNewEmptyManifest = false
	}

	var returnedModuleID moduleID
	if localOnly {
		returnedModuleID.name = moduleNameArg
		if org.PublicNamespace != "" {
			returnedModuleID.prefix = org.PublicNamespace
		} else {
			returnedModuleID.prefix = org.Id
		}
	} else {
		response, err := client.createModule(moduleNameArg, org.GetId())
		if err != nil {
			return errors.Wrap(err, "failed to register the module on app.viam.com")
		}
		returnedModuleID, err = parseModuleID(response.GetModuleId())
		if err != nil {
			return err
		}
		printf(c.App.Writer, "Successfully created '%s'", returnedModuleID.String())
		if response.GetUrl() != "" {
			printf(c.App.Writer, "You can view it here: %s", response.GetUrl())
		}
	}

	if shouldWriteNewEmptyManifest {
		emptyManifest := moduleManifest{
			Schema:     "https://dl.viam.dev/module.schema.json",
			ModuleID:   returnedModuleID.String(),
			Visibility: moduleVisibilityPrivate,
			// This is done so that the json has an empty example
			Models: []ModuleComponent{
				{},
			},
		}
		if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
			return err
		}

		printf(c.App.Writer, "Configuration for the module has been written to meta.json")
	}
	return nil
}

type updateModuleArgs struct {
	Module string
}

// UpdateModuleAction is the corresponding Action for 'module update'. It runs
// the command to update a module. This includes updating the meta.json to
// include the public namespace (if set on the org).
func UpdateModuleAction(c *cli.Context, args updateModuleArgs) error {
	manifestPath := args.Module

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}

	moduleID, err := parseModuleID(manifest.ModuleID)
	if err != nil {
		return err
	}

	validateModels(c.App.ErrWriter, &manifest)

	response, err := client.updateModule(moduleID, manifest)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Module successfully updated! You can view your changes online here: %s", response.GetUrl())

	// if the module id prefix is an org id, check to see if a public namespace has been set and update the manifest if it has
	if isValidOrgID(moduleID.prefix) {
		org, err := client.getOrg(moduleID.prefix)
		if err != nil {
			return errors.Wrap(err, "failed to update meta.json with new information from Viam")
		}
		if org.PublicNamespace != "" {
			moduleID.prefix = org.PublicNamespace
			manifest.ModuleID = moduleID.String()
			if err := writeManifest(manifestPath, manifest); err != nil {
				return errors.Wrap(err, "failed to update meta.json with new information from Viam")
			}
		}
	}
	return nil
}

type uploadModuleArgs struct {
	Module          string
	PublicNamespace string
	OrgID           string
	Name            string
	Version         string
	Platform        string
	Tags            []string
	Force           bool
	Upload          string
}

// UploadModuleAction is the corresponding action for 'module upload'.
func UploadModuleAction(c *cli.Context, args uploadModuleArgs) error {
	manifestPath := args.Module
	publicNamespaceArg := args.PublicNamespace
	orgIDArg := args.OrgID
	nameArg := args.Name
	versionArg := args.Version
	platformArg := args.Platform
	forceUploadArg := args.Force
	constraints := args.Tags
	moduleUploadPath := args.Upload
	if moduleUploadPath == "" {
		moduleUploadPath = c.Args().First()
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
	// if the manifest cant be found, use passed in arguments to determine the module id
	if _, err := os.Stat(manifestPath); err != nil {
		if nameArg == "" || (publicNamespaceArg == "" && orgIDArg == "") {
			return errors.New("unable to find the meta.json. " +
				"If you want to upload a version without a meta.json, you must supply a module name and namespace (or module name and org-id)",
			)
		}
		moduleID.name = nameArg
		if publicNamespaceArg != "" {
			moduleID.prefix = publicNamespaceArg
		} else {
			moduleID.prefix = orgIDArg
		}
	} else {
		// if we can find a manifest, use that
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return err
		}
		moduleID, err = parseModuleID(manifest.ModuleID)
		if err != nil {
			return err
		}
		if nameArg != "" && (nameArg != moduleID.name) {
			// This is almost certainly a mistake we want to catch
			return errors.Errorf("module name %q was supplied on the command line but the meta.json has a module ID of %q", nameArg,
				moduleID.name)
		}

		moduleID, err = validateModuleID(client, moduleID.String(), publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}

		validateModels(c.App.ErrWriter, &manifest)

		_, err = client.updateModule(moduleID, manifest)
		if err != nil {
			return errors.Wrap(err, "Module update failed. Please correct the following issues in your meta.json")
		}
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
		if err := validateModuleFile(client, c, moduleID, tarballPath, versionArg, platformArg); err != nil {
			return fmt.Errorf(
				"error validating module: %w. For more details, please visit: https://docs.viam.com/cli/#module ",
				err)
		}
	}

	response, err := client.uploadModuleFile(moduleID, versionArg, platformArg, constraints, tarballPath)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "Version successfully uploaded! you can view your changes online here: %s", response.GetUrl())

	return nil
}

// call validateModelAPI on all models in manifest and warn if violations.
func validateModels(errWriter io.Writer, manifest *moduleManifest) {
	for _, model := range manifest.Models {
		if err := validateModelAPI(model.API); err != nil {
			warningf(errWriter, "error validating API string %s: %s", model.API, err)
		}
	}
}

// return a useful error if the model string looks wrong.
func validateModelAPI(modelAPI string) error {
	api, err := resource.ParseAPIString(modelAPI)
	if err != nil {
		return errors.Wrap(err, "unparseable model string")
	}
	if err := api.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate API")
	}
	if !slices.Contains(rdkAPITypes, api.Type.Name) {
		return unknownRdkAPITypeError{APIType: api.Type.Name}
	}
	return nil
}

type updateModelsArgs struct {
	Module string
	Binary string
}

// UpdateModelsAction figures out the models that a module supports and updates it's metadata file.
func UpdateModelsAction(c *cli.Context, args updateModelsArgs) error {
	logger := logging.NewLogger("x")
	newModels, err := readModels(args.Binary, logger)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(args.Module)
	if err != nil {
		return err
	}

	if sameModels(newModels, manifest.Models) {
		return nil
	}

	manifest.Models = newModels
	return writeManifest(args.Module, manifest)
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
	if manifest.FirstRun != "" {
		req.FirstRun = &manifest.FirstRun
	}
	return c.client.UpdateModule(c.c.Context, &req)
}

func (c *viamClient) uploadModuleFile(
	moduleID moduleID,
	version,
	platform string,
	constraints []string,
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
		ModuleId:     moduleID.String(),
		Version:      version,
		Platform:     platform,
		PlatformTags: constraints,
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
	if err := sendUploadRequests(ctx, stream, nil, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	errs = multierr.Combine(errs, closeErr)
	return resp, errs
}

func validateModuleFile(client *viamClient, c *cli.Context, moduleID moduleID, tarballPath, version, platform string) error {
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
	// stores all names of alternative entrypoints if the user has a path error
	filesWithSameNameAsEntrypoint := []string{}
	// stores all symlinks that leave the module root
	badSymlinks := map[string]string{}
	foundEntrypoint := false
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
		if header.Typeflag == tar.TypeLink || header.Typeflag == tar.TypeSymlink {
			base := filepath.Base(tarballPath)
			if filepath.IsAbs(header.Linkname) ||
				//nolint:gosec
				!strings.HasPrefix(filepath.Join(base, header.Linkname), base) {
				badSymlinks[header.Name] = header.Linkname
			}
		}
		path := header.Name

		// if path == entrypoint, we have found the right file
		if filepath.Clean(path) == filepath.Clean(entrypoint) {
			info := header.FileInfo()
			if info.IsDir() {
				return errors.Errorf(
					"the module archive contains a directory at the entrypoint %q instead of an executable file",
					entrypoint)
			}

			if info.Mode().Perm()&0o100 == 0 {
				return errors.Errorf(
					"the module archive contains a file at the entrypoint %q, but that file is not marked as executable",
					entrypoint)
			}
			// executable file at entrypoint. validation succeeded.
			// continue looping to find symlinks
			foundEntrypoint = true
			if parsed := getExecutableArch(tarReader); parsed != "" && parsed != platform {
				warningf(c.App.ErrWriter,
					"You've tagged %s but your binary has platform %s. (This warning is experimental, ignore if it doesn't make sense).",
					platform, parsed)
			}
		}
		if filepath.Base(path) == filepath.Base(entrypoint) {
			filesWithSameNameAsEntrypoint = append(filesWithSameNameAsEntrypoint, path)
		}
	}
	if len(badSymlinks) > 0 {
		warningf(client.c.App.ErrWriter, "Module contains symlinks to files outside the package."+
			" This might cause issues on other smart machines:")
		numPrinted := 0
		for name := range badSymlinks {
			printf(client.c.App.ErrWriter, "\t%s -> %s", name, badSymlinks[name])
			// only print at most 10 links (virtual environments can have thousands of links)
			if numPrinted++; numPrinted == 10 {
				printf(client.c.App.ErrWriter, "\t...")
				break
			}
		}
	}
	if !foundEntrypoint {
		extraErrInfo := ""
		if len(filesWithSameNameAsEntrypoint) > 0 {
			extraErrInfo = fmt.Sprintf(". Did you mean to set your entrypoint to %v?", filesWithSameNameAsEntrypoint)
		}
		return errors.Errorf("the archive does not contain a file at the desired entrypoint %q%s",
			entrypoint, extraErrInfo)
	}
	// success
	return nil
}

// runs ParseFileType on the output of running `file` on whatever is in the tarball. Returns empty string if anything went wrong.
func getExecutableArch(reader *tar.Reader) string {
	if _, err := exec.LookPath("file"); err != nil {
		return ""
	}
	cmd := exec.Command("file", "-")
	cmd.Stdin = reader
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return ParseFileType(string(output))
}

func visibilityToProto(visibility moduleVisibility) (apppb.Visibility, error) {
	switch visibility {
	case moduleVisibilityPrivate:
		return apppb.Visibility_VISIBILITY_PRIVATE, nil
	case moduleVisibilityPublic:
		return apppb.Visibility_VISIBILITY_PUBLIC, nil
	case moduleVisibilityPublicUnlisted:
		return apppb.Visibility_VISIBILITY_PUBLIC_UNLISTED, nil
	default:
		return apppb.Visibility_VISIBILITY_UNSPECIFIED,
			errors.Errorf("invalid module visibility. must be either %q, %q, or %q",
				moduleVisibilityPublic, moduleVisibilityPrivate, moduleVisibilityPublicUnlisted)
	}
}

func moduleComponentToProto(moduleComponent ModuleComponent) *apppb.Model {
	model := &apppb.Model{
		Api:         moduleComponent.API,
		Model:       moduleComponent.Model,
		Description: moduleComponent.Description,
	}

	// If a markdown link is provided, read the content
	if moduleComponent.MarkdownLink != nil {
		if content, err := getMarkdownContent(*moduleComponent.MarkdownLink); err == nil {
			model.MarkdownDocumentation = &content
		} else {
			warningf(os.Stderr, "Failed to read markdown content from %s: %v", *moduleComponent.MarkdownLink, err)
		}
	}

	return model
}

func parseModuleID(id string) (moduleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
	splitModuleName := strings.Split(id, ":")
	if len(splitModuleName) != 2 {
		return moduleID{}, errors.Errorf("invalid module name '%s'."+
			" Module name must be in the form 'public-namespace:module-name' for public modules"+
			" or 'org-id:module-name' for private modules in organizations without a public namespace", id)
	}
	return moduleID{prefix: splitModuleName[0], name: splitModuleName[1]}, nil
}

func (m *moduleID) String() string {
	return fmt.Sprintf("%s:%s", m.prefix, m.name)
}

// validateModuleID tries to parse the manifestModuleID and checks that it matches the publicNamespaceArg and orgIDArg if they are provided.
func validateModuleID(
	client *viamClient,
	manifestModuleID,
	publicNamespaceArg,
	orgIDArg string,
) (moduleID, error) {
	modID, err := parseModuleID(manifestModuleID)
	if err != nil {
		return moduleID{}, err
	}

	// if either publicNamespaceArg or orgIDArg are set, check that they match the passed moduleID
	if publicNamespaceArg != "" || orgIDArg != "" {
		org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
		if err != nil {
			return moduleID{}, err
		}
		expectedOrg, err := getOrgByModuleIDPrefix(client, modID.prefix)
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
	}
	return modID, nil
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

// loadManifestOrNil doesn't throw error on missing.
func loadManifestOrNil(path string) (*moduleManifest, error) {
	manifest, err := loadManifest(path)
	if err == nil {
		return &manifest, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		//nolint:nilnil
		return nil, nil
	}
	return nil, err
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
	tmpdir, err := os.MkdirTemp("", "viam-cli-test-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			logger.Warnw("failed to delete temp directory", "path", tmpdir, "error", err)
		}
	}()
	parentAddr := tmpdir + "/parent.sock"

	// allows a module to start without connecting to a parent
	if err := os.Setenv(module.NoModuleParentEnvVar, "true"); err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer os.Unsetenv(module.NoModuleParentEnvVar)

	cfg := modconfig.Module{
		Name:    "xxxx",
		ExePath: path,
	}

	mgr := modmanager.NewManager(context.Background(), parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	defer vutils.UncheckedErrorFunc(func() error { return mgr.Close(context.Background()) })

	err = mgr.Add(context.TODO(), cfg)
	if err != nil {
		return nil, err
	}

	res := []ModuleComponent{}

	h := mgr.Handles()
	for k, v := range h[cfg.Name] {
		for _, m := range v {
			res = append(res, ModuleComponent{k.API.String(), m.String(), nil, nil})
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

func sendUploadRequests(ctx context.Context, moduleStream apppb.AppService_UploadModuleFileClient,
	pkgStream packagespb.PackageService_CreatePackageClient, file *os.File, stdout io.Writer,
) error {
	if moduleStream != nil && pkgStream != nil {
		return errors.New("can use either module or package client, not both")
	}
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()
	uploadedBytes := 0
	// Close the line with the progress reading
	defer printf(stdout, "")

	if moduleStream != nil {
		defer vutils.UncheckedErrorFunc(moduleStream.CloseSend)
	}
	if pkgStream != nil {
		defer vutils.UncheckedErrorFunc(pkgStream.CloseSend)
	}
	// Loop until there is no more content to be read from file or the context expires.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Get the next UploadRequest from the file.
		var moduleUploadReq *apppb.UploadModuleFileRequest
		if moduleStream != nil {
			moduleUploadReq, err = getNextModuleUploadRequest(file)
		}
		var pkgUploadReq *packagespb.CreatePackageRequest
		if pkgStream != nil {
			pkgUploadReq, err = getNextPackageUploadRequest(file)
		}

		// EOF means we've completed successfully.
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "could not read file")
		}

		if moduleUploadReq != nil {
			if err = moduleStream.Send(moduleUploadReq); err != nil {
				return err
			}
			uploadedBytes += len(moduleUploadReq.GetFile())
		}
		if pkgUploadReq != nil {
			if err = pkgStream.Send(pkgUploadReq); err != nil {
				return err
			}
			uploadedBytes += len(pkgUploadReq.GetContents())
		}

		// Simple progress reading until we have a proper tui library
		uploadPercent := int(math.Ceil(100 * float64(uploadedBytes) / float64(fileSize)))
		fmt.Fprintf(stdout, "\rUploading... %d%% (%d/%d bytes)", uploadPercent, uploadedBytes, fileSize) //nolint:errcheck // no newline
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

type downloadModuleFlags struct {
	Destination string
	ID          string
	Version     string
	Platform    string
}

// DownloadModuleAction downloads a module.
func DownloadModuleAction(c *cli.Context, flags downloadModuleFlags) error {
	moduleID := flags.ID
	if moduleID == "" {
		manifest, err := loadManifest(defaultManifestFilename)
		if err != nil {
			return errors.Wrap(err, "trying to get package ID from meta.json")
		}
		moduleID = manifest.ModuleID
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.ensureLoggedIn(); err != nil {
		return err
	}
	req := &apppb.GetModuleRequest{ModuleId: moduleID}
	res, err := client.client.GetModule(c.Context, req)
	if err != nil {
		return err
	}
	if len(res.Module.Versions) == 0 {
		return errors.New("module has 0 uploaded versions, nothing to download")
	}
	requestedVersion := flags.Version
	var ver *apppb.VersionHistory
	if requestedVersion == "latest" {
		ver = res.Module.Versions[len(res.Module.Versions)-1]
	} else {
		for _, iVer := range res.Module.Versions {
			if iVer.Version == requestedVersion {
				ver = iVer
				break
			}
		}
		if ver == nil {
			return fmt.Errorf("version %s not found in versions for module", requestedVersion)
		}
	}
	infof(c.App.ErrWriter, "found version %s", ver.Version)
	if len(ver.Files) == 0 {
		return fmt.Errorf("version %s has 0 files uploaded", ver.Version)
	}
	platform := flags.Platform
	if platform == "" {
		platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		infof(c.App.ErrWriter, "using default platform %s", platform)
	}
	if !slices.ContainsFunc(ver.Files, func(file *apppb.Uploads) bool { return file.Platform == platform }) {
		return fmt.Errorf("platform %s not present for version %s", platform, ver.Version)
	}
	include := true
	packageType := packagespb.PackageType_PACKAGE_TYPE_MODULE
	// note: this is working around a GetPackage quirk where platform messes with version
	fullVersion := fmt.Sprintf("%s-%s", ver.Version, strings.ReplaceAll(platform, "/", "-"))
	pkg, err := client.packageClient.GetPackage(c.Context, &packagespb.GetPackageRequest{
		Id:         strings.ReplaceAll(moduleID, ":", "/"),
		Version:    fullVersion,
		IncludeUrl: &include,
		Type:       &packageType,
	})
	if err != nil {
		return err
	}
	destName := strings.ReplaceAll(moduleID, ":", "-")
	infof(c.App.ErrWriter, "saving to %s", path.Join(flags.Destination, fullVersion, destName+".tar.gz"))
	return downloadPackageFromURL(c.Context, client.authFlow.httpClient,
		flags.Destination, destName,
		fullVersion, pkg.Package.Url, client.conf.Auth,
	)
}

// getMarkdownContent reads and returns the content from a markdown file path.
// The path may include an anchor tag (e.g., "docs/api.md#section-name").
// The anchor tag is used to get the content of the section with the given anchor.
// In the case of a nested section, the anchor tag is used to get the content of the section with the given anchor.
// And the end of the section is determined by the next heading of the same or higher level.
func getMarkdownContent(markdownPath string) (string, error) {
	parts := strings.Split(markdownPath, "#")
	filePath := parts[0]
	var anchor string
	if len(parts) > 1 {
		anchor = parts[1]
	}

	//nolint:gosec
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read markdown file at %s", filePath)
	}

	if anchor == "" {
		return string(content), nil
	}

	lines := strings.Split(string(content), "\n")
	sectionStart := -1
	startHeaderLevel := 0

	// Look for heading matching anchor and get its level
	possibleAnchors := []string{}
	anchorCounts := make(map[string]int)
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			// Count actual # characters at start of line
			headerLevel := 0
			for _, char := range line {
				if char == '#' {
					headerLevel++
				} else {
					break
				}
			}

			headingAnchor := generateAnchor(line)

			// Check if this anchor already exists
			// If it does, add a numerical suffix to the anchor to make it unique
			// This makes the anchor unique and avoids conflicts with other anchors in the same file
			if count, exists := anchorCounts[headingAnchor]; exists {
				anchorCounts[headingAnchor]++
				headingAnchor = fmt.Sprintf("%s-%d", headingAnchor, count)
			} else {
				anchorCounts[headingAnchor] = 1
			}

			possibleAnchors = append(possibleAnchors, headingAnchor)
			if headingAnchor == anchor {
				// Skip the header line to not include the header in the model docs, only the body of the section.
				sectionStart = i + 1
				startHeaderLevel = headerLevel
				break
			}
		}
	}

	// If the section matching the anchor is not found, return an error
	if sectionStart == -1 {
		return "", errors.Errorf(
			"section #%s not found in %s. Check the format of your markdown_link and ensure the anchor is correct. "+
				"You can anchor to the following headings: %v", anchor, filePath, possibleAnchors,
		)
	}

	// Find end of section (next heading of same or higher level)
	sectionEnd := len(lines)
	for i := sectionStart; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") {
			// Count actual # characters at start of line
			headerLevel := 0
			for _, char := range lines[i] {
				if char == '#' {
					headerLevel++
				} else {
					break
				}
			}

			if headerLevel <= startHeaderLevel {
				sectionEnd = i
				break
			}
		}
	}

	return strings.Join(lines[sectionStart:sectionEnd], "\n"), nil
}

func generateAnchor(header string) string {
	// Convert to lowercase
	anchor := strings.ToLower(header)

	// Remove special characters except spaces and hyphens
	re := regexp.MustCompile(`[^\w\s-]`)
	anchor = re.ReplaceAllString(anchor, "")

	// Replace spaces with hyphens
	anchor = strings.ReplaceAll(anchor, " ", "-")

	// Remove leading and trailing hyphens
	anchor = strings.Trim(anchor, "-")

	// Collapse consecutive hyphens into a single hyphen
	reHyphen := regexp.MustCompile(`-+`)
	anchor = reHyphen.ReplaceAllString(anchor, "-")

	return anchor
}
