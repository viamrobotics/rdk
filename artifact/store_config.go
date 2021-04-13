package artifact

type StoreConfig interface {
	Type() StoreType
}

type fileSystemStoreConfig struct {
	Path string `json:"path"`
}

func (c *fileSystemStoreConfig) Type() StoreType {
	return StoreTypeFileSystem
}

type googleStorageStoreConfig struct {
	Bucket string `json:"bucket"`
}

func (c *googleStorageStoreConfig) Type() StoreType {
	return StoreTypeGoogleStorage
}
