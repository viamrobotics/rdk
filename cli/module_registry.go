package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
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
	defaultManifestFilename = "manifest.json"
)

// CreateModule will call the gRPC method to create a module and then will create a manifest.json with the resulting info.
func (c *AppClient) CreateModule(moduleName, publicNamespace string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current directory")
	}
	// Check to make sure the user doesn't accidentally overwrite a module manifest
	manifestFilepath := filepath.Join(cwd, defaultManifestFilename)
	if _, err := os.Stat(manifestFilepath); err == nil {
		return errors.Errorf("A module's %v already exists in the current directory. Delete it and try again", defaultManifestFilename)
	}

	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	req := apppb.CreateModuleRequest{
		Name:            moduleName,
		PublicNamespace: publicNamespace,
	}
	resp, err := c.client.CreateModule(c.c.Context, &req)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.c.App.Writer, "Successfully created '%s'.\n", resp.GetModuleId())

	emptyManifest := ModuleManifest{
		Name: resp.GetModuleId(),
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
	fmt.Fprintf(c.c.App.Writer, "Configuration for the module has been written to %s\n", defaultManifestFilename)
	return nil
}
