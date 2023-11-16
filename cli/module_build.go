package cli

import (
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	buildpb "go.viam.com/api/app/build/v1"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/logging"
)

func (c *viamClient) startBuild(moduleID string, arch []string) (*buildpb.StartBuildResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.StartBuildRequest{
		Arch:     arch,
		ModuleId: moduleID,
	}
	return c.buildClient.StartBuild(c.c.Context, &req)
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(c *cli.Context) error {
	manifest, err := loadManifest(c.String(moduleFlagPath))
	if err != nil {
		return err
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	platforms := manifest.Build.Arch
	if len(platforms) == 0 {
		platforms = defaultBuildInfo.Arch
	}
	res, err := client.startBuild(manifest.ModuleID, platforms)
	if err != nil {
		return err
	}
	// org, err := resolveOrg(client, publicNamespaceArg, orgIDArg)
	// if err != nil {
	// 	return err
	// }
	// // Check to make sure the user doesn't accidentally overwrite a module manifest
	// if _, err := os.Stat(defaultManifestFilename); err == nil {
	// 	return errors.New("another module's meta.json already exists in the current directory. Delete it and try again")
	// }

	// response, err := client.createModule(moduleNameArg, org.GetId())
	// if err != nil {
	// 	return errors.Wrap(err, "failed to register the module on app.viam.com")
	// }

	// returnedModuleID, err := parseModuleID(response.GetModuleId())
	// if err != nil {
	// 	return err
	// }

	// printf(c.App.Writer, "Successfully created '%s'", returnedModuleID.String())
	// if response.GetUrl() != "" {
	// 	printf(c.App.Writer, "You can view it here: %s", response.GetUrl())
	// }
	// emptyManifest := moduleManifest{
	// 	ModuleID:   returnedModuleID.String(),
	// 	Visibility: moduleVisibilityPrivate,
	// 	// This is done so that the json has an empty example
	// 	Models: []ModuleComponent{
	// 		{},
	// 	},
	// }
	// if err := writeManifest(defaultManifestFilename, emptyManifest); err != nil {
	// 	return err
	// }

	// todo: change to VendorID
	printf(c.App.Writer, "got'em %s\n", *res.GithubId)
	return nil
}

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Build.Build == "" {
		return errors.New("your meta.json cannot have an empty build step. See 'viam module build --help' for more information")
	}
	infof(c.App.Writer, "Starting build")
	processConfig := pexec.ProcessConfig{
		Name:      "bash",
		OneShot:   true,
		Log:       true,
		LogWriter: c.App.Writer,
	}
	// Required logger for the ManagedProcess. Not used
	logger := logging.NewLogger("x")
	if manifest.Build.Setup != "" {
		infof(c.App.Writer, "Starting setup step: %q", manifest.Build.Setup)
		processConfig.Args = []string{"-c", manifest.Build.Setup}
		proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
		if err = proc.Start(c.Context); err != nil {
			return err
		}
	}
	infof(c.App.Writer, "Starting build step: %q", manifest.Build.Build)
	processConfig.Args = []string{"-c", manifest.Build.Build}
	proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
	if err = proc.Start(c.Context); err != nil {
		return err
	}
	infof(c.App.Writer, "Completed build")
	return nil
}
