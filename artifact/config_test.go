package artifact

import (
	"encoding/json"
	"testing"

	"github.com/go-errors/errors"
	"go.viam.com/test"

	"go.viam.com/core/utils"
)

func TestConfig(t *testing.T) {
	var emptyConfig Config
	node, err := emptyConfig.Lookup("/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)

	_, err = emptyConfig.Lookup("one")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	var errNotFound *errArtifactNotFound
	test.That(t, errors.As(err, &errNotFound), test.ShouldBeTrue)
	test.That(t, *errNotFound.path, test.ShouldEqual, "one")

	emptyConfig.RemovePath("/")
	emptyConfig.RemovePath("one")
	emptyConfig.StoreHash("hash1", 1, []string{"one", "two"})

	_, err = emptyConfig.Lookup("one/two")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	test.That(t, errors.As(err, &errNotFound), test.ShouldBeTrue)
	test.That(t, *errNotFound.path, test.ShouldEqual, "one/two")

	emptyConfig.tree = TreeNodeTree{}
	node, err = emptyConfig.Lookup("/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, node.IsInternal(), test.ShouldBeTrue)

	_, err = emptyConfig.Lookup("one")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	test.That(t, errors.As(err, &errNotFound), test.ShouldBeTrue)
	test.That(t, *errNotFound.path, test.ShouldEqual, "one")

	emptyConfig.RemovePath("/")
	emptyConfig.RemovePath("one")
	emptyConfig.StoreHash("hash1", 1, []string{"one", "two"})

	node, err = emptyConfig.Lookup("one/two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Hash, test.ShouldEqual, "hash1")
	test.That(t, node.external.Size, test.ShouldEqual, 1)
	emptyConfig.RemovePath("one")
	_, err = emptyConfig.Lookup("one")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	test.That(t, errors.As(err, &errNotFound), test.ShouldBeTrue)
	test.That(t, *errNotFound.path, test.ShouldEqual, "one")
}

func TestConfigUnmarshalJSON(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var config Config
		test.That(t, json.Unmarshal([]byte(`{}`), &config), test.ShouldBeNil)
		test.That(t, config, test.ShouldResemble, Config{
			SourcePullSizeLimit: DefaultSourcePullSizeLimitBytes,
		})
	})

	t.Run("error", func(t *testing.T) {
		var config Config
		err := json.Unmarshal([]byte(`{"source_store": {"type": "thing"}}`), &config)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown store")
		test.That(t, err.Error(), test.ShouldContainSubstring, "thing")
	})

	t.Run("full", func(t *testing.T) {
		var config Config
		err := json.Unmarshal([]byte(`{
			"cache": "somedir",
			"root": "someotherdir",
			"source_store": {
				"type": "google_storage",
				"bucket": "mybucket"
			},
			"source_pull_size_limit": 5,
			"ignore": ["one", "two"]
		}`), &config)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, config, test.ShouldResemble, Config{
			Cache: "somedir",
			Root:  "someotherdir",
			SourceStore: &GoogleStorageStoreConfig{
				Bucket: "mybucket",
			},
			SourcePullSizeLimit: 5,
			Ignore:              []string{"one", "two"},
			ignoreSet:           utils.NewStringSet("one", "two"),
		})
	})
}
