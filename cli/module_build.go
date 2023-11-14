package cli

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/logging"
)

// ModuleBuildLocalAction runs the module's build commands locally.
func ModuleBuildLocalAction(c *cli.Context) error {
	manifestPath := c.String(moduleFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	var emptyManifestBuildInfo manifestBuildInfo
	if reflect.DeepEqual(manifest.Build, emptyManifestBuildInfo) {
		return errors.New("your meta.json does not contain a build section. See 'viam module build --help' for more information")
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
	if manifest.Build.Build != "" {
		infof(c.App.Writer, "Starting build step: %q", manifest.Build.Build)
		processConfig.Args = []string{"-c", manifest.Build.Build}
		proc := pexec.NewManagedProcess(processConfig, logger.AsZap())
		if err = proc.Start(c.Context); err != nil {
			return err
		}
	} else {
		return errors.New("the build command requires a non-empty build step")
	}
	infof(c.App.Writer, "Completed build")
	return nil
}
