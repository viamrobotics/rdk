package artifact

import (
	"bytes"
	"encoding/hex"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/utils"
)

// the global cache singleton is used in all contexts where
// a config or cache are not explicitly created.
var (
	globalCacheSingleton   Cache
	globalCacheSingletonMu sync.Mutex
)

// GlobalCache returns a cache to be used globally based
// on an automatically discovered configuration file. If the
// configuration file cannot be found, a noop implementation
// of the cache is created. In the future this should possibly
// automatically create the configuration file.
func GlobalCache() (Cache, error) {
	globalCacheSingletonMu.Lock()
	defer globalCacheSingletonMu.Unlock()
	if globalCacheSingleton != nil {
		return globalCacheSingleton, nil
	}
	config, err := LoadConfig()
	if err != nil {
		if errors.Is(err, ErrConfigNotFound) {
			return &noopCache{}, nil
		}
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
	Ensure(path string, ignoreLimit bool) (string, error)

	// Remove removes the given path from the tree and root if present
	// but not from the cache. Use clean with cache true to clear out
	// the cache.
	Remove(path string) error

	// Clean makes sure that the user visible assets reflect 1:1
	// the versioned assets in the tree.
	Clean() error

	// WriteThroughUser makes sure that the user visible assets not
	// yet versioned are added to the tree and "written through" to
	// any stores responsible for caching.
	WriteThroughUser() error

	// Status inspects the root and returns a git like status of what is to
	// be added.
	Status() (*Status, error)

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
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
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
	if err := os.MkdirAll(artifactsRoot, 0o750); err != nil {
		return nil, err
	}
	fsStore, err := newFileSystemStore(&FileSystemStoreConfig{Path: cacheDir})
	if err != nil {
		return nil, err
	}
	cStore := cachedStore{
		cache:   fsStore,
		config:  config,
		rootDir: artifactsRoot,
	}
	if config.SourceStore == nil {
		cStore.source = fsStore
		return &cStore, nil
	}
	sourceStore, err := NewStore(config.SourceStore)
	if err != nil {
		return nil, err
	}
	cStore.source = sourceStore
	return &cStore, nil
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
	} else if s.source == nil || !IsNotFoundError(err) {
		return err
	}
	return s.source.Contains(hash)
}

func (s *cachedStore) Load(hash string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rc, err := s.cache.Load(hash); err == nil {
		return rc, nil
	} else if s.source == nil || !IsNotFoundError(err) {
		return nil, err
	}
	return s.source.Load(hash)
}

func (s *cachedStore) Store(hash string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if err := s.source.Store(hash, bytes.NewReader(data)); err != nil {
		return err
	}
	return s.cache.Store(hash, bytes.NewReader(data))
}

func (s *cachedStore) NewPath(to string) string {
	return filepath.Join(s.rootDir, to)
}

func (s *cachedStore) Ensure(path string, ignoreLimit bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, err := s.config.Lookup(path)
	if err != nil {
		return "", err
	}
	return s.ensureNode(node, s.NewPath(path), ignoreLimit)
}

func (s *cachedStore) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.HasPrefix(path, s.rootDir) {
		path = strings.TrimPrefix(path, s.rootDir+"/")
	}
	if path != "/" {
		path = strings.TrimPrefix(path, "/")
	}
	s.config.RemovePath(path)
	return s.config.commitFn()
}

func (s *cachedStore) Clean() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanTree(s.config.tree, s.rootDir)
}

// WriteThroughUser writes all objects in the user visible area to the
// through to the file system cache and the source cache.
func (s *cachedStore) WriteThroughUser() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeThroughUserTree(s.config.tree, nil, s.rootDir); err != nil {
		return err
	}
	return s.config.commitFn()
}

func (s *cachedStore) Status() (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status()
}

func (s *cachedStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if closer, ok := s.source.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s *cachedStore) store(hash string, data []byte) error {
	if err := s.source.Store(hash, bytes.NewReader(data)); err != nil {
		return err
	}
	return s.cache.Store(hash, bytes.NewReader(data))
}

// ensureNode verifies that all nodes living under a tree with respect to a given
// path are placed in the cache.
func (s *cachedStore) ensureNode(node *TreeNode, dstPath string, ignoreLimit bool) (string, error) {
	if node.IsInternal() {
		for name, child := range node.internal {
			if _, err := s.ensureNode(child, filepath.Join(dstPath, name), ignoreLimit); err != nil {
				return "", err
			}
		}
		return dstPath, nil
	}
	nodeHash := node.external.Hash

	if err := s.cache.Contains(nodeHash); err == nil {
		if err := emplaceFile(s.cache, nodeHash, dstPath); err != nil {
			return "", errors.Wrap(err, "error emplacing into file system cache")
		}
		return dstPath, nil
	} else if !IsNotFoundError(err) {
		return "", errors.Wrap(err, "error checking if hash is in file system cache")
	}

	if !ignoreLimit && s.config.SourcePullSizeLimit != 0 && node.external.Size > s.config.SourcePullSizeLimit {
		Logger.Infow("too large to load from source", "path", dstPath, "hash", nodeHash, "size", node.external.Size)
		return "", nil
	}

	Logger.Debugw("loading from source", "path", dstPath, "hash", nodeHash)
	rc, err := s.source.Load(nodeHash)
	if err != nil {
		return "", errors.Wrap(err, "error loading from source cache")
	}
	defer utils.UncheckedErrorFunc(rc.Close)
	if err := s.cache.Store(nodeHash, rc); err != nil {
		return "", errors.Wrap(err, "error storing into file system cache")
	}
	if err := emplaceFile(s.cache, nodeHash, dstPath); err != nil {
		return "", errors.Wrap(err, "error emplacing into file system cache")
	}
	return dstPath, nil
}

