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

// applyModulePackageVersions copies each registry module's package version onto the module so the
// module manager can record which version a module process reached ready with. A module is matched
// to the package whose data directory contains its resolved ExePath. A module outside the packages
// directory is a local path reload; one inside it with no matching package gets no version.
func applyModulePackageVersions(cfg *config.Config) {
	packagesDir := config.DefaultPackagesDir()
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if mod.Type != config.ModuleTypeRegistry {
			continue
		}
		mod.PackageVersion = ""
		if !inDir(mod.ExePath, packagesDir) {
			mod.PackageVersion = localReloadVersion
			continue
		}
		for j := range cfg.Packages {
			p := &cfg.Packages[j]
			if p.Type == config.PackageTypeModule && inDir(mod.ExePath, p.LocalDataDirectory(packagesDir)) {
				mod.PackageVersion = p.Version
				break
			}
		}
	}
}

// inDir reports whether path is inside dir. A plain prefix check would match sibling directories
// such as packages-local or a package whose version extends another's (1.2.3 and 1.2.30).
func inDir(path, dir string) bool {
	return strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)+string(filepath.Separator))
}

// ModuleVersions reports the running versions of the registry modules in the most recently applied
// config. It reads in-memory state every cloud config fetch and does not block on an in-progress
// reconfigure.
func (r *localRobot) ModuleVersions() []*apppb.ModuleVersionInfo {
	cfg, ok := r.mostRecentCfg.Load().(config.Config)
	if !ok || len(cfg.Modules) == 0 {
		return nil
	}
	return moduleVersionInfos(cfg.Modules, r.manager.moduleManager.Status(), time.Now())
}

// moduleVersionInfos builds one entry per reportable registry module from the configured modules
// and the current module statuses.
func moduleVersionInfos(mods []config.Module, statuses []modulestatus.Status, now time.Time) []*apppb.ModuleVersionInfo {
	byName := make(map[string]modulestatus.Status, len(statuses))
	for _, s := range statuses {
		byName[s.Name] = s
	}

	var infos []*apppb.ModuleVersionInfo
	for _, mod := range mods {
		if mod.Type != config.ModuleTypeRegistry || mod.ModuleID == "" {
			continue
		}
		info := &apppb.ModuleVersionInfo{ModuleId: mod.ModuleID}
		s, ok := byName[mod.Name]
		if ok && s.State == modulestatus.ModuleStateReady && s.Version != "" {
			info.RunningVersion = s.Version
			info.RunningFor = durationpb.New(now.Sub(s.LastUpdated))
		}
		infos = append(infos, info)
	}
	return infos
}
