package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
	apppb "go.viam.com/api/app/v1"
	goutils "go.viam.com/utils"

	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

const (
	reloadVersionPrefix       = "reload"
	reloadSourceVersionPrefix = "reload-source"
)

// configureModule is the configuration step of module reloading. Returns (updated robotPart, needsRestart, error).
// Mutates the passed part.RobotConfigJson.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func configureModule(
	ctx context.Context, cmd *cli.Command, vc *viamClient, manifest *ModuleManifest, part *apppb.RobotPart,
	local, cloudReload bool, reloadUser, annotation string, reloadUnixTS int64,
) (*apppb.RobotPart, bool, error) {
	if manifest == nil {
		return part, false, fmt.Errorf("reconfiguration requires valid manifest json passed to --%s", moduleFlagPath)
	}
	args, err := getGlobalArgs(cmd)
	if err != nil {
		return part, false, err
	}

	// needsRestart reflects the config actually written; on an OCC retry it is recomputed from the
	// re-fetched config, which is what we want the caller to act on.
	var needsRestart bool
	attempt := func(p *apppb.RobotPart) error {
		cfgJSON, err := partConfigJSON(p)
		if err != nil {
			return err
		}
		cfgJSON, needsRestart, err = mutateModuleConfig(
			cmd, cfgJSON, *manifest, local, cloudReload, reloadUser, annotation, reloadUnixTS)
		if err != nil {
			return err
		}
		writeBackConfig(p, cfgJSON)
		debugf(cmd.Root().Writer, args.Debug, "writing back config changes")
		return vc.updateRobotPart(ctx, p, cfgJSON, p.LastUpdated)
	}

	if err := vc.updateWithOCCRetry(ctx, part, attempt); err != nil {
		return part, false, errors.Wrap(err, "configureModule")
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

// mutateModuleConfig edits the modules list in the config JSON string to hot-reload with the
// given manifest. reloadUser is the email/identity of the user performing the reload.
// Returns (updatedConfigJSON, needsRestart, error).
// needsRestart means the module binary needs an explicit restart (the reload path/enabled were already correct).
func mutateModuleConfig(
	cmd *cli.Command,
	cfgJSON string,
	manifest ModuleManifest,
	local bool,
	cloudReload bool,
	reloadUser string,
	annotation string,
	reloadUnixTS int64,
) (string, bool, error) {
	var needsRestart bool

	modules := gjson.Get(cfgJSON, "modules").Array()
	foundIdx := -1
	for i, mod := range modules {
		if mod.Get("module_id").String() == manifest.ModuleID {
			foundIdx = i
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
			return "", false, err
		}
	} else {
		absEntrypoint = reloadingDestination(cmd, &manifest)
	}

	args, err := getGlobalArgs(cmd)
	if err != nil {
		return "", false, err
	}

	reloadTime := time.Unix(reloadUnixTS, 0).UTC().Format(time.RFC3339)

	editor := &jsonEditor{json: cfgJSON}
	if foundIdx >= 0 && modules[foundIdx].Get("type").String() == string(rdkConfig.ModuleTypeRegistry) {
		foundMod := modules[foundIdx]
		prefix := fmt.Sprintf("modules.%d.", foundIdx)
		same, err := samePath(foundMod.Get("reload_path").String(), absEntrypoint)
		if err != nil {
			return "", false, err
		}
		switch {
		case same && foundMod.Get("reload_enabled").Bool():
			debugf(cmd.Root().Writer, args.Debug, "ReloadPath is up to date and ReloadEnabled, updating user and time")
			needsRestart = true
		case same:
			debugf(cmd.Root().Writer, args.Debug, "ReloadPath is up to date, setting ReloadEnabled true")
			editor.set(prefix+"reload_enabled", true)
		default:
			debugf(cmd.Root().Writer, args.Debug, "updating ReloadPath and ReloadEnabled")
			if cloudReload {
				editor.del(prefix + "reload_path")
			} else {
				editor.set(prefix+"reload_path", absEntrypoint)
			}
			editor.set(prefix+"reload_enabled", true)
		}
		editor.set(prefix+"reload_user", reloadUser)
		editor.set(prefix+"reload_time", reloadTime)
		if annotation != "" {
			editor.set(prefix+"reload_annotation", annotation)
		}
	} else {
		if foundIdx < 0 {
			debugf(cmd.Root().Writer, args.Debug, "module not found, inserting")
		} else {
			debugf(cmd.Root().Writer, args.Debug, "found local module, inserting registry module")
		}
		editor.appendObject("modules", len(modules),
			newModulePairs(manifest.ModuleID, absEntrypoint, reloadUser, reloadTime, annotation, cloudReload))
	}
	if editor.err != nil {
		return "", false, editor.err
	}
	return editor.json, needsRestart, nil
}

// addResourceFromModule adds a resource to the components or services array if missing.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func (c *viamClient) addResourceFromModule(
	cmd *cli.Command, part *apppb.RobotPart, manifest *ModuleManifest, modelName, resourceName string,
) error {
	var resolvedName string
	attempt := func(p *apppb.RobotPart) error {
		cfgJSON, err := partConfigJSON(p)
		if err != nil {
			return err
		}
		cfgJSON, resolvedName, err = applyResourceToJSON(cfgJSON, manifest, modelName, resourceName)
		if err != nil {
			return err
		}
		writeBackConfig(p, cfgJSON)
		return c.updateRobotPart(context.Background(), p, cfgJSON, p.LastUpdated)
	}

	if err := c.updateWithOCCRetry(context.Background(), part, attempt); err != nil {
		return errors.Wrap(err, "addResourceFromModule")
	}
	infof(cmd.Root().Writer, "installing %s model with name %s on target machine", modelName, resolvedName)
	return nil
}

// jsonEditor threads an error through a sequence of sjson mutations so call sites don't have to
// error-check every Set/Delete. The first failure is retained and short-circuits the rest; check
// err once when done. (sjson only errors on malformed path syntax, never on the data itself.)
type jsonEditor struct {
	json string
	err  error
}

func (e *jsonEditor) set(path string, val any) {
	if e.err == nil {
		e.json, e.err = sjson.Set(e.json, path, val)
	}
}

func (e *jsonEditor) del(path string) {
	if e.err == nil {
		e.json, e.err = sjson.Delete(e.json, path)
	}
}

// kvPair is an ordered key/value used to build JSON objects with a deterministic field order.
type kvPair struct {
	key   string
	value any
}

// appendObject appends a new object with the given ordered fields to the array at arrayPath.
// idx must be the current length of that array. Setting fields one-by-one (rather than passing a
// map) guarantees a deterministic key order.
func (e *jsonEditor) appendObject(arrayPath string, idx int, pairs []kvPair) {
	e.set(arrayPath+".-1", map[string]any{})
	prefix := fmt.Sprintf("%s.%d.", arrayPath, idx)
	for _, p := range pairs {
		e.set(prefix+p.key, p.value)
	}
}

// partConfigJSON returns the robot config of a part as a raw JSON string. It prefers the
// RobotConfigJson field (which preserves field order); if that is empty it falls back to
// serializing the RobotConfig struct (e.g. for older responses).
func partConfigJSON(part *apppb.RobotPart) (string, error) {
	if j := part.GetRobotConfigJson(); j != "" {
		return j, nil
	}
	if part.RobotConfig == nil {
		return "{}", nil
	}
	raw, err := part.RobotConfig.MarshalJSON()
	if err != nil {
		return "", errors.Wrap(err, "marshaling robot config")
	}
	return string(raw), nil
}

// writeBackConfig stores an edited config JSON string back on part.RobotConfigJson; this is
// necessary so that changes aren't lost when we make multiple updateRobotPart calls. It also clears
// the legacy RobotConfig struct so no stale copy of the config is left behind: partConfigJSON and
// the app backend both read RobotConfigJson.
func writeBackConfig(part *apppb.RobotPart, cfgJSON string) {
	part.RobotConfig = nil
	part.RobotConfigJson = &cfgJSON
}

// applyResourceToJSON adds a resource to the components or services array if missing, mutating
// the config JSON string. Returns the updated config JSON and the resolved resource name.
// Returns an error if the modelName isn't in the manifest, or if the specified resourceName
// already exists.
func applyResourceToJSON(
	cfgJSON string, manifest *ModuleManifest, modelName, resourceName string,
) (string, string, error) {
	if manifest == nil {
		return "", "", errors.New("unable to add resource from config without a meta.json")
	}

	var modelAPI string
	for _, model := range manifest.Models {
		if model.Model == modelName {
			modelAPI = model.API
			break
		}
	}

	if len(modelAPI) == 0 {
		return "", "", errors.New("provided model name was not found in the meta.json")
	}

	APISlice := strings.Split(modelAPI, ":")
	if len(APISlice) != 3 {
		return "", "", errors.New("the provided model's API is malformed; unable to determine resource type")
	}

	resourceType := APISlice[1] + "s" // `components`, not `component`

	existing := gjson.Get(cfgJSON, resourceType).Array()
	nameExists := func(name string) bool {
		for _, r := range existing {
			if r.Get("name").String() == name {
				return true
			}
		}
		return false
	}

	// if the user provides a resource name but it's already in the config, alert the user and return
	if resourceName != "" {
		if nameExists(resourceName) {
			return "", "", errors.Errorf("resource name %s already exists in part config", resourceName)
		}
	} else { // if the user doesn't provide a resource name, find a valid one
		resourceSubtype := APISlice[2]
		for resourceNum := 1; ; resourceNum++ {
			name := fmt.Sprintf("%s-%d", resourceSubtype, resourceNum)
			if !nameExists(name) {
				resourceName = name
				break
			}
		}
	}

	editor := &jsonEditor{json: cfgJSON}
	editor.appendObject(resourceType, len(existing), []kvPair{
		{"name", resourceName},
		{"api", modelAPI},
		{"model", modelName},
	})
	if editor.err != nil {
		return "", "", editor.err
	}
	return editor.json, resourceName, nil
}

// addShellService adds a shell service to the services array if missing. Mutates part.RobotConfigJson.
// Returns (wasAdded, error) where wasAdded indicates if the shell service was newly added.
// Uses optimistic concurrency control with last_known_update to avoid overwriting concurrent changes.
func addShellService(
	ctx context.Context, cmd *cli.Command, vc *viamClient, logger logging.Logger, part *apppb.RobotPart, wait bool,
) (bool, error) {
	args, err := getGlobalArgs(cmd)
	if err != nil {
		return false, err
	}
	var added bool
	attempt := func(p *apppb.RobotPart) error {
		cfgJSON, err := partConfigJSON(p)
		if err != nil {
			return err
		}
		has, err := hasShellService(ctx, vc, cfgJSON)
		if err != nil {
			debugf(cmd.Root().Writer, args.Debug, "could not check for existing shell service: %v; installing one anyway", err)
		} else if has {
			debugf(cmd.Root().Writer, args.Debug, "shell service found on target machine, not installing")
			added = false
			return nil
		}
		cfgJSON, err = appendShellServiceToJSON(cfgJSON)
		if err != nil {
			return err
		}
		writeBackConfig(p, cfgJSON)
		if err := vc.updateRobotPart(ctx, p, cfgJSON, p.LastUpdated); err != nil {
			return err
		}
		added = true
		return nil
	}

	if err := vc.updateWithOCCRetry(ctx, part, attempt); err != nil {
		return false, errors.Wrap(err, "addShellService")
	}
	if !added {
		// shell service already present on the part (or a fragment); nothing to wait for.
		return false, nil
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

// appendShellServiceToJSON appends a shell service entry to the services array in the config JSON.
func appendShellServiceToJSON(cfgJSON string) (string, error) {
	editor := &jsonEditor{json: cfgJSON}
	editor.appendObject("services", len(gjson.Get(cfgJSON, "services").Array()), []kvPair{
		{"name", "shell"},
		{"api", "rdk:service:shell"},
	})
	return editor.json, editor.err
}

// hasShellService checks if a part or any of its fragments (recursively) already has a shell
// service configured. Field order is irrelevant for a presence check, so the config JSON is
// decoded to a map for the recursive walk.
func hasShellService(ctx context.Context, vc *viamClient, cfgJSON string) (bool, error) {
	var cfgMap map[string]any
	if err := json.Unmarshal([]byte(cfgJSON), &cfgMap); err != nil {
		return false, errors.Wrap(err, "parsing robot config")
	}
	return vc.findResourceInPartOrFragments(ctx, cfgMap, isShellService, map[string]bool{})
}

// isShellService matches a resource config entry for the shell service, tolerating
// both the modern `api` key and the legacy `type` key.
func isShellService(resource map[string]any) bool {
	return resource["type"] == "shell" || resource["api"] == "rdk:service:shell"
}

// findResourceInPartOrFragments returns true if predicate matches any resource in
// the part config or any of its fragments (recursively). Fragments are stored as
// refs ({"id": ...}) so their bodies must be fetched via GetFragment.
func (c *viamClient) findResourceInPartOrFragments(
	ctx context.Context,
	cfgMap map[string]any,
	predicate func(resource map[string]any) bool,
	visited map[string]bool,
) (bool, error) {
	for _, key := range []string{"services", "components"} {
		raw, ok := cfgMap[key]
		if !ok {
			continue
		}
		for _, r := range raw.([]any) {
			if predicate(r.(map[string]any)) {
				return true, nil
			}
		}
	}
	fragsRaw, ok := cfgMap["fragments"]
	if !ok {
		return false, nil
	}
	for _, f := range fragsRaw.([]any) {
		id, _ := f.(map[string]any)["id"].(string)
		if id == "" || visited[id] {
			continue
		}
		visited[id] = true
		resp, err := c.client.GetFragment(ctx, &apppb.GetFragmentRequest{Id: id})
		if err != nil {
			return false, err
		}
		if resp.Fragment == nil || resp.Fragment.Fragment == nil {
			continue
		}
		found, err := c.findResourceInPartOrFragments(ctx, resp.Fragment.Fragment.AsMap(), predicate, visited)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// updateWithOCCRetry runs attempt against part; if it fails, it re-fetches the part (picking up the
// latest last_known_update and config) and runs attempt once more. This handles the
// optimistic-concurrency-control conflict where a concurrent edit bumped last_known_update between
// our fetch and our write. attempt must be safe to run twice: it re-derives the config from the
// part it is given on each call.
func (c *viamClient) updateWithOCCRetry(
	ctx context.Context, part *apppb.RobotPart, attempt func(*apppb.RobotPart) error,
) error {
	if err := attempt(part); err == nil {
		return nil
	}
	partResp, refetchErr := c.getRobotPart(ctx, part.Id)
	if refetchErr != nil {
		return errors.Wrap(refetchErr, "update failed and could not re-fetch part config")
	}
	if err := attempt(partResp.Part); err != nil {
		return errors.Wrap(err, "retry also failed")
	}
	return nil
}

// newModulePairs returns the ordered fields for a new registry module entry configured for hot reload.
func newModulePairs(moduleID, entryPoint, reloadUser, reloadTime, annotation string, cloudReload bool) []kvPair {
	pairs := []kvPair{
		{"type", string(rdkConfig.ModuleTypeRegistry)},
		{"module_id", moduleID},
		{"name", localizeModuleID(moduleID)},
		{"reload_enabled", true},
		{"version", "latest-with-prerelease"},
		{"reload_user", reloadUser},
		{"reload_time", reloadTime},
	}
	if annotation != "" {
		pairs = append(pairs, kvPair{"reload_annotation", annotation})
	}
	if !cloudReload {
		pairs = append(pairs, kvPair{"reload_path", entryPoint})
	}
	return pairs
}

// localizeModuleID converts a module ID of the form "<namespace>:<moduleName>"
// (or "<orgID>:<moduleName>" if no namespace is set) into a name suitable for
// adding to a robot config, i.e. "<namespace>_<moduleName>" or "<orgID>_<moduleName>".
// TODO(APP-4019): remove this logic after registry modules can have local ExecPath.
func localizeModuleID(moduleID string) string {
	return strings.ReplaceAll(moduleID, ":", "_")
}

// reloadUser returns the identity string to stamp on reload configs.
// For token-based auth this is the user's email; for API keys it's the key ID.
func reloadUser(conf *Config) string {
	// let reload succeed even if the auth is nil, lets monitor to see if this case ever happens.
	if conf == nil || conf.Auth == nil {
		return ""
	}
	return conf.Auth.String()
}
