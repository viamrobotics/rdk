package cli

import (
	"context"
	"encoding/json"
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

// mapToStructJson converts a map to a struct via json. The `mapstructure` package doesn't use json tags.
func mapToStructJson(raw map[string]interface{}, target interface{}) error {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

// structToMapJson does json ser/des to convert a struct to a map.
func structToMapJson(orig interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(orig)
	if err != nil {
		return nil, err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(encoded, &ret)
	return ret, err
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
		func(raw interface{}) (*rdkConfig.Module, error) {
			var mod rdkConfig.Module
			err := mapToStructJson(raw.(map[string]interface{}), &mod)
			if err != nil {
				return nil, err
			}
			return &mod, nil
		},
	)
	if err != nil {
		return err
	}

	localName := "hr_" + strings.ReplaceAll(manifest.ModuleID, ":", "_")
	var foundMod *rdkConfig.Module
	dirty := false
	for _, mod := range modules {
		if (mod.ModuleID == manifest.ModuleID) || (mod.Name == localName) {
			foundMod = mod
			break
		}
	}
	absEntrypoint, err := filepath.Abs(manifest.Entrypoint)
	if err != nil {
		return err
	}
	if foundMod == nil {
		logger.Debug("module not found, inserting")
		dirty = true
		newMod := &rdkConfig.Module{
			Name:    localName,
			ExePath: absEntrypoint,
			Type:    rdkConfig.ModuleTypeLocal,
			// todo: let user pass through LogLevel and Environment
		}
		modules = append(modules, newMod)
	} else {
		if same, err := samePath(foundMod.ExePath, manifest.Entrypoint); err != nil {
			logger.Debug("ExePath is right, doing nothing")
			return err
		} else if !same {
			dirty = true
			logger.Debug("replacing entrypoint")
			if foundMod.Type == rdkConfig.ModuleTypeRegistry {
				// warning: there's a chance of inserting a dupe name here in odd cases
				// todo: prompt user
				logger.Warnf("you're replacing a registry module. we're converting it to a local module")
				foundMod.Type = rdkConfig.ModuleTypeLocal
				foundMod.ModuleID = ""
			}
			foundMod.ExePath = absEntrypoint
		}
	}
	mapModules, err := mapOver(modules, func(mod *rdkConfig.Module) (interface{}, error) {
		ret, err := structToMapJson(mod)
		return ret, err
	})
	if err != nil {
		return err
	}
	partMap["modules"] = mapModules
	if dirty {
		logger.Debug("writing back config changes")
		err = vc.updateRobotPart(part.Part, partMap)
		if err != nil {
			return err
		}
	}
	return nil
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
		return err
	}
	req := apppb.UpdateRobotPartRequest{
		Id:          part.Id,
		Name:        part.Name,
		RobotConfig: confStruct,
	}
	_, err = c.client.UpdateRobotPart(c.c.Context, &req)
	return err
}
