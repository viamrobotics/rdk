package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"go.viam.com/utils"
)

// The artifact file names.
const (
	ConfigName = "config.json"
	TreeName   = "tree.json"
)

// LoadConfig attempts to automatically load an artifact config
// by searching for the default configuration file upwards in
// the file system.
func LoadConfig() (*Config, error) {
	configPath, err := searchConfig()
	if err != nil {
		return nil, err
	}
	return LoadConfigFromFile(configPath)
}

// ErrConfigNotFound is used when the configuration file cannot be found anywhere.
var ErrConfigNotFound = errors.Errorf("%q not found on system", ConfigName)

// searchConfig searches for the default configuration file by
// traversing the filesystem upwards from the current working
// directory.
func searchConfig() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	wdAbs, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}
	var helper func(path string) (string, error)
	helper = func(path string) (string, error) {
		candidate := filepath.Join(path, DotDir, ConfigName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		next := filepath.Join(path, "..")
		if next == path {
			return "", nil
		}
		return helper(next)
	}
	location, err := helper(wdAbs)
	if err != nil {
		return "", err
	}
	if location == "" {
		return "", ErrConfigNotFound
	}
	return location, nil
}

// LoadConfigFromFile loads a Config from the given path. It also
// searches for an adjacent tree file (not required to exist).
func LoadConfigFromFile(path string) (*Config, error) {
	pathDir := filepath.Dir(path)
	//nolint:gosec
	configFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(configFile.Close)

	configDec := json.NewDecoder(configFile)

	var config Config
	if err := configDec.Decode(&config); err != nil {
		return nil, err
	}

	treePath := filepath.Join(pathDir, TreeName)
	config.configDir = pathDir
	config.commitFn = func() error {
		//nolint:gosec
		newTreeFile, err := os.OpenFile(treePath, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return err
		}
		defer utils.UncheckedErrorFunc(newTreeFile.Close)
		if err := newTreeFile.Truncate(0); err != nil {
			return err
		}
		enc := json.NewEncoder(newTreeFile)
		enc.SetIndent("", "  ")
		return enc.Encode(config.tree)
	}

	//nolint:gosec
	treeFile, err := os.Open(treePath)
	if err == nil {
		defer utils.UncheckedErrorFunc(treeFile.Close)

		treeDec := json.NewDecoder(treeFile)

		var tree TreeNodeTree
		if err := treeDec.Decode(&tree); err != nil {
			return nil, err
		}
		config.tree = tree
	} else {
		config.tree = TreeNodeTree{}
	}

	return &config, nil
}
