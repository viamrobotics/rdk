package artifact

import (
	"encoding/json"
	"testing"

	"go.viam.com/test"
)

func TestFileSystemStoreConfig(t *testing.T) {
	var empty fileSystemStoreConfig
	test.That(t, empty.Type(), test.ShouldEqual, StoreTypeFileSystem)

	var fromJSON fileSystemStoreConfig
	err := json.Unmarshal([]byte(`{"path": 1}`), &fromJSON)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	err = json.Unmarshal([]byte(`{"path": "somePath"}`), &fromJSON)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fromJSON.Path, test.ShouldEqual, "somePath")
}

func TestGoogleStorageStoreConfig(t *testing.T) {
	var empty googleStorageStoreConfig
	test.That(t, empty.Type(), test.ShouldEqual, StoreTypeGoogleStorage)

	var fromJSON googleStorageStoreConfig
	err := json.Unmarshal([]byte(`{"bucket": 1}`), &fromJSON)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	err = json.Unmarshal([]byte(`{"bucket": "someBucket"}`), &fromJSON)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fromJSON.Bucket, test.ShouldEqual, "someBucket")
}
