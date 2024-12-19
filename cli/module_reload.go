package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"

	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]any

// ServiceMap is the same kind of thing as ModuleMap (see above), a map representing a single service.
type ServiceMap map[string]any

// addShellService adds a shell service to the services slice if missing. Mutates part.RobotConfig.
func addShellService(c *cli.Context, vc *viamClient, part *apppb.RobotPart, wait bool) error {
	args, err := getGlobalArgs(c)
	if err != nil {
		return err
	}
	partMap := part.RobotConfig.AsMap()
	if _, ok := partMap["services"]; !ok {
		partMap["services"] = make([]any, 0, 1)
	}
	services, _ := rutils.MapOver(partMap["services"].([]any), //nolint:errcheck
		func(raw any) (ServiceMap, error) { return ServiceMap(raw.(map[string]any)), nil },
	)
	if slices.ContainsFunc(services, func(service ServiceMap) bool { return service["type"] == "shell" }) {
		debugf(c.App.Writer, args.Debug, "shell service found on target machine, not installing")
		return nil
	}
	services = append(services, ServiceMap{"name": "shell", "type": "shell"})
	asAny, _ := rutils.MapOver(services, func(service ServiceMap) (any, error) { //nolint:errcheck
		return map[string]any(service), nil
	})
	partMap["services"] = asAny
	if err := writeBackConfig(part, partMap); err != nil {
		return err
	}
	infof(c.App.Writer, "installing shell service on target machine for file transfer")
	if err := vc.updateRobotPart(part, partMap); err != nil {
		return err
	}
	if !wait {
		return nil
	}
	// note: we wait up to 11 seconds; that's the 10 second default Cloud.RefreshInterval plus padding.
	// If we don't wait, the reload command will usually fail on first run.
	for i := 0; i < 11; i++ {
		time.Sleep(time.Second)
		_, closeClient, err := vc.connectToShellServiceFqdn(part.Fqdn, args.Debug, logging.NewLogger("shellsvc"))
		if err == nil {
			goutils.UncheckedError(closeClient(c.Context))
			return nil
		}
		if !errors.Is(err, errNoShellService) {
			return err
		}
	}
	return errors.New("timed out waiting for shell service to start")
}

// writeBackConfig mutates part.RobotConfig with an edited config; this is necessary so that changes
// aren't lost when we make multiple updateRobotPart calls.
func writeBackConfig(part *apppb.RobotPart, configAsMap map[string]any) error {
	modifiedConfig, err := structpb.NewStruct(configAsMap)
	if err != nil {
		return err
	}
	part.RobotConfig = modifiedConfig
	return nil
}

// configureModule is the configuration step of module reloading. Returns (needsRestart, error). Mutates part.RobotConfig.
func configureModule(c *cli.Context, vc *viamClient, manifest *moduleManifest, part *apppb.RobotPart, local bool) (bool, error) {
	if manifest == nil {
		return false, fmt.Errorf("reconfiguration requires valid manifest json passed to --%s", moduleFlagPath)
	}
	partMap := part.RobotConfig.AsMap()
	if _, ok := partMap["modules"]; !ok {
		partMap["modules"] = make([]any, 0, 1)
	}
	modules, err := rutils.MapOver(
		partMap["modules"].([]any),
		func(raw any) (ModuleMap, error) { return ModuleMap(raw.(map[string]any)), nil },
	)
	if err != nil {
		return false, err
	}

	modules, dirty, err := mutateModuleConfig(c, modules, *manifest, local)
	if err != nil {
		return false, err
	}
	// note: converting to any or else proto serializer will fail downstream in NewStruct.
	modulesAsInterfaces, err := rutils.MapOver(modules, func(mod ModuleMap) (any, error) {
		return map[string]any(mod), nil
	})
	if err != nil {
		return false, err
	}
	partMap["modules"] = modulesAsInterfaces
	if err := writeBackConfig(part, partMap); err != nil {
		return false, err
	}
	if dirty {
		args, err := getGlobalArgs(c)
		if err != nil {
			return false, err
		}
		debugf(c.App.Writer, args.Debug, "writing back config changes")
		err = vc.updateRobotPart(part, partMap)
		if err != nil {
			return false, err
		}
	}
	// if we modified config, caller doesn't need to restart module.
	return !dirty, nil
}

// localizeModuleID converts a module ID to its 'local mode' name.
// TODO(APP-4019): remove this logic after registry modules can have local ExecPath.
func localizeModuleID(moduleID string) string {
	return strings.ReplaceAll(moduleID, ":", "_") + "_from_reload"
}

// mutateModuleConfig edits the modules list to hot-reload with the given manifest.
func mutateModuleConfig(c *cli.Context, modules []ModuleMap, manifest moduleManifest, local bool) ([]ModuleMap, bool, error) {
	var dirty bool
	localName := localizeModuleID(manifest.ModuleID)
	var foundMod ModuleMap
	for _, mod := range modules {
		if (mod["module_id"] == manifest.ModuleID) || (mod["name"] == localName) {
			foundMod = mod
			break
		}
	}

	var absEntrypoint string
	var err error
	if local {
		absEntrypoint, err = filepath.Abs(manifest.Entrypoint)
		if err != nil {
			return nil, dirty, err
		}
	} else {
		absEntrypoint = reloadingDestination(c, &manifest)
	}

	args, err := getGlobalArgs(c)
	if err != nil {
		return nil, false, err
	}
	if foundMod == nil {
		debugf(c.App.Writer, args.Debug, "module not found, inserting")
		dirty = true
		newMod := ModuleMap(map[string]any{
			"name":            localName,
			"executable_path": absEntrypoint,
			"type":            string(rdkConfig.ModuleTypeLocal),
		})
		modules = append(modules, newMod)
	} else {
		if same, err := samePath(getMapString(foundMod, "executable_path"), absEntrypoint); err != nil {
			debugf(c.App.Writer, args.Debug, "ExePath is right, doing nothing")
			return nil, dirty, err
		} else if !same {
			dirty = true
			debugf(c.App.Writer, args.Debug, "replacing entrypoint")
			if getMapString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
				// warning: there's a chance of inserting a dupe name here in odd cases
				warningf(c.App.Writer, "You're replacing a registry module. We're converting it to a local module. "+
					"To revert this change, use your machine's history page on app.viam.com.")
				foundMod["type"] = string(rdkConfig.ModuleTypeLocal)
				foundMod["name"] = localName
				delete(foundMod, "module_id")
				// TODO(APP-5844): stop clearing this once backend no longer rejects; we will use it for revert
				delete(foundMod, "version")
			}
			foundMod["executable_path"] = absEntrypoint
		}
	}
	return modules, dirty, nil
}
