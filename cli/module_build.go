package cli

import (
	buildpb "go.viam.com/api/app/build/v1"

	"github.com/urfave/cli/v2"
)

func (c *viamClient) startBuild(arch []string) (*buildpb.StartBuildResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req := buildpb.StartBuildRequest{
		Arch: arch,
	}
	return c.buildClient.StartBuild(c.c.Context, &req)
}

// ModuleBuildStartAction starts a cloud build.
func ModuleBuildStartAction(c *cli.Context) error {
	// moduleNameArg := c.String(moduleFlagName)
	// publicNamespaceArg := c.String(moduleFlagPublicNamespace)
	// orgIDArg := c.String(moduleFlagOrgID)

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
	res, err := client.startBuild(platforms)
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
