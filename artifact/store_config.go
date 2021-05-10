package artifact

// A StoreConfig describes how to configure and set up a Store.
type StoreConfig interface {
	// Type returns the type of the Store.
	Type() StoreType
}

// fileSystemStoreConfig is for configuring a local file system
// based Store.
type fileSystemStoreConfig struct {
	Path string `json:"path"`
}

// Type returns that this is a file system Store.
func (c *fileSystemStoreConfig) Type() StoreType {
	return StoreTypeFileSystem
}

// googleStorageStoreConfig is for configuring a Google based
// Store that has its credentials automatically looked up.
type googleStorageStoreConfig struct {
	Bucket string `json:"bucket"`
}

// Type returns that this is a Google storage Store.
func (c *googleStorageStoreConfig) Type() StoreType {
	return StoreTypeGoogleStorage
}
