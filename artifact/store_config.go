package artifact

// A StoreConfig describes how to configure and set up a Store.
type StoreConfig interface {
	// Type returns the type of the Store.
	Type() StoreType
}

// FileSystemStoreConfig is for configuring a local file system
// based Store.
type FileSystemStoreConfig struct {
	Path string `json:"path"`
}

// Type returns that this is a file system Store.
func (c *FileSystemStoreConfig) Type() StoreType {
	return StoreTypeFileSystem
}

// GoogleStorageStoreConfig is for configuring a Google based
// Store that has its credentials automatically looked up.
type GoogleStorageStoreConfig struct {
	Bucket string `json:"bucket"`
}

// Type returns that this is a Google storage Store.
func (c *GoogleStorageStoreConfig) Type() StoreType {
	return StoreTypeGoogleStorage
}
