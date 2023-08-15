package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	apppb "go.viam.com/api/app/v1"
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

// moduleComponent represents an api - model pair.
type moduleComponent struct {
	API   string `json:"api"`
	Model string `json:"model"`
}

// moduleID represents a prefix:name pair where prefix can be either an org id or a namespace.
type moduleID struct {
	prefix string
	name   string
}

// moduleManifest is used to create & parse manifest.json.
type moduleManifest struct {
	Name        string            `json:"name"`
	Visibility  moduleVisibility  `json:"visibility"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Models      []moduleComponent `json:"models"`
	Entrypoint  string            `json:"entrypoint"`
}

const (
	defaultManifestFilename = "meta.json"
)

// CreateModuleAction is the corresponding Action for 'module create'. It runs
// the command to create a module. This includes both a gRPC call to register
// the module on app.viam.com and creating the manifest file.
func CreateModuleAction(c *cli.Context) error {
	moduleNameArg := c.String("name")
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")

	client, err := newAppClient(c)
	if err != nil {
		return err
	}
	org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}
	if org == nil {
		return errors.Errorf("unable to determine org from org-id (%q) and namespace (%q)", orgIDArg, publicNamespaceArg)
	}
	// Check to make sure the user doesn't accidentally overwrite a module manifest
	if _, err := os.Stat(defaultManifestFilename); err == nil {
		return errors.New("another module's meta.json already exists in the current directory. delete it and try again")
	}

	response, err := client.createModule(moduleNameArg, org.GetId())
	if err != nil {
		return errors.Wrap(err, "failed to register the module on app.viam.com")
	}

	returnedModuleID, err := parseModuleID(response.GetModuleId())
	if err != nil {
		return err
	}
	// The registry team is currently of the opinion that including an org id in the meta.json file
	// is non-ideal.
	// If you do change this, edit the UpdateCommand().. function to also check if the manifest prefix is an orgid
	// during the replacement to a public namespace
	if isValidOrgID(returnedModuleID.prefix) {
		returnedModuleID.prefix = ""
	}
	fmt.Fprintf(c.App.Writer, "successfully created '%s'.\n", returnedModuleID.String())
	if response.GetUrl() != "" {
		fmt.Fprintf(c.App.Writer, "you can view it here: %s \n", response.GetUrl())
	}
	emptyManifest := moduleManifest{
		Name:       returnedModuleID.String(),
		Visibility: moduleVisibilityPrivate,
		// This is done so that the json has an empty example
		Models: []moduleComponent{
			{},
		},
	}
	if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "configuration for the module has been written to meta.json\n")
	return nil
}

// UpdateModuleAction is the corresponding Action for 'module update'. It runs
// the command to update a module. This includes updating the meta.json to
// include the public namespace (if set on the org).
func UpdateModuleAction(c *cli.Context) error {
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")
	manifestPathArg := c.String("module")

	manifestPath := defaultManifestFilename
	if manifestPathArg != "" {
		manifestPath = manifestPathArg
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}

	moduleID, err := updateManifestModuleIDWithArgs(c, client, manifest.Name, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}

	response, err := client.updateModule(moduleID, manifest)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "module successfully updated! you can view your changes online here: %s\n", response.GetUrl())

	// If the namespace isn't set, modify the meta.json to set it (if available)
	manifestModuleID, err := parseModuleID(manifest.Name)
	if err != nil {
		return err // shouldn't happen because this has already been parsed
	}
	if manifestModuleID.prefix == "" || isValidOrgID(manifestModuleID.prefix) {
		org, err := getOrgByModuleIDPrefix(client, moduleID.prefix)
		if err != nil {
			// hopefully a user never sees this. An alternative would be to fail silently here
			// to prevent the user from being surprised/scared that their update failed
			return errors.Wrap(err, "failed to update meta.json with new information from Viam")
		}
		if org.PublicNamespace != "" {
			moduleID.prefix = org.PublicNamespace
			manifest.Name = moduleID.String()
			if err := writeManifest(manifestPath, manifest); err != nil {
				return errors.Wrap(err, "failed to update meta.json with new information from Viam")
			}
			fmt.Fprintf(c.App.Writer, "\nupdated meta.json to use the public namespace of %q which is %q\n",
				org.Name, org.PublicNamespace)
			infof(c.App.Writer, "you no longer need to specify org-id or public-namespace")
		}
	}
	return nil
}

// UploadModuleAction is the corresponding action for 'module upload'.
func UploadModuleAction(c *cli.Context) error {
	manifestPathArg := c.String("module")
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")
	nameArg := c.String("name")
	versionArg := c.String("version")
	platformArg := c.String("platform")
	tarballPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}
	if tarballPath == "" {
		return errors.New("no package to upload -- please provide an archive containing your module. use --help for more information")
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	manifestPath := defaultManifestFilename
	if manifestPathArg != "" {
		manifestPath = manifestPathArg
	}
	var moduleID moduleID
	// if the manifest cant be found
	if _, err := os.Stat(manifestPath); err != nil {
		// no manifest found.
		if nameArg == "" || (publicNamespaceArg == "" && orgIDArg == "") {
			return errors.New("unable to find the meta.json. " +
				"if you want to upload a version without a meta.json, you must supply a module name and namespace (or module name and org-id)",
			)
		}
		moduleID, err = updateManifestModuleIDWithArgs(c, client, nameArg, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
	} else {
		// if we can find a manifest, use that
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return err
		}

		moduleID, err = updateManifestModuleIDWithArgs(c, client, manifest.Name, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
		if nameArg != "" && nameArg != moduleID.name {
			// This is almost certainly a mistake we want to catch
			return errors.Errorf("module name %q was supplied on the command line but the meta.json has a module name of %q",
				nameArg, moduleID.name)
		}
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	// TODO(APP-2226): support .tar.xz
	if !strings.HasSuffix(file.Name(), ".tar.gz") {
		return errors.New("you must upload your module in the form of a .tar.gz")
	}
	response, err := client.uploadModuleFile(moduleID, versionArg, platformArg, file)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "version successfully uploaded! you can view your changes online here: %s\n", response.GetUrl())

	return nil
}

func (c *appClient) createModule(moduleName, organizationID string) (*apppb.CreateModuleResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := apppb.CreateModuleRequest{
		Name:           moduleName,
		OrganizationId: organizationID,
	}
	return c.client.CreateModule(c.c.Context, &req)
}

func (c *appClient) updateModule(moduleID moduleID, manifest moduleManifest) (*apppb.UpdateModuleResponse, error) {
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

func (c *appClient) uploadModuleFile(
	moduleID moduleID,
	version,
	platform string,
	file *os.File,
) (*apppb.UploadModuleFileResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
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
	defer fmt.Fprint(stdout, "\n")

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
		fmt.Fprintf(stdout, "\r\auploading... %d%% (%d/%d bytes)", uploadPercent, uploadedBytes, fileSize)
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

func moduleComponentToProto(moduleComponent moduleComponent) *apppb.Model {
	return &apppb.Model{
		Api:   moduleComponent.API,
		Model: moduleComponent.Model,
	}
}

func parseModuleID(moduleName string) (moduleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
	// We also allow for empty prefixes here (unlike the backend) to simplify the flexible way to parse user input
	splitModuleName := strings.Split(moduleName, ":")
	switch len(splitModuleName) {
	case 1:
		return moduleID{prefix: "", name: moduleName}, nil
	case 2:
		return moduleID{prefix: splitModuleName[0], name: splitModuleName[1]}, nil
	default:
		return moduleID{}, errors.Errorf("invalid module name '%s'."+
			" module name must be in the form 'prefix:module-name' for public modules"+
			" or just 'module-name' for private modules in organizations without a public namespace", moduleName)
	}
}

func (m *moduleID) String() string {
	if m.prefix == "" {
		return m.name
	}
	return fmt.Sprintf("%s:%s", m.prefix, m.name)
}

// updateManifestModuleIDWithArgs tries to parse the manifestNameEntry to see if it is a valid moduleID with a prefix
// if it is not, it uses the publicNamespaceArg and orgIDArg to determine what the moduleID prefix should be.
func updateManifestModuleIDWithArgs(
	c *cli.Context,
	client *appClient,
	manifestNameEntry,
	publicNamespaceArg,
	orgIDArg string,
) (moduleID, error) {
	mid, err := parseModuleID(manifestNameEntry)
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
					org.GetName(), expectedOrg.GetName())
			}
			fmt.Fprintln(c.App.Writer, "the module's meta.json already specifies a full module id. ignoring public-namespace and org-id arg")
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
func resolveOrg(client *appClient, publicNamespace, orgID string) (*apppb.Organization, error) {
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

func getOrgByModuleIDPrefix(client *appClient, moduleIDPrefix string) (*apppb.Organization, error) {
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
