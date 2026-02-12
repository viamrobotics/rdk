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

const (
	reloadVersionPrefix       = "reload"
	reloadSourceVersionPrefix = "reload-source"
)

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]any

// ResourceMap is the same kind of thing as ModuleMap (see above), a map representing a single resource.
type ResourceMap map[string]any

// addResourceFromModule adds a resource to the components or services slice if missing. Mutates part.RobotConfig.
// Returns an error if the modelName isn't in the manifest, or if the specified resourceName already exists.
func (c *viamClient) addResourceFromModule(
	ctx *cli.Context, part *apppb.RobotPart, manifest *ModuleManifest, modelName, resourceName string,
) error {
	if manifest == nil {
		return errors.New("unable to add resource from config without a meta.json")
	}

	var modelAPI string
	for _, model := range manifest.Models {
		if model.Model == modelName {
			modelAPI = model.API
			break
		}
	}

	if len(modelAPI) == 0 {
		return errors.New("provided model name was not found in the meta.json")
	}

	APISlice := strings.Split(modelAPI, ":")
	if len(APISlice) != 3 {
		return errors.New("the provided model's API is malformed; unable to determine resource type")
	}

	resourceType := APISlice[1] + "s" // `components`, not `component`

	partMap := part.RobotConfig.AsMap()
	if _, ok := partMap[resourceType]; !ok {
		partMap[resourceType] = make([]any, 0, 1)
	}
	resources, _ := rutils.MapOver(partMap[resourceType].([]any), //nolint:errcheck
		func(raw any) (ResourceMap, error) { return ResourceMap(raw.(map[string]any)), nil },
	)

	resourceNameAlreadyExists := func(name string) bool {
		match := rutils.FindInSlice(partMap[resourceType].([]any),
			func(raw any) bool { return raw.(map[string]any)["name"].(string) == name })

		return match != nil
	}

	// if the user provides a resource name but it's already in the config, alert the user and return
	if resourceName != "" {
		if resourceNameAlreadyExists(resourceName) {
			return errors.Errorf("resource name %s already exists in part config", resourceName)
		}
	} else { // if the user doesn't provide a resource name, find a valid one
		resourceNum := 1
		resourceSubtype := APISlice[2]
		for {
			name := fmt.Sprintf("%s-%d", resourceSubtype, resourceNum)
			if resourceNameAlreadyExists(name) {
				resourceNum++
			} else {
				resourceName = name
				break
			}
		}
	}

	resources = append(resources, ResourceMap{"name": resourceName, "api": modelAPI, "model": modelName})
	asAny, _ := rutils.MapOver(resources, func(resource ResourceMap) (any, error) { //nolint:errcheck
		return map[string]any(resource), nil
	})
	partMap[resourceType] = asAny
	if err := writeBackConfig(part, partMap); err != nil {
		return err
	}
	infof(ctx.App.Writer, "installing %s model with name %s on target machine", modelName, resourceName)
	if err := c.updateRobotPart(part, partMap); err != nil {
		return err
	}

	return nil
}

