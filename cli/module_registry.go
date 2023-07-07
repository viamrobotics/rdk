package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// These types mirror app's internal representations of modules

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
	publicNamespace, err := resolvePublicNamespace(client, orgIDArg, publicNamespaceArg)
	if err != nil {
		return err
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

	response, err := client.CreateModule(moduleNameArg, publicNamespace)
	if err != nil {
		return errors.Wrap(err, "failed to register the module on app.viam.com")
	}

	fmt.Fprintf(c.App.Writer, "Successfully created '%s'.\n", response.GetModuleId())
	if response.GetUrl() != "" {
		fmt.Fprintf(c.App.Writer, "You can view it here: %s \n", response.GetUrl())
	}

	emptyManifest := ModuleManifest{
		Name:       response.GetModuleId(),
		Visibility: ModuleVisibilityPrivate,
		// This is done so that the json has an empty example
		Models: []ModuleComponent{
			{},
		},
	}
	emptyManifestBytes, err := json.MarshalIndent(emptyManifest, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	manifestFile, err := os.Create(manifestFilepath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", defaultManifestFilename)
	}
	if _, err := manifestFile.Write(emptyManifestBytes); err != nil {
		return errors.Wrapf(err, "failed to write manifest to %s", defaultManifestFilename)
	}
	fmt.Fprintf(c.App.Writer, "Configuration for the module has been written to %s\n", defaultManifestFilename)
	return nil
}

// resolvePublicNamespace accepts either an orgID or a publicNamespace (one must be an empty string).
// If publicNamespace is an empty string, it will use the orgID to resolve it.
func resolvePublicNamespace(client *AppClient, orgID, publicNamespace string) (string, error) {
	if publicNamespace != "" {
		if orgID != "" {
			return "", errors.New("cannot specify both org id and public namespace")
		}
		return publicNamespace, nil
	}
	// Use orgID to back-derive what the public namespace is
	if orgID == "" {
		return "", errors.New("must specify either org id or public namespace")
	}
	if err := client.selectOrganization(orgID); err != nil {
		return "", err
	}
	selectedOrg := client.selectedOrg
	if selectedOrg == nil {
		return "", errors.New("unable to find specified organization")
	}
	if selectedOrg.PublicNamespace == "" {
		return "", errors.New("you must claim a public namespace for your org on the settings page on app.viam.com")
	}
	return selectedOrg.PublicNamespace, nil
}