// cleanTree removes any files not referenced by the tree with respect to the given
// local path.
func (s *cachedStore) cleanTree(tree TreeNodeTree, localPath string) error {
	localFileInfos, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, info := range localFileInfos {
		name := info.Name()
		newLocalPath := filepath.Join(localPath, name)
		node, ok := tree[name]
		if ok {
			if node.IsInternal() {
				if err := s.cleanTree(node.internal, newLocalPath); err != nil {
					return err
				}
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

// nodeChangeType describes a change to a node.
type nodeChangeType int

// The set of types of changes.
const (
	nodeChangeTypeUnstored nodeChangeType = iota
	nodeChangeTypeModified
)

// walkUserTreeUncached examines the tree with respect to the given local path and visits all artifacts
// not in the tree.
func (s *cachedStore) walkUserTreeUncached(
	tree map[string]*TreeNode,
	treePath []string,
	localPath string,
	visit func(changeType nodeChangeType, nodeHash, localPath string, treePath []string, data []byte) error,
) error {
	localFileInfos, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, info := range localFileInfos {
		name := info.Name()
		if !info.IsDir() {
			if _, ok := s.config.ignoreSet[name]; ok {
				continue
			}
		}
		newTreePath := append([]string{}, treePath...)
		newTreePath = append(newTreePath, name)
		newLocalPath := filepath.Join(localPath, name)
		stat, err := os.Stat(newLocalPath)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			next, ok := tree[name]
			if !ok || !next.IsInternal() {
				next = &TreeNode{internal: TreeNodeTree{}}
				tree[name] = next
			}
			if err := s.walkUserTreeUncached(next.internal, newTreePath, newLocalPath, visit); err != nil {
				return err
			}
			continue
		}
		existingNode, hasExistingNode := tree[name]
		//nolint:gosec
		f, err := os.Open(newLocalPath)
		if err != nil {
			return errors.Wrap(err, "error opening file to write through cache")
		}
		if err := func() error {
			defer utils.UncheckedErrorFunc(f.Close)
			data, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			nodeHash, err := computeHash(data)
			if err != nil {
				return err
			}
			if hasExistingNode && !existingNode.IsInternal() && existingNode.external.Hash == nodeHash {
				return nil
			}
			var changeType nodeChangeType
			if hasExistingNode {
				changeType = nodeChangeTypeModified
			} else {
				changeType = nodeChangeTypeUnstored
			}
			return visit(changeType, nodeHash, newLocalPath, newTreePath, data)
		}(); err != nil {
			return err
		}
	}
	return nil
}

// writeThroughUserTree examines the tree with respect to the given local path and stores all artifacts
// not in the tree into the underlying store and updates the tree with the artifact location/hash.
func (s *cachedStore) writeThroughUserTree(tree map[string]*TreeNode, treePath []string, localPath string) error {
	return s.walkUserTreeUncached(
		tree,
		treePath,
		localPath,
		func(changeType nodeChangeType, nodeHash, localPath string, treePath []string, data []byte) error {
			Logger.Debugw("writing through", "path", localPath, "hash", nodeHash)
			if err := s.store(nodeHash, data); err != nil {
				return err
			}
			s.config.StoreHash(nodeHash, len(data), treePath)
			return nil
		})
}

// status examines the tree with respect to the given local path and reports all artifacts
// not in the tree.
func (s *cachedStore) status() (*Status, error) {
	var status Status
	if err := s.walkUserTreeUncached(
		s.config.tree,
		nil,
		s.rootDir,
		func(changeType nodeChangeType, nodeHash, localPath string, treePath []string, data []byte) error {
			switch changeType {
			case nodeChangeTypeUnstored:
				status.Unstored = append(status.Unstored, localPath)
			case nodeChangeTypeModified:
				status.Modified = append(status.Modified, localPath)
			}
			return nil
		}); err != nil {
		return nil, err
	}
	return &status, nil
}

func computeHash(data []byte) (string, error) {
	hasher := fnv.New128a()
	if _, err := hasher.Write(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type noopCache struct{}

func (cache *noopCache) Contains(hash string) error {
	return NewArtifactNotFoundHashError(hash)
}

func (cache *noopCache) Load(hash string) (io.ReadCloser, error) {
	return nil, NewArtifactNotFoundHashError(hash)
}

func (cache *noopCache) Store(hash string, r io.Reader) error {
	return nil
}

func (cache *noopCache) NewPath(to string) string {
	return ""
}

func (cache *noopCache) Ensure(path string, ignoreLimit bool) (string, error) {
	return "", NewArtifactNotFoundPathError(path)
}

func (cache *noopCache) Remove(path string) error {
	return nil
}

func (cache *noopCache) Clean() error {
	return nil
}

func (cache *noopCache) WriteThroughUser() error {
	return nil
}

func (cache *noopCache) Status() (*Status, error) {
	return &Status{}, nil
}

func (cache *noopCache) Close() error {
	return nil
}
