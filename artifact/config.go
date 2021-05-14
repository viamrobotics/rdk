package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"

	"go.viam.com/core/utils"
)

// The default artifact file names.
const (
	DefaultConfigName = ".artifact.json"
	DefaultTreeName   = ".artifact.tree.json"
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
		return "", errors.Errorf("%q not found on system", DefaultConfigName)
	}
	return location, nil
}

// LoadConfigFromFile loads a Config from the given path. It also
// searches for an adjacent tree file (not required to exist).
func LoadConfigFromFile(path string) (*Config, error) {
	pathDir := filepath.Dir(path)
	configFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(configFile.Close)
	treePath := filepath.Join(pathDir, DefaultTreeName)
	treeFile, err := os.Open(treePath)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(treeFile.Close)

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
		defer utils.UncheckedErrorFunc(newTreeFile.Close)
		if err := newTreeFile.Truncate(0); err != nil {
			return err
		}
		enc := json.NewEncoder(newTreeFile)
		enc.SetIndent("", "  ")
		return enc.Encode(config.Tree)
	}
	return &config, nil
}

// A Config describes how artifact should function.
type Config struct {
	Cache               string       `json:"cache"`
	Root                string       `json:"root"`
	Store               StoreConfig  `json:"store"`
	Tree                TreeNodeTree `json:"tree"`
	SourcePullSizeLimit int          `json:"source_pull_size_limit"`
	configDir           string
	commitFn            func() error
}

// Lookup looks an artifact up by its path and returns its
// associated node if it exists.
func (c *Config) Lookup(path string) (*TreeNode, error) {
	if path == "/" {
		return &TreeNode{internal: c.Tree}, nil
	}
	parts := strings.Split(path, "/")
	node, ok := c.Tree.lookup(parts)
	if !ok {
		return nil, NewErrArtifactNotFoundPath(path)
	}
	return node, nil
}

// DefaultSourcePullSizeLimitBytes is the limit where if a normal pull happens,
// a file larger than this size will not be pulled down from source
// unless pull with ignoring the limit is used.
const DefaultSourcePullSizeLimitBytes = 1 << 22

// UnmarshalJSON unmarshals the config from JSON data.
func (c *Config) UnmarshalJSON(data []byte) error {
	rawConfig := &struct {
		Cache               string           `json:"cache"`
		Root                string           `json:"root"`
		Store               *json.RawMessage `json:"store"`
		Tree                TreeNodeTree     `json:"tree"`
		SourcePullSizeLimit *int             `json:"source_pull_size_limit,omitempty"`
	}{}
	if err := json.Unmarshal(data, rawConfig); err != nil {
		return err
	}
	c.Cache = rawConfig.Cache
	c.Root = rawConfig.Root
	if rawConfig.SourcePullSizeLimit == nil {
		c.SourcePullSizeLimit = DefaultSourcePullSizeLimitBytes
	} else {
		c.SourcePullSizeLimit = *rawConfig.SourcePullSizeLimit
	}

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
		return nil, errors.Errorf("unknown store type %q", partialConfig.Type)
	}
}

func (c *Config) storeHash(nodeHash string, nodeSize int, path []string) error {
	return c.Tree.storeHash(nodeHash, nodeSize, path)
}

// RemovePath removes nodes that fall into the given path.
func (c *Config) RemovePath(path string) {
	c.Tree.removePath(strings.Split(path, "/"))
}

// A TreeNode represents a node in an artifact tree. The tree
// is a hierarchy of artifacts that mimics a file system.
type TreeNode struct {
	internal TreeNodeTree
	external *TreeNodeExternal
}

// IsInternal returns if this node is an internal node.
func (tn *TreeNode) IsInternal() bool {
	return tn.internal != nil
}

// A TreeNodeExternal is an external node representing the location
// of an artifact identified by its content hash.
type TreeNodeExternal struct {
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

// UnmarshalJSON unmarshals JSON into a specific tree node
// that may be internal or external.
func (tn *TreeNode) UnmarshalJSON(data []byte) error {
	var temp struct {
		Size *int `json:"size"`
	}
	if err := json.Unmarshal(data, &temp); err == nil && temp.Size != nil {
		tn.external = &TreeNodeExternal{}
		return json.Unmarshal(data, tn.external)
	}
	return json.Unmarshal(data, &tn.internal)
}

// MarshalJSON marshals the node out into JSON.
func (tn *TreeNode) MarshalJSON() ([]byte, error) {
	if tn.IsInternal() {
		return json.Marshal(tn.internal)
	}
	return json.Marshal(tn.external)
}

// A TreeNodeTree is an internal node with mappings to other
// nodes.
type TreeNodeTree map[string]*TreeNode

// lookup attempts to find a node by its path looking downwards.
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
	return node.internal.lookup(path[1:])
}

// storeHash stores a node hash by traversing down the tree to the destination
// creating nodes along the way.
func (tnt TreeNodeTree) storeHash(nodeHash string, nodeSize int, path []string) error {
	if tnt == nil || len(path) == 0 {
		return nil
	}
	if len(path) == 1 {
		tnt[path[0]] = &TreeNode{external: &TreeNodeExternal{Hash: nodeHash, Size: nodeSize}}
		return nil
	}
	node, ok := tnt[path[0]]
	if !ok {
		next := TreeNodeTree{}
		tnt[path[0]] = &TreeNode{internal: next}
		return next.storeHash(nodeHash, nodeSize, path[1:])
	}
	if !node.IsInternal() {
		node = &TreeNode{internal: TreeNodeTree{}}
		tnt[path[0]] = node
	}
	return node.internal.storeHash(nodeHash, nodeSize, path[1:])
}

// removePath removes nodes that fall into the given path.
func (tnt TreeNodeTree) removePath(path []string) {
	if tnt == nil || len(path) == 0 {
		return
	}
	if len(path) == 1 {
		delete(tnt, path[0])
		return
	}
	node, ok := tnt[path[0]]
	if !ok {
		return
	}
	node.internal.removePath(path[1:])
}
