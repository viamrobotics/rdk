package robotimpl

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config"
	modulestatus "go.viam.com/rdk/module/status"
)

// localVersion is reported for local modules, which have no package version. This includes
// registry modules that app sends as local because `viam module reload` set a local path for them.
// Changes show up as config changes (such as a new reload_time) followed by a restart.
const localVersion = "local"

// applyModulePackageVersions sets PackageVersion on each module so the module manager can record
// which version a module process reached ready with. A registry module is matched to the package
// whose data directory contains its resolved ExePath, and gets no version if none does. Local
// modules get localVersion.
func applyModulePackageVersions(cfg *config.Config) {
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if mod.Type == config.ModuleTypeLocal {
			mod.PackageVersion = localVersion
			continue
		}
		if mod.Type != config.ModuleTypeRegistry {
			continue
		}
		mod.PackageVersion = ""
		if pkg, ok := modulePackage(*mod, cfg.Packages); ok {
			mod.PackageVersion = pkg.Version
		}
	}
}

// modulePackage returns the module package whose data directory is or contains mod's resolved
// ExePath. App names a registry module's package differently from the module, so the ExePath's
// ${packages.module.<name>} placeholder is the link. The separator check keeps a package whose
// version extends another's (1.2.3 and 1.2.30) from matching.
func modulePackage(mod config.Module, pkgs []config.PackageConfig) (config.PackageConfig, bool) {
	exePath := filepath.Clean(mod.ExePath)
	for _, pkg := range pkgs {
		if pkg.Type != config.PackageTypeModule {
			continue
		}
		dir := filepath.Clean(pkg.LocalDataDirectory(config.DefaultPackagesDir()))
		if exePath == dir || strings.HasPrefix(exePath, dir+string(filepath.Separator)) {
			return pkg, true
		}
	}
	return config.PackageConfig{}, false
}

// FillAgentInfo adds the modules the module manager is handling and whether the maintenance sensor
// is holding back a config change to info. Modules come from the module statuses rather than the
// last config received, so a blocked change doesn't add or drop modules before it's applied. It
// reads in-memory state on every cloud config fetch and does not block on an in-progress reconfigure.
func (r *localRobot) FillAgentInfo(info *apppb.AgentInfo) {
	info.ReconfigureBlocked = r.reconfigureBlocked.Load()
	statuses := r.manager.moduleManager.Status()
	if len(statuses) == 0 {
		return
	}
	cfg, _ := r.mostRecentCfg.Load().(config.Config)
	info.ModuleVersions = moduleVersionInfos(r.manager.moduleManager.Configs(), cfg.Modules, statuses, time.Now())
}

// moduleVersionInfos builds one entry per registry or local module with a status, sorted by name.
// running holds modules that have started and configured the last config received; a module that
// never started is only in configured. Local modules without a registry module ID are identified
// by name.
func moduleVersionInfos(
	running, configured []config.Module, statuses []modulestatus.Status, now time.Time,
) []*apppb.ModuleVersionInfo {
	modsByName := make(map[string]config.Module, len(running)+len(configured))
	for _, mod := range configured {
		modsByName[mod.Name] = mod
	}
	for _, mod := range running {
		modsByName[mod.Name] = mod
	}
	statuses = slices.Clone(statuses)
	slices.SortFunc(statuses, func(a, b modulestatus.Status) int { return strings.Compare(a.Name, b.Name) })

	var infos []*apppb.ModuleVersionInfo
	for _, s := range statuses {
		mod, ok := modsByName[s.Name]
		if !ok || (mod.Type != config.ModuleTypeRegistry && mod.Type != config.ModuleTypeLocal) {
			continue
		}
		pbStatus := s.ToProto()
		info := &apppb.ModuleVersionInfo{
			ModuleId:            mod.ModuleID,
			ModuleName:          mod.Name,
			State:               pbStatus.State,
			ConsecutiveFailures: pbStatus.ConsecutiveFailures,
			ReadyCount:          uint32(s.ReadyCount),
		}
		if s.State == modulestatus.ModuleStateReady && s.Version != "" {
			info.RunningVersion = s.Version
			info.RunningFor = durationpb.New(now.Sub(s.LastUpdated))
		}
		infos = append(infos, info)
	}
	return infos
}
