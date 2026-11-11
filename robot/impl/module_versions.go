package robotimpl

import (
	"fmt"
	"strings"
	"time"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config"
	modulestatus "go.viam.com/rdk/module/status"
)

// localReloadVersion marks a registry module running from a `viam module reload` local path, which
// has no package version. It is reported as "reload-<unix time it became ready>" so each reload is
// distinct. Cloud reloads keep their unique "reload-..." package version.
const localReloadVersion = "reload"

// applyModulePackageVersions copies each registry module's package version onto the module so the
// module manager can record which version a module process reached ready with. A module is matched
// to the package whose data directory its resolved ExePath lives in.
func applyModulePackageVersions(cfg *config.Config) {
	packagesDir := config.DefaultPackagesDir()
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if mod.Type != config.ModuleTypeRegistry {
			continue
		}
		mod.PackageVersion = localReloadVersion
		for j := range cfg.Packages {
			p := &cfg.Packages[j]
			if p.Type == config.PackageTypeModule && strings.HasPrefix(mod.ExePath, p.LocalDataDirectory(packagesDir)) {
				mod.PackageVersion = p.Version
				break
			}
		}
	}
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
			if s.Version == localReloadVersion {
				info.RunningVersion = fmt.Sprintf("%s-%d", localReloadVersion, s.LastUpdated.Unix())
			}
			info.RunningFor = durationpb.New(now.Sub(s.LastUpdated))
		}
		infos = append(infos, info)
	}
	return infos
}
