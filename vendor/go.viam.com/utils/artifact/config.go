package artifact

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/utils"
)

var (
	// DotDir is the location that artifact uses for itself.
	DotDir = ".artifact"

	// DefaultCachePath is the default relative location to store all cached
	// files (by hash).
	DefaultCachePath = "cache"
)

// DefaultSourcePullSizeLimitBytes is the limit where if a normal pull happens,
// a file larger than this size will not be pulled down from source
// unless pull with ignoring the limit is used.
const DefaultSourcePullSizeLimitBytes = 1 << 22

// A Config describes how artifact should function.
type Config struct {
	// Cache is where the hashed files should live. If unset, it defaults
	// to DefaultCachePath.
	Cache string

	// Root is where the files represented by the tree should live. These
	// reflect files in the cache but is based on the current tree. If unset,
	// it defaults to a data directory within DefaultCachePath.
	Root string

	// SourceStore is the configuration for where artifacts are remotely pushed
	// to and pulled from.
	SourceStore StoreConfig

	// SourcePullSizeLimit is the limit where if a normal pull happens,
	// a file larger than this size will not be pulled down from source
	// unless pull with ignoring the limit is used. If unset, DefaultSourcePullSizeLimitBytes
	// is used.
	SourcePullSizeLimit int

	// Ignore is a list of simple file names to ignore when scanning through
	// the root.
	Ignore []string

	ignoreSet utils.StringSet
	tree      TreeNodeTree
	configDir string
	commitFn  func() error
}

// Lookup looks an artifact up by its path and returns its
// associated node if it exists.
func (c *Config) Lookup(path string) (*TreeNode, error) {
	if path == "/" {
		return &TreeNode{internal: c.tree}, nil
	}
	parts := strings.Split(path, "/")
	node, ok := c.tree.lookup(parts)
	if !ok {
		return nil, NewArtifactNotFoundPathError(path)
	}
	return node, nil
}

// RemovePath removes nodes that fall into the given path.
func (c *Config) RemovePath(path string) {
	c.tree.removePath(strings.Split(path, "/"))
}

// StoreHash associates a path to the given node hash. The path is able to overwrite
// any existing one, so this method can be destructive if an external node were
// to replace an internal one.
func (c *Config) StoreHash(nodeHash string, nodeSize int, path []string) {
	c.tree.storeHash(nodeHash, nodeSize, path)
}

// UnmarshalJSON unmarshals the config from JSON data.
func (c *Config) UnmarshalJSON(data []byte) error {
	rawConfig := &struct {
		Cache               string           `json:"cache"`
		Root                string           `json:"root"`
		SourceStore         *json.RawMessage `json:"source_store"`
		SourcePullSizeLimit *int             `json:"source_pull_size_limit,omitempty"`
		Ignore              []string         `json:"ignore"`
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
	c.Ignore = rawConfig.Ignore
	if c.Ignore != nil {
		c.ignoreSet = utils.NewStringSet(c.Ignore...)
	}

	if rawConfig.SourceStore != nil {
		storeConfig, err := unmarshalJSONToStoreConfig([]byte(*rawConfig.SourceStore))
		if err != nil {
			return err
		}
		c.SourceStore = storeConfig
	}

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
	case StoreTypeFileSystem:
		var config FileSystemStoreConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
		return &config, nil
	case StoreTypeGoogleStorage:
		var config GoogleStorageStoreConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
		return &config, nil
	default:
		return nil, errors.Errorf("unknown store type %q", partialConfig.Type)
	}
}
