package artifact

import (
	"encoding/json"
	"testing"

	"go.viam.com/test"
)

func TestFileSystemStoreConfig(t *testing.T) {
	var empty FileSystemStoreConfig
	test.That(t, empty.Type(), test.ShouldEqual, StoreTypeFileSystem)

	var fromJSON FileSystemStoreConfig
	err := json.Unmarshal([]byte(`{"path": 1}`), &fromJSON)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	err = json.Unmarshal([]byte(`{"path": "somePath"}`), &fromJSON)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fromJSON.Path, test.ShouldEqual, "somePath")
}

func TestGoogleStorageStoreConfig(t *testing.T) {
	var empty GoogleStorageStoreConfig
	test.That(t, empty.Type(), test.ShouldEqual, StoreTypeGoogleStorage)

	var fromJSON GoogleStorageStoreConfig
	err := json.Unmarshal([]byte(`{"bucket": 1}`), &fromJSON)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	err = json.Unmarshal([]byte(`{"bucket": "someBucket"}`), &fromJSON)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fromJSON.Bucket, test.ShouldEqual, "someBucket")
}
