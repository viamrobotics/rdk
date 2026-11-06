package robotimpl

import (
	"strings"
	"time"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config"
	modulestatus "go.viam.com/rdk/module/status"
)

// reloadVersionPrefix matches the package versions the CLI uploads for `viam module reload`
// (see cli.IsReloadVersion). Those are dev builds, not registry releases, so they are not reported.
const reloadVersionPrefix = "reload"

// applyModulePackageVersions copies each registry module's package version onto the module so the
// module manager can record which version a module process reached ready with.
func applyModulePackageVersions(cfg *config.Config) {
	versions := make(map[string]string, len(cfg.Packages))
	for _, p := range cfg.Packages {
		if p.Type == config.PackageTypeModule {
			versions[p.Name] = p.Version
		}
	}
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if mod.Type == config.ModuleTypeRegistry {
			mod.PackageVersion = versions[mod.Name]
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
		if mod.Type != config.ModuleTypeRegistry || mod.ModuleID == "" || strings.HasPrefix(mod.PackageVersion, reloadVersionPrefix) {
			continue
		}
		info := &apppb.ModuleVersionInfo{ModuleId: mod.ModuleID}
		s, ok := byName[mod.Name]
		if ok && s.State == modulestatus.ModuleStateReady && s.Version != "" && !strings.HasPrefix(s.Version, reloadVersionPrefix) {
			info.RunningVersion = s.Version
			info.RunningFor = durationpb.New(now.Sub(s.LastUpdated))
		}
		infos = append(infos, info)
	}
	return infos
}
