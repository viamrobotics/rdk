package artifact

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// DefaultCachePath is the default relative location to store all cached
// files (by hash).
const DefaultCachePath = ".artifact"

// the global cache singleton is used in all contexts where
// a config or cache are not explicitly created
var (
	globalCacheSingleton   Cache
	globalCacheSingletonMu sync.Mutex
)

// GlobalCache returns a cache to be used globally based
// on an automatically discovered configuration file. It errors
// if initial creation fails. It is not auto init()'d because
// right now a config file is required and would cause it to
// fail in some contexts if this were an independent tool in
// its own repository.
func GlobalCache() (Cache, error) {
	globalCacheSingletonMu.Lock()
	defer globalCacheSingletonMu.Unlock()
	if globalCacheSingleton != nil {
		return globalCacheSingleton, nil
	}
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	globalCacheSingleton, err = NewCache(config)
	if err != nil {
		return nil, err
	}
	return globalCacheSingleton, nil
}

// A Cache is similar to a store in functionality but
// it also has capabilities to refer to artifacts
// by their designated path name and not by a hash.
// In addition, it understands how to update the
// underlying stores that the cache is based off of as
// well as the user visible, versioned assets.
type Cache interface {
	Store

	// NewPath returns where a user visible asset would belong
	NewPath(to string) string

	// Ensure guarantees that a user visible path is populated with
	// the cached artifact. This can error if the path is unknown
	// or a failure happens retrieving it from underlying Store.
	Ensure(path string) (string, error)

	// Clean makes sure that the user visible assets reflect 1:1
	// the versioned assets in the tree.
	Clean() error

	// WriteThroughUser makes sure that the user visible assets not
	// yet versioned are added to the tree and "written through" to
	// any stores responsible for caching.
	WriteThroughUser() error

	// Close must be called in order to clean up any in use resources.
	Close() error
}

// NewCache returns a new cache based on the given config. It
// ensures that directory structures and stores are accessible
// in advance.
func NewCache(config *Config) (Cache, error) {
	var cacheDir string
	if config.Cache == "" {
		cacheDir = DefaultCachePath
	} else {
		cacheDir = config.Cache
	}
	if !filepath.IsAbs(cacheDir) && config.configDir != "" {
		cacheDir = filepath.Join(config.configDir, cacheDir)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	var artifactsRoot string
	if config.Root == "" {
		artifactsRoot = filepath.Join(cacheDir, "data")
	} else {
		if filepath.IsAbs(config.Root) {
			artifactsRoot = config.Root
		} else {
			artifactsRoot = filepath.Join(config.configDir, config.Root)
		}
	}
	if err := os.MkdirAll(artifactsRoot, 0755); err != nil {
		return nil, err
	}
	fsStore, err := newFileSystemStore(&fileSystemStoreConfig{Path: cacheDir})
	if err != nil {
		return nil, err
	}
	if config.Store == nil {
		return &cachedStore{cache: fsStore}, nil
	}
	sourceStore, err := NewStore(config.Store)
	if err != nil {
		return nil, err
	}
	return &cachedStore{
		cache:   fsStore,
		source:  sourceStore,
		config:  config,
		rootDir: artifactsRoot,
	}, nil
}

type cachedStore struct {
	mu      sync.Mutex
	cache   *fileSystemStore
	source  Store
	config  *Config
	rootDir string
}

func (s *cachedStore) Contains(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.cache.Contains(hash); err == nil {
		return nil
	} else if s.source == nil || !IsErrArtifactNotFound(err) {
		return err
	}
	return s.source.Contains(hash)
}

func (s *cachedStore) Load(hash string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rc, err := s.cache.Load(hash); err == nil {
		return rc, nil
	} else if s.source == nil || !IsErrArtifactNotFound(err) {
		return nil, err
	}
	return s.source.Load(hash)
}

func (s *cachedStore) Store(hash string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	if err := s.source.Store(hash, bytes.NewReader(data)); err != nil {
		return err
	}
	return s.cache.Store(hash, bytes.NewReader(data))
}

func (s *cachedStore) store(hash string, data []byte) error {
	if err := s.source.Store(hash, bytes.NewReader(data)); err != nil {
		return err
	}
	return s.cache.Store(hash, bytes.NewReader(data))
}

func (s *cachedStore) NewPath(to string) string {
	return filepath.Join(s.rootDir, to)
}

func (s *cachedStore) Ensure(path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, err := s.config.Lookup(path)
	if err != nil {
		return "", fmt.Errorf("path not in config: %q", path)
	}
	return s.ensureNode(node, s.NewPath(path))

}

func (s *cachedStore) ensureNode(node *TreeNode, dstPath string) (string, error) {
	if node.IsTree() {
		for name, child := range node.tree {
			if _, err := s.ensureNode(child, filepath.Join(dstPath, name)); err != nil {
				return "", err
			}
		}
		return dstPath, nil
	}
	nodeHash := node.leaf.hash

	if err := s.cache.Contains(nodeHash); err == nil {
		if err := s.cache.Emplace(nodeHash, dstPath); err != nil {
			return "", fmt.Errorf("error emplacing into file system cache: %w", err)
		}
		return dstPath, nil
	} else if !IsErrArtifactNotFound(err) {
		return "", fmt.Errorf("error checking if hash is in file system cache: %w", err)
	}

	Logger.Debugw("loading from source", "path", dstPath, "hash", nodeHash)
	rc, err := s.source.Load(nodeHash)
	if err != nil {
		return "", fmt.Errorf("error loading from source cache: %w", err)
	}
	defer rc.Close()
	if err := s.cache.Store(nodeHash, rc); err != nil {
		return "", fmt.Errorf("error storing into file system cache: %w", err)
	}
	if err := s.cache.Emplace(nodeHash, dstPath); err != nil {
		return "", fmt.Errorf("error emplacing into file system cache: %w", err)
	}
	return dstPath, nil
}

func (s *cachedStore) Clean() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanTree(s.config.Tree, s.rootDir)
}

