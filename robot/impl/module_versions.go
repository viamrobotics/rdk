package robotimpl

import (
	"path/filepath"
	"strings"
	"time"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config"
	modulestatus "go.viam.com/rdk/module/status"
)

// localReloadVersion is reported for a registry module running from a `viam module reload` local
// path, which has no package version. Each reload is a separate config change (reload_time), so
// the version does not need to change per reload.
const localReloadVersion = "local"

// isLocalReload reports whether mod is a registry module that app sent as a local module because
// `viam module reload` set a local path for it.
func isLocalReload(mod config.Module) bool {
	return mod.Type == config.ModuleTypeLocal && mod.ModuleID != ""
}

// applyModulePackageVersions sets PackageVersion on each registry module so the module manager can
// record which version a module process reached ready with. A module is matched
// to the package whose data directory contains its resolved ExePath, and gets no version if none
// does. Local path reloads get localReloadVersion.
func applyModulePackageVersions(cfg *config.Config) {
	packagesDir := config.DefaultPackagesDir()
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if isLocalReload(*mod) {
			mod.PackageVersion = localReloadVersion
			continue
		}
		if mod.Type != config.ModuleTypeRegistry {
			continue
		}
		mod.PackageVersion = ""
		for j := range cfg.Packages {
			p := &cfg.Packages[j]
			if p.Type == config.PackageTypeModule && inDir(mod.ExePath, p.LocalDataDirectory(packagesDir)) {
				mod.PackageVersion = p.Version
				break
			}
		}
	}
}

// inDir reports whether path is inside dir. A plain prefix check would match a package whose
// version extends another's (1.2.3 and 1.2.30).
func inDir(path, dir string) bool {
	return strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)+string(filepath.Separator))
}

// FillAgentInfo adds the registry modules of the most recently applied config and whether the
// maintenance sensor is holding back a config change to info. It reads in-memory state on every
// cloud config fetch and does not block on an in-progress reconfigure.
func (r *localRobot) FillAgentInfo(info *apppb.AgentInfo) {
	info.ReconfigureBlocked = r.reconfigureBlocked.Load()
	cfg, ok := r.mostRecentCfg.Load().(config.Config)
	if !ok || len(cfg.Modules) == 0 {
		return
	}
	info.ModuleVersions = moduleVersionInfos(cfg.Modules, r.manager.moduleManager.Status(), time.Now())
}

// moduleVersionInfos builds one entry per registry module, including local path reloads, from the
// configured modules and the current module statuses.
func moduleVersionInfos(mods []config.Module, statuses []modulestatus.Status, now time.Time) []*apppb.ModuleVersionInfo {
	byName := make(map[string]modulestatus.Status, len(statuses))
	for _, s := range statuses {
		byName[s.Name] = s
	}

	var infos []*apppb.ModuleVersionInfo
	for _, mod := range mods {
		if (mod.Type != config.ModuleTypeRegistry || mod.ModuleID == "") && !isLocalReload(mod) {
			continue
		}
		info := &apppb.ModuleVersionInfo{ModuleId: mod.ModuleID, ModuleName: mod.Name}
		s, ok := byName[mod.Name]
		if ok {
			pbStatus := s.ToProto()
			info.State = pbStatus.State
			info.Error = pbStatus.Error
			info.ConsecutiveFailures = pbStatus.ConsecutiveFailures
			info.RestartCount = uint32(s.ReadyCount)
		}
		if ok && s.State == modulestatus.ModuleStateReady && s.Version != "" {
			info.RunningVersion = s.Version
			info.RunningFor = durationpb.New(now.Sub(s.LastUpdated))
		}
		infos = append(infos, info)
	}
	return infos
}
