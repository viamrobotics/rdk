package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"google.golang.org/protobuf/types/known/structpb"
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

// mapOver applies fn() to a slice of items and returns a slice of the return values.
func mapOver[T, U any](items []T, fn func(T) (U, error)) ([]U, error) {
	ret := make([]U, 0, len(items))
	for _, item := range items {
		newItem, err := fn(item)
		if err != nil {
			return nil, err
		}
		ret = append(ret, newItem)
	}
	return ret, nil
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
		if same, err := samePath(getString(foundMod, "executable_path"), absEntrypoint); err != nil {
			logger.Debug("ExePath is right, doing nothing")
			return nil, dirty, err
		} else if !same {
			dirty = true
			logger.Debug("replacing entrypoint")
			if getString(foundMod, "type") == string(rdkConfig.ModuleTypeRegistry) {
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

// samePath returns true if abs(path1) and abs(path2) are the same.
func samePath(path1, path2 string) (bool, error) {
	abs1, err := filepath.Abs(path1)
	if err != nil {
		return false, err
	}
	abs2, err := filepath.Abs(path2)
	if err != nil {
		return false, err
	}
	return abs1 == abs2, nil
}

// getString is a helper that returns map_[key] if it exists and is a string, otherwise empty string.
func getString(map_ map[string]interface{}, key string) string {
	if val, ok := map_[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return ""
		}
	}
	return ""
}

func (c *viamClient) getRobotPart(partId string) (*apppb.GetRobotPartResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	return c.client.GetRobotPart(c.c.Context, &apppb.GetRobotPartRequest{Id: partId})
}

func (c *viamClient) updateRobotPart(part *apppb.RobotPart, confMap map[string]interface{}) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	confStruct, err := structpb.NewStruct(confMap)
	if err != nil {
		return errors.Wrap(err, "in NewStruct")
	}
	req := apppb.UpdateRobotPartRequest{
		Id:          part.Id,
		Name:        part.Name,
		RobotConfig: confStruct,
	}
	_, err = c.client.UpdateRobotPart(c.c.Context, &req)
	return err
}
