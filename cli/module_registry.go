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

// ModuleID represents a prefix:name pair where prefix can be either an org id or a namespace
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
	if returnedModuleID.Prefix == "" && org.PublicNamespace != "" {
		returnedModuleID.Prefix = org.PublicNamespace
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

	moduleID, err := parseModuleID(manifest.Name)
	if err != nil {
		return err
	}
	// TODO(zaporter) This logic is duplicated in update and upload but will be refactored once we figure out what we are doing with orgid
	if publicNamespaceArg != "" {
		switch moduleID.Prefix {
		case "":
			moduleID.Prefix = publicNamespaceArg
		case publicNamespaceArg:
			// the meta.json manifest == public_namespace arg
			fmt.Fprintf(c.App.Writer, "The module's %s already specifies a public namespace. Ignoring\n", defaultManifestFilename)
		default:
			// the meta.json manifest != public_namespace arg
			// we may want to investigate a better way of handling this error case
			// For now, it seems like bad UX to ignore this error
			return errors.Errorf("the module's %s specifies a namespace of '%s'"+
				" but a namespace of '%s' was provided in command line arguments",
				defaultManifestFilename, moduleID.Prefix, publicNamespaceArg)
		}
	}
	var orgID *string
	if orgIDArg != "" {
		if moduleID.Prefix == "" {
			orgID = &orgIDArg
		} else {
			fmt.Fprintf(c.App.Writer, "A public namespace (%s) is specified in the config."+
				" It is not necessary to provide an org_id\n", moduleID.Prefix)
		}
	}
	if orgID == nil && moduleID.Prefix == "" {
		return errors.Errorf("The module's namespace is not set in %s."+
			" You must provide a public_namespace (if you have set one) or supply your org id", defaultManifestFilename)
	}
	manifest.Name = moduleID.toString()
	// end duplicated logic

	response, err := client.UpdateModule(manifest, orgID)
	fmt.Fprintf(c.App.Writer, "Module successfully updated! You can view your changes online here: %s\n", response.GetUrl())
	if err != nil {
		return err
	}

	// If the namespace isn't set, modify the meta.json to set it (if available)
	if moduleID.Prefix == "" {
		org, err := resolveOrg(client, orgIDArg, publicNamespaceArg)
		if err != nil {
			return err
		}
		if org.PublicNamespace != "" {
			moduleID.Prefix = org.PublicNamespace
			manifest.Name = moduleID.toString()
			if err := writeManifest(manifestPath, manifest); err != nil {
				return err
			}
			fmt.Fprintf(c.App.Writer, "\nUpdated %s to use the public namespace of %q which is %q\n",
				defaultManifestFilename, org.Name, org.PublicNamespace)
			fmt.Fprintf(c.App.Writer, "You no longer need to specify org_id or public_namespace\n")
		}
	}

	return nil
}

func UploadModuleCommand(c *cli.Context) error {
	manifestPathArg := c.String("module")
	publicNamespaceArg := c.String("public_namespace")
	orgIDArg := c.String("org_id")
	nameArg := c.String("name")
	versionArg := c.String("version")
	platformArg := c.String("platform")
	tarballPath := c.Args().First()

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

			return errors.Errorf("Unable to find %s. If you want to upload a version without a %s, you must supply a module name and namespace (or module name and orgid)\n\n", defaultManifestFilename, defaultManifestFilename)
		}
		moduleID = ModuleID{Prefix: publicNamespaceArg, Name: nameArg}
	} else {
        // if we can find a manifest, use that
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			fmt.Fprintf(c.App.ErrWriter, "If you want to upload a version without a %s, you must supply a module name and namespace (or module name and orgid)\n\n", defaultManifestFilename)
			return err
		}


		moduleID, err = parseModuleID(manifest.Name)
		if err != nil {
			return err
		}
        if nameArg != "" && nameArg != moduleID.Name{
            // This is almost certainly a mistake we want to catch
            return errors.Errorf("Module name %q was supplied via command line args but the %s has a module name of %q", nameArg, defaultManifestFilename, moduleID.Name)
        }
	}
	// TODO(zaporter) This logic is duplicated in update and upload but will be refactored once we figure out what we are doing with orgid
	if publicNamespaceArg != "" {
		switch moduleID.Prefix {
		case "":
			moduleID.Prefix = publicNamespaceArg
		case publicNamespaceArg:
			// the meta.json manifest == public_namespace arg
			fmt.Fprintf(c.App.Writer, "The module's %s already specifies a public namespace. Ignoring\n", defaultManifestFilename)
		default:
			// the meta.json manifest != public_namespace arg
			// we may want to investigate a better way of handling this error case
			// For now, it seems like bad UX to ignore this error
			return errors.Errorf("the module's %s specifies a namespace of '%s'"+
				" but a namespace of '%s' was provided in command line arguments",
				defaultManifestFilename, moduleID.Prefix, publicNamespaceArg)
		}
	}

	var orgID *string
	if orgIDArg != "" {
		if moduleID.Prefix == "" {
			orgID = &orgIDArg
		} else {
			fmt.Fprintf(c.App.Writer, "A public namespace (%s) is specified in the config."+
				" It is not necessary to provide an org_id\n", moduleID.Prefix)
		}
	}
	if orgID == nil && moduleID.Prefix == "" {
		return errors.Errorf("The module's namespace is not set in %s."+
			" You must provide a public_namespace (if you have set one) or supply your org id", defaultManifestFilename)
	}
	// end duplicated logic

	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	response, err := client.UploadModuleFile(moduleID.toString(), versionArg, platformArg, orgID, file)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Version successfully uploaded! You can view your changes online here: %s\n", response.GetUrl())

	return nil
}

func parseModuleID(moduleName string) (ModuleID, error) {
	// This parsing is intentionally lenient so that the backend does the real validation
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

func loadManifest(manifestPath string) (ModuleManifest, error) {
	if _, err := os.Stat(manifestPath); err != nil {
		return ModuleManifest{}, errors.Wrapf(err, "Cannot find %s", manifestPath)
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
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
