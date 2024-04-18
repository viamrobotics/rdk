package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// ModuleMap is a type alias to indicate where a map represents a module config.
// We don't convert to rdkConfig.Module because it can get out of date with what's in the db.
// Using maps directly also saves a lot of high-maintenance ser/des work.
type ModuleMap map[string]interface{}

var reloadCommand = cli.Command{
	Name:  "reload",
	Usage: "run this module on a robot (only works on local robot for now)",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: moduleFlagPath, Value: "meta.json"},
		&cli.StringFlag{Name: generalFlagAliasRobotID},
		&cli.StringFlag{Name: configFlag},
	},
	Action: ModuleReloadAction,
}

func getPartId(ctx context.Context, configPath string) (string, error) {
	conf, err := rdkConfig.ReadLocalConfig(ctx, configPath, logging.Global())
	if err != nil {
		return "", err
	}
	return conf.Cloud.ID, nil
}

func ModuleReloadAction(cCtx *cli.Context) error {
	vc, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	if err := configureModule(cCtx, vc); err != nil {
		return err
	}
	return nil
}

// configureModule is the configuration step of module reloading.
func configureModule(cCtx *cli.Context, vc *viamClient) error {
	logger := logging.Global()
	robotId := cCtx.String(generalFlagAliasRobotID)
	configPath := cCtx.String(configFlag)
	// todo: switch to MutuallyExclusiveFlags when available
	if (len(robotId) == 0) && (len(configPath) == 0) {
		return fmt.Errorf("provide exactly one of --%s or --%s", generalFlagAliasRobotID, configFlag)
	}

	manifest, err := loadManifest(cCtx.String(moduleFlagPath))
	if err != nil {
		return err
	}
	var partId string
	if len(robotId) != 0 {
		return errors.New("robot-id not implemented yet")
	} else {
		partId, err = getPartId(cCtx.Context, configPath)
		if err != nil {
			return err
		}
	}

	part, err := vc.getRobotPart(partId)
	if err != nil {
		return err
	}
	partMap := part.Part.RobotConfig.AsMap()
	modules, err := mapOver(
		partMap["modules"].([]interface{}),
		func(raw interface{}) (ModuleMap, error) { return ModuleMap(raw.(map[string]interface{})), nil },
	)
	if err != nil {
		return err
	}

	modules, dirty, err := mutateModuleConfig(modules, manifest)
	if err != nil {
		return err
	}
	// note: converting to interface{} or else proto serializer will fail downstream in NewStruct.
	serializable, err := mapOver(modules, func(mod ModuleMap) (interface{}, error) {
		return map[string]interface{}(mod), nil
	})
	if err != nil {
		return err
	}
	partMap["modules"] = serializable
	if dirty {
		logger.Debug("writing back config changes")
		err = vc.updateRobotPart(part.Part, partMap)
		if err != nil {
			return err
		}
	}
	return nil
}

// mutateModuleConfig edits the modules list to hot-reload with the given manifest.
func mutateModuleConfig(modules []ModuleMap, manifest moduleManifest) ([]ModuleMap, bool, error) {
	var dirty bool
	logger := logging.Global()
	localName := "hr_" + strings.ReplaceAll(manifest.ModuleID, ":", "_")

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
				logger.Warnf("you're replacing a registry module. we're converting it to a local module")
				foundMod["type"] = string(rdkConfig.ModuleTypeLocal)
				foundMod["name"] = localName
				foundMod["module_id"] = ""
			}
			foundMod["executable_path"] = absEntrypoint
		}
	}
	return modules, dirty, nil
}
