package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"

	rdkConfig "go.viam.com/rdk/config"
	rutils "go.viam.com/rdk/utils"
)

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]any

// ServiceMap is the same kind of thing as ModuleMap (see above), a map representing a single service.
type ServiceMap map[string]any

// addShellService adds a shell service to the services slice if missing.
func addShellService(c *cli.Context, vc *viamClient, part *apppb.RobotPart) error {
	partMap := part.RobotConfig.AsMap()
	if _, ok := partMap["services"]; !ok {
		partMap["services"] = make([]any, 0, 1)
	}
	services, _ := rutils.MapOver(partMap["services"].([]any),
		func(raw any) (ServiceMap, error) { return ServiceMap(raw.(map[string]any)), nil },
	)
	if slices.ContainsFunc(services, func(service ServiceMap) bool { return service["type"] == "shell" }) {
		debugf(c.App.Writer, c.Bool(debugFlag), "shell service found on target machine, not installing")
		return nil
	}
	services = append(services, ServiceMap{"type": "shell"})
	asAny, _ := rutils.MapOver(services, func(service ServiceMap) (any, error) {
		return map[string]any(service), nil
	})
	partMap["services"] = asAny
	infof(c.App.Writer, "installing shell service on target machine for file transfer")
	return vc.updateRobotPart(part, partMap)
}

// configureModule is the configuration step of module reloading. Returns (needsRestart, error).
func configureModule(c *cli.Context, vc *viamClient, manifest *moduleManifest, part *apppb.RobotPart) (bool, error) {
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

	modules, dirty, err := mutateModuleConfig(c, modules, *manifest)
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
	if dirty {
		debugf(c.App.Writer, c.Bool(debugFlag), "writing back config changes")
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
func mutateModuleConfig(c *cli.Context, modules []ModuleMap, manifest moduleManifest) ([]ModuleMap, bool, error) {
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
	if c.Bool(moduleFlagLocal) {
		absEntrypoint, err = filepath.Abs(manifest.Entrypoint)
		if err != nil {
			return nil, dirty, err
		}
	} else {
		return nil, false, errors.New("todo: entrypoint path in remote case")
	}

	if foundMod == nil {
		debugf(c.App.Writer, c.Bool(debugFlag), "module not found, inserting")
		dirty = true
		newMod := ModuleMap(map[string]any{
			"name":            localName,
			"executable_path": absEntrypoint,
			"type":            string(rdkConfig.ModuleTypeLocal),
		})
		modules = append(modules, newMod)
	} else {
		if same, err := samePath(getMapString(foundMod, "executable_path"), absEntrypoint); err != nil {
			debugf(c.App.Writer, c.Bool(debugFlag), "ExePath is right, doing nothing")
			return nil, dirty, err
		} else if !same {
			dirty = true
			debugf(c.App.Writer, c.Bool(debugFlag), "replacing entrypoint")
			if getMapString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
				// warning: there's a chance of inserting a dupe name here in odd cases
				warningf(c.App.Writer, "you're replacing a registry module. we're converting it to a local module")
				foundMod["type"] = string(rdkConfig.ModuleTypeLocal)
				foundMod["name"] = localName
				foundMod["module_id"] = ""
			}
			foundMod["executable_path"] = absEntrypoint
		}
	}
	return modules, dirty, nil
}