func (s *cachedStore) cleanTree(tree map[string]*TreeNode, localPath string) error {
	localFileInfos, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, info := range localFileInfos {
		name := info.Name()
		newLocalPath := filepath.Join(localPath, name)
		node, ok := tree[name]
		if ok {
			if node.IsTree() {
				return s.cleanTree(node.tree, newLocalPath)
			}
			continue
		}
		Logger.Debugw("removing", "path", newLocalPath)
		if err := os.RemoveAll(newLocalPath); err != nil {
			return err
		}
	}
	return nil
}

func (s *cachedStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if closer, ok := s.source.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// WriteThroughUser writes all objects in the user visible area to the
// through to the file system cache and the source cache
func (s *cachedStore) WriteThroughUser() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeThroughUserTree(s.config.Tree, nil, s.rootDir); err != nil {
		return err
	}
	return s.config.commitFn()
}

func (s *cachedStore) writeThroughUserTree(tree map[string]*TreeNode, treePath []string, localPath string) error {
	localFileInfos, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, info := range localFileInfos {
		name := info.Name()
		newTreePath := append(treePath, name)
		newLocalPath := filepath.Join(localPath, name)
		stat, err := os.Stat(newLocalPath)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			next, ok := tree[name]
			if !ok || !next.IsTree() {
				next = &TreeNode{tree: TreeNodeTree{}}
				tree[name] = next
			}
			if err := s.writeThroughUserTree(next.tree, newTreePath, newLocalPath); err != nil {
				return err
			}
			continue
		}
		existingNode, hasExistingNode := tree[name]
		f, err := os.Open(newLocalPath)
		if err != nil {
			return fmt.Errorf("error opening file to write through cache: %w", err)
		}
		if err := func() error {
			defer f.Close()
			data, err := ioutil.ReadAll(f)
			if err != nil {
				return err
			}
			nodeHash, err := computeHash(data)
			if err != nil {
				return err
			}
			if hasExistingNode && !existingNode.IsTree() && existingNode.leaf.hash == nodeHash {
				return nil
			}
			Logger.Debugw("writing through", "path", newLocalPath, "hash", nodeHash)
			if err := s.store(nodeHash, data); err != nil {
				return err
			}
			return s.config.storeHash(nodeHash, newTreePath)
		}(); err != nil {
			return err
		}
	}
	return nil
}

func computeHash(data []byte) (string, error) {
	hasher := fnv.New128a()
	_, err := hasher.Write(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
