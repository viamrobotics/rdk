package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
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

// reloadUser returns the identity string to stamp on reload configs.
// For token-based auth this is the user's email; for API keys it's the key ID.
func reloadUser(conf *Config) string {
	// let reload succeed even if the auth is nil, lets monitor to see if this case ever happens.
	if conf == nil || conf.Auth == nil {
		return ""
	}
	return conf.Auth.String()
}

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]any

// ResourceMap is the same kind of thing as ModuleMap (see above), a map representing a single resource.
type ResourceMap map[string]any

// applyResourceToPartMap adds a resource to the components or services slice if missing. Mutates the partMap.
// Returns an error if the modelName isn't in the manifest, or if the specified resourceName already exists.
func applyResourceToPartMap(
	partMap map[string]any, manifest *ModuleManifest, modelName, resourceName string,
) (string, error) {
	if manifest == nil {
		return "", errors.New("unable to add resource from config without a meta.json")
	}

	var modelAPI string
	for _, model := range manifest.Models {
		if model.Model == modelName {
			modelAPI = model.API
			break
		}
	}

	if len(modelAPI) == 0 {
		return "", errors.New("provided model name was not found in the meta.json")
	}

	APISlice := strings.Split(modelAPI, ":")
	if len(APISlice) != 3 {
		return "", errors.New("the provided model's API is malformed; unable to determine resource type")
	}

	resourceType := APISlice[1] + "s" // `components`, not `component`

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
			return "", errors.Errorf("resource name %s already exists in part config", resourceName)
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
	return resourceName, nil
}

// addResourceFromModule adds a resource to the components or services slice if missing.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func (c *viamClient) addResourceFromModule(
	cmd *cli.Command, part *apppb.RobotPart, manifest *ModuleManifest, modelName, resourceName string,
) error {
	partMap := part.RobotConfig.AsMap()
	resolvedName, err := applyResourceToPartMap(partMap, manifest, modelName, resourceName)
	if err != nil {
		return err
	}
	if err := writeBackConfig(part, partMap); err != nil {
		return err
	}
	infof(cmd.Root().Writer, "installing %s model with name %s on target machine", modelName, resolvedName)

	if err := c.updateRobotPart(context.Background(), part, partMap, part.LastUpdated); err != nil {
		partResp, refetchErr := c.getRobotPart(context.Background(), part.Id)
		if refetchErr != nil {
			return errors.Wrap(err, "update failed and could not re-fetch part config")
		}
		retryPart := partResp.Part
		retryMap := retryPart.RobotConfig.AsMap()
		if _, retryErr := applyResourceToPartMap(retryMap, manifest, modelName, resourceName); retryErr != nil {
			return retryErr
		}
		if retryErr := writeBackConfig(retryPart, retryMap); retryErr != nil {
			return retryErr
		}
		if retryErr := c.updateRobotPart(context.Background(), retryPart, retryMap, retryPart.LastUpdated); retryErr != nil {
			return errors.Wrap(retryErr, "retry of addResourceFromModule also failed")
		}
	}
	return nil
}

// hasShellService checks if a partMap already has a shell service configured.
func hasShellService(partMap map[string]any) bool {
	servicesRaw, ok := partMap["services"]
	if !ok {
		return false
	}
	services, _ := rutils.MapOver(servicesRaw.([]any), //nolint:errcheck
		func(raw any) (ResourceMap, error) { return ResourceMap(raw.(map[string]any)), nil },
	)
	return slices.ContainsFunc(services, func(service ResourceMap) bool {
		return service["type"] == "shell" || service["api"] == "rdk:service:shell"
	})
}

// applyShellServiceToPartMap adds a shell service to the partMap if not already present. Returns true if added.
func applyShellServiceToPartMap(partMap map[string]any) bool {
	if _, ok := partMap["services"]; !ok {
		partMap["services"] = make([]any, 0, 1)
	}
	if hasShellService(partMap) {
		return false
	}
	services, _ := rutils.MapOver(partMap["services"].([]any), //nolint:errcheck
		func(raw any) (ResourceMap, error) { return ResourceMap(raw.(map[string]any)), nil },
	)
	services = append(services, ResourceMap{"name": "shell", "api": "rdk:service:shell"})
	asAny, _ := rutils.MapOver(services, func(service ResourceMap) (any, error) { //nolint:errcheck
		return map[string]any(service), nil
	})
	partMap["services"] = asAny
	return true
}

