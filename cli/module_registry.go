package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// ModuleID represents a public_namespace:name pair.
type ModuleID struct {
	Namespace string
	Name      string
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
	orgIDArg := c.String("org_id")
	publicNamespaceArg := c.String("public_namespace")

	client, err := NewAppClient(c)
	if err != nil {
		return err
	}
	org, err := resolveOrg(client, orgIDArg, publicNamespaceArg)
	if err != nil {
		return err
	}
	if org == nil {
		return errors.Errorf("Unable to determine org from orgID(%q) and namespace(%q)", orgIDArg, publicNamespaceArg)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to find the current directory")
	}
	// Check to make sure the user doesn't accidentally overwrite a module manifest
	manifestFilepath := filepath.Join(cwd, defaultManifestFilename)
	if _, err := os.Stat(manifestFilepath); err == nil {
		return errors.Errorf("Another module's %v already exists in the current directory. Delete it and try again", defaultManifestFilename)
	}

	response, err := client.CreateModule(moduleNameArg, org.GetId())
	if err != nil {
		return errors.Wrap(err, "failed to register the module on app.viam.com")
	}

	fmt.Fprintf(c.App.Writer, "Successfully created '%s'.\n", response.GetModuleId())
	if response.GetUrl() != "" {
		fmt.Fprintf(c.App.Writer, "You can view it here: %s \n", response.GetUrl())
	}

	returnedModuleID, err := parseModuleID(response.GetModuleId())
	if err != nil {
		return err
	}
	if returnedModuleID.Namespace == "" && org.PublicNamespace != "" {
		returnedModuleID.Namespace = org.PublicNamespace
	}
	emptyManifest := ModuleManifest{
		Name:       returnedModuleID.toString(),
		Visibility: ModuleVisibilityPrivate,
		// This is done so that the json has an empty example
		Models: []ModuleComponent{
			{},
		},
	}
	if err := writeManifest(manifestFilepath, emptyManifest); err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "Configuration for the module has been written to %s\n", defaultManifestFilename)
	return nil
}

// UpdateModuleCommand runs the command to update a module.
// This includes updating the meta.json to include the public namespace (if set on the org).
func UpdateModuleCommand(c *cli.Context) error {
	publicNamespaceArg := c.String("public_namespace")
	orgIDArg := c.String("org_id")
	manifestPathArg := c.String("module")

	client, err := NewAppClient(c)
	if err != nil {
		return err
	}
	manifestPath := defaultManifestFilename
	if manifestPathArg != "" {
		manifestPath = manifestPathArg
	}
	if _, err := os.Stat(manifestPath); err != nil {
		return errors.Wrapf(err, "Cannot find %s", manifestPath)
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest ModuleManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	moduleID, err := parseModuleID(manifest.Name)
	if err != nil {
		return err
	}
	if publicNamespaceArg != "" {
		switch moduleID.Namespace {
		case "":
			moduleID.Namespace = publicNamespaceArg
		case publicNamespaceArg:
			// the meta.json manifest == public_namespace arg
			fmt.Fprintf(c.App.Writer, "The module's %s already specifies a public namespace. Ignoring\n", defaultManifestFilename)
		default:
			// the meta.json manifest != public_namespace arg
			// we may want to investigate a better way of handling this error case
			// For now, it seems like bad UX to ignore this error
			return errors.Errorf("the module's %s specifies a namespace of '%s'"+
				" but a namespace of '%s' was provided in command line arguments",
				defaultManifestFilename, moduleID.Namespace, publicNamespaceArg)
		}
	}
	var orgID *string
	if orgIDArg != "" {
		if moduleID.Namespace == "" {
			orgID = &orgIDArg
		} else {
			fmt.Fprintf(c.App.Writer, "A public namespace (%s) is specified in the config."+
				" It is not necessary to provide an org_id\n", moduleID.Namespace)
		}
	}
	if orgID == nil && moduleID.Namespace == "" {
		return errors.Errorf("The module's namespace is not set in %s."+
			" You must provide a public_namespace (if you have set one) or supply your org id", defaultManifestFilename)
	}
	manifest.Name = moduleID.toString()

	response, err := client.UpdateModule(manifest, orgID)
	fmt.Fprintf(c.App.Writer, "Module successfully updated! You can view your changes online here: %s\n", response.GetUrl())
	if err != nil {
		return err
	}

	// If the namespace isn't set, modify the meta.json to set it (if available)
	if moduleID.Namespace == "" {
		org, err := resolveOrg(client, orgIDArg, publicNamespaceArg)
		if err != nil {
			return err
		}
		if org.PublicNamespace != "" {
			moduleID.Namespace = org.PublicNamespace
			manifest.Name = moduleID.toString()
			if err := writeManifest(manifestPath, manifest); err != nil {
				return err
			}
			fmt.Fprintf(c.App.Writer, "\nUpdated %s to use the public namespace of %q which is %q\n",
				manifestPath, org.Name, org.PublicNamespace)
			fmt.Fprintf(c.App.Writer, "You no longer need to specify org_id or public_namespace\n")
		}
	}

	return nil
}

func parseModuleID(moduleName string) (ModuleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
	splitModuleName := strings.Split(moduleName, ":")
	switch len(splitModuleName) {
	case 1:
		return ModuleID{Namespace: "", Name: moduleName}, nil
	case 2:
		return ModuleID{Namespace: splitModuleName[0], Name: splitModuleName[1]}, nil
	default:
		return ModuleID{}, errors.Errorf("Invalid module name '%s'."+
			" It must be in the form 'public-namespace:module-name' for public modules"+
			" or just 'module-name' for private modules in organizations without a public namespace", moduleName)
	}
}

func (m *ModuleID) toString() string {
	if m.Namespace == "" {
		return m.Name
	}
	return fmt.Sprintf("%s:%s", m.Namespace, m.Name)
}

// resolveOrg accepts either an orgID or a publicNamespace (one must be an empty string).
// If orgID is an empty string, it will use the publicNamespace to resolve it.
func resolveOrg(client *AppClient, orgID, publicNamespace string) (*apppb.Organization, error) {
	if orgID != "" {
		if publicNamespace != "" {
			return nil, errors.New("cannot specify both org id and public namespace")
		}
		org, err := client.GetOrg(orgID)
		if err != nil {
			return nil, err
		}
		return org, nil
	}
	// Use publicNamespace to back-derive what the org is
	if publicNamespace == "" {
		return nil, errors.New("must specify either org id or public namespace")
	}
	org, err := client.GetUserOrgByPublicNamespace(publicNamespace)
	if err != nil {
		return nil, err
	}
	return org, nil
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
