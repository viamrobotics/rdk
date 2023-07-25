package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

// ModuleVisibility determines whether modules are public or private.
type ModuleVisibility string

// Permissions enumeration.
const (
	ModuleVisibilityPrivate ModuleVisibility = "private"
	ModuleVisibilityPublic  ModuleVisibility = "public"
)

// ModuleComponent represents an api - model pair.
type ModuleComponent struct {
	API   string `json:"api"`
	Model string `json:"model"`
}

// ModuleID represents a prefix:name pair where prefix can be either an org id or a namespace.
type ModuleID struct {
	Prefix string
	Name   string
}

// ModuleManifest is used to create & parse manifest.json.
type ModuleManifest struct {
	Name        string            `json:"name"`
	Visibility  ModuleVisibility  `json:"visibility"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Models      []ModuleComponent `json:"models"`
	Entrypoint  string            `json:"entrypoint"`
}

const (
	defaultManifestFilename = "meta.json"
)

// CreateModuleCommand runs the command to create a module
// This includes both a gRPC call to register the module on app.viam.com and creating the manifest file.
func CreateModuleCommand(c *cli.Context) error {
	moduleNameArg := c.String("name")
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")

	client, err := NewAppClient(c)
	if err != nil {
		return err
	}
	org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}
	if org == nil {
		return errors.Errorf("Unable to determine org from orgID(%q) and namespace(%q)", orgIDArg, publicNamespaceArg)
	}
	// Check to make sure the user doesn't accidentally overwrite a module manifest
	if _, err := os.Stat(defaultManifestFilename); err == nil {
		return errors.New("Another module's meta.json already exists in the current directory. Delete it and try again")
	}

	response, err := client.CreateModule(moduleNameArg, org.GetId())
	if err != nil {
		return errors.Wrap(err, "failed to register the module on app.viam.com")
	}

	returnedModuleID, err := parseModuleID(response.GetModuleId())
	if err != nil {
		return err
	}
	// The registry team is currently of the opinion that including an org id in the meta.json file
	// is non-ideal.
	// If you do change this, edit the UpdateCommand().. function to also check if the manifestprefix is an orgid
	// during the replacement to a public namespace
	if isValidOrgID(returnedModuleID.Prefix) {
		returnedModuleID.Prefix = ""
	}
	fmt.Fprintf(c.App.Writer, "Successfully created '%s'.\n", returnedModuleID.toString())
	if response.GetUrl() != "" {
		fmt.Fprintf(c.App.Writer, "You can view it here: %s \n", response.GetUrl())
	}
	emptyManifest := ModuleManifest{
		Name:       returnedModuleID.toString(),
		Visibility: ModuleVisibilityPrivate,
		// This is done so that the json has an empty example
		Models: []ModuleComponent{
			{},
		},
	}
	if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "Configuration for the module has been written to meta.json\n")
	return nil
}

// UpdateModuleCommand runs the command to update a module.
// This includes updating the meta.json to include the public namespace (if set on the org).
func UpdateModuleCommand(c *cli.Context) error {
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")
	manifestPathArg := c.String("module")

	manifestPath := defaultManifestFilename
	if manifestPathArg != "" {
		manifestPath = manifestPathArg
	}

	client, err := NewAppClient(c)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}

	moduleID, err := resolveModuleIDFromMultipleSources(c, client, manifest.Name, publicNamespaceArg, orgIDArg)
	if err != nil {
		return err
	}

	response, err := client.UpdateModule(moduleID, manifest)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "Module successfully updated! You can view your changes online here: %s\n", response.GetUrl())

	// If the namespace isn't set, modify the meta.json to set it (if available)
	manifestModuleID, err := parseModuleID(manifest.Name)
	if err != nil {
		return err // shouldn't happen because this has already been parsed
	}
	if manifestModuleID.Prefix == "" || isValidOrgID(manifestModuleID.Prefix) {
		org, err := getOrgByModuleIDPrefix(client, moduleID.Prefix)
		if err != nil {
			// hopefully a user never sees this. An alternative would be to fail silently here
			// to prevent the user from being surprised/scared that their update failed
			return errors.Wrap(err, "error while trying to tidy up the local meta.json")
		}
		if org.PublicNamespace != "" {
			moduleID.Prefix = org.PublicNamespace
			manifest.Name = moduleID.toString()
			if err := writeManifest(manifestPath, manifest); err != nil {
				return errors.Wrap(err, "error while trying to tidy up the local meta.json")
			}
			fmt.Fprintf(c.App.Writer, "\nUpdated meta.json to use the public namespace of %q which is %q\n",
				org.Name, org.PublicNamespace)
			fmt.Fprintf(c.App.Writer, "You no longer need to specify org-id or public-namespace\n")
		}
	}
	return nil
}

// UploadModuleCommand runs the command to upload a new version of a module.
func UploadModuleCommand(c *cli.Context) error {
	manifestPathArg := c.String("module")
	publicNamespaceArg := c.String("public-namespace")
	orgIDArg := c.String("org-id")
	nameArg := c.String("name")
	versionArg := c.String("version")
	platformArg := c.String("platform")
	tarballPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("Too many arguments passed to upload command. " +
			"Make sure to specify flag and optional arguments before the required positional package argument")
	}
	if tarballPath == "" {
		return errors.New("No package to upload -- please provide an archive containing your module. See the help for more information")
	}

	client, err := NewAppClient(c)
	if err != nil {
		return err
	}

	manifestPath := defaultManifestFilename
	if manifestPathArg != "" {
		manifestPath = manifestPathArg
	}
	var moduleID ModuleID
	// if the manifest cant be found
	if _, err := os.Stat(manifestPath); err != nil {
		// no manifest found.
		if nameArg == "" || (publicNamespaceArg == "" && orgIDArg == "") {
			return errors.New("Unable to find the meta.json. " +
				"If you want to upload a version without a meta.json, you must supply a module name and namespace (or module name and orgid)",
			)
		}
		moduleID, err = resolveModuleIDFromMultipleSources(c, client, nameArg, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
	} else {
		// if we can find a manifest, use that
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return err
		}

		moduleID, err = resolveModuleIDFromMultipleSources(c, client, manifest.Name, publicNamespaceArg, orgIDArg)
		if err != nil {
			return err
		}
		if nameArg != "" && nameArg != moduleID.Name {
			// This is almost certainly a mistake we want to catch
			return errors.Errorf("Module name %q was supplied via command line args but the meta.json has a module name of %q",
				nameArg, moduleID.Name)
		}
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	// TODO(APP-2226) support .tar.xz
	if !strings.HasSuffix(file.Name(), ".tar.gz") {
		return errors.New("You must upload your module in the form of a .tar.gz")
	}
	response, err := client.UploadModuleFile(moduleID, versionArg, platformArg, file)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Version successfully uploaded! You can view your changes online here: %s\n", response.GetUrl())

	return nil
}

func parseModuleID(moduleName string) (ModuleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
	// We also allow for empty prefixes here (unlike the backend) to simplify the flexible way to parse user input
	splitModuleName := strings.Split(moduleName, ":")
	switch len(splitModuleName) {
	case 1:
		return ModuleID{Prefix: "", Name: moduleName}, nil
	case 2:
		return ModuleID{Prefix: splitModuleName[0], Name: splitModuleName[1]}, nil
	default:
		return ModuleID{}, errors.Errorf("Invalid module name '%s'."+
			" It must be in the form 'prefix:module-name' for public modules"+
			" or just 'module-name' for private modules in organizations without a public namespace", moduleName)
	}
}

func (m *ModuleID) toString() string {
	if m.Prefix == "" {
		return m.Name
	}
	return fmt.Sprintf("%s:%s", m.Prefix, m.Name)
}

// resolveModuleIDFromMultipleSources tries to parse the manifestNameEntry to see if it is a valid moduleID with a prefix
// if it is not, it uses the publicNamespaceArg and orgIDArg to determine what the moduleID prefix should be
func resolveModuleIDFromMultipleSources(
	c *cli.Context,
	client *AppClient,
	manifestNameEntry,
	publicNamespaceArg,
	orgIDArg string,
) (ModuleID, error) {
	moduleID, err := parseModuleID(manifestNameEntry)
	if err != nil {
		return ModuleID{}, err
	}
	if moduleID.Prefix != "" {
		if publicNamespaceArg != "" || orgIDArg != "" {
			org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
			if err != nil {
				return ModuleID{}, err
			}
			expectedOrg, err := getOrgByModuleIDPrefix(client, moduleID.Prefix)
			if err != nil {
				return ModuleID{}, err
			}
			if org.GetId() != expectedOrg.GetId() {
				// This is almost certainly a user mistake
				// Preferring org name rather than orgid here because the manifest probably has it specified in terms of
				// public_namespace so returning the ids would be frustrating
				return ModuleID{}, errors.Errorf("The meta.json specifies a different org %q than the one provided via args %q",
					org.GetName(), expectedOrg.GetName())

			}
			fmt.Fprintln(c.App.Writer, "The module's meta.json already specifies a full module id. Ignoring public-namespace and org-id arg")
		}
		return moduleID, nil
	}
	// moduleID.Prefix is empty. Need to use orgIDArg and publicNamespaceArg to figure out what it should be
	org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	if err != nil {
		return ModuleID{}, err
	}
	if org.PublicNamespace != "" {
		moduleID.Prefix = org.PublicNamespace
	} else {
		moduleID.Prefix = org.Id
	}
	return moduleID, nil
}

// resolveOrg accepts either an orgID or a publicNamespace (one must be an empty string).
// If orgID is an empty string, it will use the publicNamespace to resolve it.
func resolveOrg(client *AppClient, publicNamespace, orgID string) (*apppb.Organization, error) {
	if orgID != "" {
		if publicNamespace != "" {
			return nil, errors.New("cannot specify both org-id and public-namespace")
		}
		if !isValidOrgID(orgID) {
			return nil, errors.Errorf("provided org-id %q is not a valid org-id", orgID)
		}
		org, err := client.GetOrg(orgID)
		if err != nil {
			return nil, err
		}
		return org, nil
	}
	// Use publicNamespace to back-derive what the org is
	if publicNamespace == "" {
		return nil, errors.New("must provide either org-id or public-namespace")
	}
	org, err := client.GetUserOrgByPublicNamespace(publicNamespace)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func getOrgByModuleIDPrefix(client *AppClient, moduleIDPrefix string) (*apppb.Organization, error) {
	if isValidOrgID(moduleIDPrefix) {
		return client.GetOrg(moduleIDPrefix)
	} else {
		return client.GetUserOrgByPublicNamespace(moduleIDPrefix)
	}
}

// isValidOrgID checks if the str is a valid uuid
func isValidOrgID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

func loadManifest(manifestPath string) (ModuleManifest, error) {
	//nolint:gosec
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ModuleManifest{}, errors.Wrapf(err, "Cannot find %s", manifestPath)
		}
		return ModuleManifest{}, err
	}
	var manifest ModuleManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ModuleManifest{}, err
	}
	return manifest, nil
}

func writeManifest(manifestPath string, manifest ModuleManifest) error {
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
