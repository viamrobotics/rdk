package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	apppb "go.viam.com/api/app/v1"

	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]interface{}

// configureModule is the configuration step of module reloading. Returns (needsRestart, error).
func configureModule(vc *viamClient, manifest *moduleManifest, part *apppb.RobotPart) (bool, error) {
	logger := logging.Global()
	if manifest == nil {
		return false, fmt.Errorf("reconfiguration requires valid manifest json passed to --%s", moduleFlagPath)
	}
	partMap := part.RobotConfig.AsMap()
	modules, err := mapOver(
		partMap["modules"].([]interface{}),
		func(raw interface{}) (ModuleMap, error) { return ModuleMap(raw.(map[string]interface{})), nil },
	)
	if err != nil {
		return false, err
	}

	modules, dirty, err := mutateModuleConfig(modules, *manifest)
	if err != nil {
		return false, err
	}
	// note: converting to interface{} or else proto serializer will fail downstream in NewStruct.
	modulesAsInterfaces, err := mapOver(modules, func(mod ModuleMap) (interface{}, error) {
		return map[string]interface{}(mod), nil
	})
	if err != nil {
		return false, err
	}
	partMap["modules"] = modulesAsInterfaces
	if dirty {
		logger.Debug("writing back config changes")
		err = vc.updateRobotPart(part, partMap)
		if err != nil {
			return false, err
		}
	}
	// if we modified config, caller doesn't need to restart module.
	return !dirty, nil
}

// localizeModuleID converts a module ID to its 'local mode' name.
// TODO(RSDK-6712): remove this logic after registry modules can have local ExecPath.
func localizeModuleID(moduleID string) string {
	return "hr_" + strings.ReplaceAll(moduleID, ":", "_")
}

// mutateModuleConfig edits the modules list to hot-reload with the given manifest.
func mutateModuleConfig(modules []ModuleMap, manifest moduleManifest) ([]ModuleMap, bool, error) {
	var dirty bool
	logger := logging.Global()
	localName := localizeModuleID(manifest.ModuleID)
	var foundMod ModuleMap
	for _, mod := range modules {
		if (mod["module_id"] == manifest.ModuleID) || (mod["name"] == localName) {
			foundMod = mod
			break
		}
	}

	absEntrypoint, err := filepath.Abs(manifest.Entrypoint)
	if err != nil {
		return nil, dirty, err
	}

	if foundMod == nil {
		logger.Debug("module not found, inserting")
		dirty = true
		newMod := ModuleMap(map[string]interface{}{
			"name":            localName,
			"executable_path": absEntrypoint,
			"type":            string(rdkConfig.ModuleTypeLocal),
		})
		modules = append(modules, newMod)
	} else {
		if same, err := samePath(getMapString(foundMod, "executable_path"), absEntrypoint); err != nil {
			logger.Debug("ExePath is right, doing nothing")
			return nil, dirty, err
		} else if !same {
			dirty = true
			logger.Debug("replacing entrypoint")
			if getMapString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
				// warning: there's a chance of inserting a dupe name here in odd cases
				// todo: prompt user
				logger.Warn("you're replacing a registry module. we're converting it to a local module")
				foundMod["type"] = string(rdkConfig.ModuleTypeLocal)
				foundMod["name"] = localName
				foundMod["module_id"] = ""
			}
			foundMod["executable_path"] = absEntrypoint
		}
	}
	return modules, dirty, nil
}