// addShellService adds a shell service to the services slice if missing. Mutates part.RobotConfig.
// Returns (wasAdded, error) where wasAdded indicates if the shell service was newly added.
func addShellService(c *cli.Context, vc *viamClient, logger logging.Logger, part *apppb.RobotPart, wait bool) (bool, error) {
	args, err := getGlobalArgs(c)
	if err != nil {
		return false, err
	}
	partMap := part.RobotConfig.AsMap()
	if _, ok := partMap["services"]; !ok {
		partMap["services"] = make([]any, 0, 1)
	}
	services, _ := rutils.MapOver(partMap["services"].([]any), //nolint:errcheck
		func(raw any) (ResourceMap, error) { return ResourceMap(raw.(map[string]any)), nil },
	)
	if slices.ContainsFunc(services, func(service ResourceMap) bool {
		return service["type"] == "shell" || service["api"] == "rdk:service:shell"
	}) {
		debugf(c.App.Writer, args.Debug, "shell service found on target machine, not installing")
		return false, nil
	}
	services = append(services, ResourceMap{"name": "shell", "api": "rdk:service:shell"})
	asAny, _ := rutils.MapOver(services, func(service ResourceMap) (any, error) { //nolint:errcheck
		return map[string]any(service), nil
	})
	partMap["services"] = asAny
	if err := writeBackConfig(part, partMap); err != nil {
		return false, err
	}
	if err := vc.updateRobotPart(part, partMap); err != nil {
		return false, err
	}
	if !wait {
		return true, nil
	}
	// note: we wait up to 11 seconds; that's the 10 second default Cloud.RefreshInterval plus padding.
	// If we don't wait, the reload command will usually fail on first run.
	for i := 0; i < 11; i++ {
		time.Sleep(time.Second)
		_, closeClient, err := vc.connectToShellServiceFqdn(part.Fqdn, args.Debug, logger)
		if err == nil {
			goutils.UncheckedError(closeClient(c.Context))
			return true, nil
		}
		if !errors.Is(err, errNoShellService) {
			return false, err
		}
	}
	return false, errors.New("timed out waiting for shell service to start")
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

// configureModule is the configuration step of module reloading. Returns (updated robotPartpart, needsRestart, error).
// Mutates the passed part.RobotConfig.
func configureModule(
	c *cli.Context, vc *viamClient, manifest *ModuleManifest, part *apppb.RobotPart, local bool,
) (*apppb.RobotPart, bool, error) {
	if manifest == nil {
		return part, false, fmt.Errorf("reconfiguration requires valid manifest json passed to --%s", moduleFlagPath)
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
		return part, false, err
	}

	modules, dirty, err := mutateModuleConfig(c, modules, *manifest, local)
	if err != nil {
		return part, false, err
	}
	// note: converting to any or else proto serializer will fail downstream in NewStruct.
	modulesAsInterfaces, err := rutils.MapOver(modules, func(mod ModuleMap) (any, error) {
		return map[string]any(mod), nil
	})
	if err != nil {
		return part, false, err
	}
	partMap["modules"] = modulesAsInterfaces
	if err := writeBackConfig(part, partMap); err != nil {
		return part, false, err
	}
	if dirty {
		args, err := getGlobalArgs(c)
		if err != nil {
			return part, false, err
		}
		debugf(c.App.Writer, args.Debug, "writing back config changes")
		err = vc.updateRobotPart(part, partMap)
		if err != nil {
			return part, false, err
		}
	}

	// there is an issue whereby mutations are getting lost or not properly reflected in the
	// robotPart config after the robot part is updated. To address this, we query the part again
	// to get the most up-to-date version, and return it for further use.
	partResponse, err := vc.getRobotPart(part.Id)
	if err != nil {
		return part, !dirty, err
	}
	// if we modified config, caller doesn't need to restart module.
	return partResponse.Part, !dirty, nil
}

// localizeModuleID converts a module ID to its 'local mode' name.
// TODO(APP-4019): remove this logic after registry modules can have local ExecPath.
func localizeModuleID(moduleID string) string {
	return strings.ReplaceAll(moduleID, ":", "_") + "_from_reload"
}

// mutateModuleConfig edits the modules list to hot-reload with the given manifest.
func mutateModuleConfig(
	c *cli.Context,
	modules []ModuleMap,
	manifest ModuleManifest,
	local bool,
) ([]ModuleMap, bool, error) {
	var dirty bool
	var foundMod ModuleMap
	for _, mod := range modules {
		if mod["module_id"] == manifest.ModuleID {
			foundMod = mod
			break
		}
	}

	var absEntrypoint string
	var err error
	if local {
		// This flag means that viam server is running on the same machine running the CLI
		// Does not indicate module type (registry vs local)
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

	if foundMod != nil && getMapString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
		samePath, err := samePath(getMapString(foundMod, "reload_path"), absEntrypoint)
		if err != nil {
			return nil, dirty, err
		}
		reloadFlag := foundMod["reload_enabled"]
		if samePath && reloadFlag == true {
			debugf(c.App.Writer, args.Debug, "ReloadPath is up to date and ReloadEnabled, doing nothing")
			return nil, dirty, err
		}
		dirty = true
		if samePath {
			debugf(c.App.Writer, args.Debug, "ReloadPath is up to date, setting ReloadEnabled true")
			foundMod["reload_enabled"] = true
		} else {
			debugf(c.App.Writer, args.Debug, "updating ReloadPath and ReloadEnabled")
			foundMod["reload_path"] = absEntrypoint
			foundMod["reload_enabled"] = true
		}
	} else {
		dirty = true
		if foundMod == nil {
			debugf(c.App.Writer, args.Debug, "module not found, inserting")
		} else {
			debugf(c.App.Writer, args.Debug, "found local module, inserting registry module")
		}
		newMod := createNewModuleMap(manifest.ModuleID, absEntrypoint)
		modules = append(modules, newMod)
	}
	return modules, dirty, nil
}

func createNewModuleMap(moduleID, entryPoint string) ModuleMap {
	localName := localizeModuleID(moduleID)
	newMod := ModuleMap(map[string]any{
		"type":           string(rdkConfig.ModuleTypeRegistry),
		"module_id":      moduleID,
		"name":           localName,
		"reload_path":    entryPoint,
		"reload_enabled": true,
		"version":        "latest-with-prerelease",
	})
	return newMod
}