// addShellService adds a shell service to the services slice if missing. Mutates part.RobotConfig.
// Returns (wasAdded, error) where wasAdded indicates if the shell service was newly added.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func addShellService(
	ctx context.Context, cmd *cli.Command, vc *viamClient, logger logging.Logger, part *apppb.RobotPart, wait bool,
) (bool, error) {
	args, err := getGlobalArgs(cmd)
	if err != nil {
		return false, err
	}
	partMap := part.RobotConfig.AsMap()
	if !applyShellServiceToPartMap(partMap) {
		debugf(cmd.Root().Writer, args.Debug, "shell service found on target machine, not installing")
		return false, nil
	}
	if err := writeBackConfig(part, partMap); err != nil {
		return false, err
	}
	if err := vc.updateRobotPart(ctx, part, partMap, part.LastUpdated); err != nil {
		partResp, refetchErr := vc.getRobotPart(ctx, part.Id)
		if refetchErr != nil {
			return false, errors.Wrap(err, "update failed and could not re-fetch part config")
		}
		retryPart := partResp.Part
		retryMap := retryPart.RobotConfig.AsMap()
		if !applyShellServiceToPartMap(retryMap) {
			debugf(cmd.Root().Writer, args.Debug, "shell service found on target machine after re-fetch, not installing")
			return false, nil
		}
		if retryErr := writeBackConfig(retryPart, retryMap); retryErr != nil {
			return false, retryErr
		}
		if retryErr := vc.updateRobotPart(ctx, retryPart, retryMap, retryPart.LastUpdated); retryErr != nil {
			return false, errors.Wrap(retryErr, "retry of addShellService also failed")
		}
	}
	if !wait {
		return true, nil
	}
	// note: we wait up to 11 seconds; that's the 10 second default Cloud.RefreshInterval plus padding.
	// If we don't wait, the reload command will usually fail on first run.
	for i := 0; i < 11; i++ {
		time.Sleep(time.Second)
		_, closeClient, err := vc.connectToShellServiceFqdn(ctx, part.Fqdn, args.Debug, logger)
		if err == nil {
			goutils.UncheckedError(closeClient(ctx))
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

// applyModuleConfigToPartMap applies the module reload configuration changes to a partMap.
// Returns (modules, dirty, needsRestart, error).
// dirty means config was changed and needs to be written back.
// needsRestart means the module needs an explicit restart (the reload path was already correct).
func applyModuleConfigToPartMap(
	cmd *cli.Command,
	partMap map[string]any,
	manifest ModuleManifest,
	local,
	cloudReload bool,
	reloadUser,
	annotation string,
	reloadUnixTS int64,
) ([]ModuleMap, bool, bool, error) {
	if _, ok := partMap["modules"]; !ok {
		partMap["modules"] = make([]any, 0, 1)
	}
	modules, err := rutils.MapOver(
		partMap["modules"].([]any),
		func(raw any) (ModuleMap, error) { return ModuleMap(raw.(map[string]any)), nil },
	)
	if err != nil {
		return nil, false, false, err
	}

	modules, dirty, needsRestart, err := mutateModuleConfig(cmd, modules, manifest, local, cloudReload, reloadUser, annotation, reloadUnixTS)
	if err != nil {
		return nil, false, false, err
	}
	modulesAsInterfaces, err := rutils.MapOver(modules, func(mod ModuleMap) (any, error) {
		return map[string]any(mod), nil
	})
	if err != nil {
		return nil, false, false, err
	}
	partMap["modules"] = modulesAsInterfaces
	return modules, dirty, needsRestart, nil
}

// configureModule is the configuration step of module reloading. Returns (updated robotPartpart, needsRestart, error).
// Mutates the passed part.RobotConfig.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func configureModule(
	ctx context.Context, cmd *cli.Command, vc *viamClient, manifest *ModuleManifest, part *apppb.RobotPart,
	local, cloudReload bool, reloadUser, annotation string, reloadUnixTS int64,
) (*apppb.RobotPart, bool, error) {
	if manifest == nil {
		return part, false, fmt.Errorf("reconfiguration requires valid manifest json passed to --%s", moduleFlagPath)
	}
	partMap := part.RobotConfig.AsMap()
	_, dirty, needsRestart, err := applyModuleConfigToPartMap(
		cmd, partMap, *manifest, local, cloudReload, reloadUser, annotation, reloadUnixTS)
	if err != nil {
		return part, false, err
	}
	if err := writeBackConfig(part, partMap); err != nil {
		return part, false, err
	}
	if dirty {
		args, err := getGlobalArgs(cmd)
		if err != nil {
			return part, false, err
		}
		debugf(cmd.Root().Writer, args.Debug, "writing back config changes")
		if err := vc.updateRobotPart(ctx, part, partMap, part.LastUpdated); err != nil {
			partResp, refetchErr := vc.getRobotPart(ctx, part.Id)
			if refetchErr != nil {
				return part, false, errors.Wrap(err, "update failed and could not re-fetch part config")
			}
			retryPart := partResp.Part
			retryMap := retryPart.RobotConfig.AsMap()
			_, _, _, retryErr := applyModuleConfigToPartMap(cmd, retryMap, *manifest, local, cloudReload, reloadUser, annotation, reloadUnixTS)
			if retryErr != nil {
				return part, false, retryErr
			}
			if retryErr = writeBackConfig(retryPart, retryMap); retryErr != nil {
				return part, false, retryErr
			}
			if retryErr = vc.updateRobotPart(ctx, retryPart, retryMap, retryPart.LastUpdated); retryErr != nil {
				return part, false, errors.Wrap(retryErr, "retry of configureModule also failed")
			}
		}
	}

	// there is an issue whereby mutations are getting lost or not properly reflected in the
	// robotPart config after the robot part is updated. To address this, we query the part again
	// to get the most up-to-date version, and return it for further use.
	partResponse, err := vc.getRobotPart(ctx, part.Id)
	if err != nil {
		return part, false, err
	}
	return partResponse.Part, needsRestart, nil
}

// localizeModuleID converts a module ID to its 'local mode' name.
// TODO(APP-4019): remove this logic after registry modules can have local ExecPath.
func localizeModuleID(moduleID string) string {
	return strings.ReplaceAll(moduleID, ":", "_") + "_from_reload"
}

// mutateModuleConfig edits the modules list to hot-reload with the given manifest.
// reloadUser is the email/identity of the user performing the reload.
// Returns (modules, dirty, needsRestart, error).
// dirty means config was changed and needs to be written back.
// needsRestart means the module binary needs an explicit restart (the reload path/enabled were already correct).
func mutateModuleConfig(
	cmd *cli.Command,
	modules []ModuleMap,
	manifest ModuleManifest,
	local bool,
	cloudReload bool,
	reloadUser string,
	annotation string,
	reloadUnixTS int64,
) ([]ModuleMap, bool, bool, error) {
	var dirty bool
	var needsRestart bool
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
			return nil, dirty, false, err
		}
	} else {
		absEntrypoint = reloadingDestination(cmd, &manifest)
	}

	args, err := getGlobalArgs(cmd)
	if err != nil {
		return nil, false, false, err
	}

	reloadTime := time.Unix(reloadUnixTS, 0).UTC().Format(time.RFC3339)

	if foundMod != nil && getMapString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
		samePath, err := samePath(getMapString(foundMod, "reload_path"), absEntrypoint)
		if err != nil {
			return nil, dirty, false, err
		}
		reloadFlag := foundMod["reload_enabled"]
		if samePath && reloadFlag == true {
			debugf(cmd.Root().Writer, args.Debug, "ReloadPath is up to date and ReloadEnabled, updating user and time")
			needsRestart = true
		} else {
			if samePath {
				debugf(cmd.Root().Writer, args.Debug, "ReloadPath is up to date, setting ReloadEnabled true")
				foundMod["reload_enabled"] = true
			} else {
				debugf(cmd.Root().Writer, args.Debug, "updating ReloadPath and ReloadEnabled")
				if cloudReload {
					delete(foundMod, "reload_path")
				} else {
					foundMod["reload_path"] = absEntrypoint
				}
				foundMod["reload_enabled"] = true
			}
		}
		dirty = true
		foundMod["reload_user"] = reloadUser
		foundMod["reload_time"] = reloadTime
		if annotation != "" {
			foundMod["reload_annotation"] = annotation
		}
	} else {
		dirty = true
		if foundMod == nil {
			debugf(cmd.Root().Writer, args.Debug, "module not found, inserting")
		} else {
			debugf(cmd.Root().Writer, args.Debug, "found local module, inserting registry module")
		}
		newMod := createNewModuleMap(manifest.ModuleID, absEntrypoint, reloadUser, reloadTime, annotation, cloudReload)
		modules = append(modules, newMod)
	}
	return modules, dirty, needsRestart, nil
}

func createNewModuleMap(moduleID, entryPoint, reloadUser, reloadTime, annotation string, cloudReload bool) ModuleMap {
	localName := localizeModuleID(moduleID)
	newMod := ModuleMap(map[string]any{
		"type":           string(rdkConfig.ModuleTypeRegistry),
		"module_id":      moduleID,
		"name":           localName,
		"reload_enabled": true,
		"version":        "latest-with-prerelease",
		"reload_user":    reloadUser,
		"reload_time":    reloadTime,
	})
	if annotation != "" {
		newMod["reload_annotation"] = annotation
	}
	if !cloudReload {
		newMod["reload_path"] = entryPoint
	}
	return newMod
}
