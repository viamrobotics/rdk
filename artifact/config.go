package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultConfigName = ".artifact.json"
	DefaultTreeName   = ".artifact.tree.json"
)

func LoadConfig() (*Config, error) {
	configPath, err := searchConfig()
	if err != nil {
		return nil, err
	}
	return LoadConfigFromFile(configPath)
}

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
		candidate := filepath.Join(path, DefaultConfigName)
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
		return "", fmt.Errorf("%q not found on system", DefaultConfigName)
	}
	return location, nil
}

func LoadConfigFromFile(path string) (*Config, error) {
	pathDir := filepath.Dir(path)
	configFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()
	treePath := filepath.Join(pathDir, DefaultTreeName)
	treeFile, err := os.Open(treePath)
	if err != nil {
		return nil, err
	}
	defer treeFile.Close()

	configDec := json.NewDecoder(configFile)
	treeDec := json.NewDecoder(treeFile)

	var config Config
	if err := configDec.Decode(&config); err != nil {
		return nil, err
	}
	var tree TreeNodeTree
	if err := treeDec.Decode(&tree); err != nil {
		return nil, err
	}
	config.Tree = tree
	config.configDir = pathDir
	config.commitFn = func() error {
		newTreeFile, err := os.OpenFile(treePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		defer newTreeFile.Close()
		if err := newTreeFile.Truncate(0); err != nil {
			return err
		}
		enc := json.NewEncoder(newTreeFile)
		enc.SetIndent("", "  ")
		return enc.Encode(config.Tree)
	}
	return &config, nil
}

type Config struct {
	Cache     string       `json:"cache"`
	Root      string       `json:"root"`
	Store     StoreConfig  `json:"store"`
	Tree      TreeNodeTree `json:"tree"`
	configDir string
	commitFn  func() error
}

func (c *Config) Lookup(path string) (*TreeNode, error) {
	if path == "/" {
		return &TreeNode{tree: c.Tree}, nil
	}
	parts := strings.Split(path, "/")
	node, ok := c.Tree.lookup(parts)
	if !ok {
		return nil, NewErrArtifactNotFoundPath(path)
	}
	return node, nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	rawConfig := &struct {
		Cache string           `json:"cache"`
		Root  string           `json:"root"`
		Store *json.RawMessage `json:"store"`
		Tree  TreeNodeTree     `json:"tree"`
	}{}
	if err := json.Unmarshal(data, rawConfig); err != nil {
		return err
	}
	c.Cache = rawConfig.Cache
	c.Root = rawConfig.Root

	if rawConfig.Store != nil {
		storeConfig, err := unmarshalJSONToStoreConfig([]byte(*rawConfig.Store))
		if err != nil {
			return err
		}
		c.Store = storeConfig
	}

	c.Tree = rawConfig.Tree

	return nil
}

func unmarshalJSONToStoreConfig(data []byte) (StoreConfig, error) {
	partialConfig := &struct {
		Type StoreType `json:"type"`
	}{}
	if err := json.Unmarshal(data, partialConfig); err != nil {
		return nil, err
	}
	switch partialConfig.Type {
	case StoreTypeGoogleStorage:
		var config googleStorageStoreConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
		return &config, nil
	default:
		return nil, fmt.Errorf("unknown store type %q", partialConfig.Type)
	}
}

func (c *Config) storeHash(nodeHash string, path []string) error {
	return c.Tree.storeHash(nodeHash, path)
}

type TreeNode struct {
	tree TreeNodeTree
	leaf *TreeNodeLeaf
}

func (tn *TreeNode) IsTree() bool {
	return tn.tree != nil
}

func (tn *TreeNode) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch v := tok.(type) {
	case string:
		tn.leaf = &TreeNodeLeaf{hash: v}
	case json.Delim:
		if v == '{' {
			var tree TreeNodeTree
			if err := json.Unmarshal(data, &tree); err != nil {
				return err
			}
			tn.tree = tree
		} else {
			return fmt.Errorf("invalid json delimiter %q", v)
		}
	default:
		return fmt.Errorf("invalid tree node type '%T'", tok)
	}
	return nil
}

func (tn *TreeNode) MarshalJSON() ([]byte, error) {
	if tn.IsTree() {
		return json.Marshal(tn.tree)
	}
	return json.Marshal(tn.leaf.hash)
}

type TreeNodeTree map[string]*TreeNode

func (tnt TreeNodeTree) lookup(path []string) (*TreeNode, bool) {
	if tnt == nil || len(path) == 0 {
		return nil, false
	}
	node, ok := tnt[path[0]]
	if !ok {
		return nil, false
	}
	if len(path) == 1 {
		return node, true
	}
	return node.tree.lookup(path[1:])
}

func (tnt TreeNodeTree) storeHash(nodeHash string, path []string) error {
	if tnt == nil || len(path) == 0 {
		return nil
	}
	if len(path) == 1 {
		tnt[path[0]] = &TreeNode{leaf: &TreeNodeLeaf{hash: nodeHash}}
		return nil
	}
	node, ok := tnt[path[0]]
	if !ok {
		next := TreeNodeTree{}
		tnt[path[0]] = &TreeNode{tree: next}
		return next.storeHash(nodeHash, path[1:])
	}
	if !node.IsTree() {
		node = &TreeNode{tree: TreeNodeTree{}}
		tnt[path[0]] = node
	}
	return node.tree.storeHash(nodeHash, path[1:])
}

type TreeNodeLeaf struct {
	hash string
}
