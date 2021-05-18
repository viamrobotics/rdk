package artifact

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/utils"
)

var (
	confRaw = `{
	"cache": "somedir",
	"root": "someotherdir",
	"source_store": {
		"type": "google_storage",
		"bucket": "mybucket"
	},
	"source_pull_size_limit": 5,
	"ignore": ["one", "two"]
}`

	treeRaw = `{
	"one": {
		"two": {
			"size": 5,
			"hash": "hash1"
		},
		"three": {
			"size": 6,
			"hash": "hash2"
		}
	},
	"two": {
		"three": {
			"four": {
				"size": 7,
				"hash": "hash3"
			}
		}
	}
}`
)

func TestLoadConfig(t *testing.T) {
	dir, undo := TestSetupGlobalCache(t)
	defer undo()

	_, err := LoadConfig()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	test.That(t, os.MkdirAll(filepath.Join(dir, DotDir), 0755), test.ShouldBeNil)
	confPath := filepath.Join(dir, DotDir, ConfigName)
	test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)

	found, err := searchConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, found, test.ShouldContainSubstring, confPath)

	config, err := LoadConfig()
	test.That(t, err, test.ShouldBeNil)
	commitFn := config.commitFn
	test.That(t, commitFn, test.ShouldNotBeNil)
	config.commitFn = nil
	test.That(t, config, test.ShouldResemble, &Config{
		Cache: "somedir",
		Root:  "someotherdir",
		SourceStore: &GoogleStorageStoreConfig{
			Bucket: "mybucket",
		},
		SourcePullSizeLimit: 5,
		Ignore:              []string{"one", "two"},
		ignoreSet:           utils.NewStringSet("one", "two"),
		configDir:           filepath.Dir(found),
		tree:                TreeNodeTree{},
	})

	test.That(t, os.Remove(confPath), test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(dir, "../../", DotDir), 0755), test.ShouldBeNil)
	confPath = filepath.Join(dir, "../../", DotDir, ConfigName)
	treePath := filepath.Join(dir, "../../", DotDir, TreeName)
	test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(treePath, []byte(treeRaw), 0644), test.ShouldBeNil)

	found, err = searchConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, found, test.ShouldContainSubstring, confPath)

	config, err = LoadConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	commitFn = config.commitFn
	test.That(t, commitFn, test.ShouldNotBeNil)
	config.commitFn = nil
	test.That(t, config, test.ShouldResemble, &Config{
		Cache: "somedir",
		Root:  "someotherdir",
		SourceStore: &GoogleStorageStoreConfig{
			Bucket: "mybucket",
		},
		SourcePullSizeLimit: 5,
		Ignore:              []string{"one", "two"},
		ignoreSet:           utils.NewStringSet("one", "two"),
		configDir:           filepath.Dir(found),
		tree: TreeNodeTree{
			"one": &TreeNode{
				internal: TreeNodeTree{
					"three": &TreeNode{
						external: &TreeNodeExternal{Hash: "hash2", Size: 6},
					},
					"two": &TreeNode{
						external: &TreeNodeExternal{Hash: "hash1", Size: 5},
					},
				},
			},
			"two": &TreeNode{
				internal: TreeNodeTree{
					"three": &TreeNode{
						internal: TreeNodeTree{
							"four": &TreeNode{
								external: &TreeNodeExternal{Hash: "hash3", Size: 7},
							},
						},
					},
				},
			},
		},
	})
}

func TestSearchConfig(t *testing.T) {
	dir, undo := TestSetupGlobalCache(t)
	defer undo()

	_, err := searchConfig()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	test.That(t, os.MkdirAll(filepath.Join(dir, DotDir), 0755), test.ShouldBeNil)
	confPath := filepath.Join(dir, DotDir, ConfigName)
	test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)

	found, err := searchConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, found, test.ShouldContainSubstring, confPath)

	test.That(t, os.Remove(confPath), test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(dir, "../../", DotDir), 0755), test.ShouldBeNil)
	confPath = filepath.Join(dir, "../../", DotDir, ConfigName)
	test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)

	found, err = searchConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, found, test.ShouldContainSubstring, confPath)
}

func TestLoadConfigFromFile(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := LoadConfigFromFile(filepath.Join(dir, "config.json"))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no such")
	})

	t.Run("without tree", func(t *testing.T) {
		dir := t.TempDir()
		confPath := filepath.Join(dir, "config.json")
		test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)
		config, err := LoadConfigFromFile(confPath)
		test.That(t, err, test.ShouldBeNil)
		commitFn := config.commitFn
		test.That(t, commitFn, test.ShouldNotBeNil)
		config.commitFn = nil
		test.That(t, config, test.ShouldResemble, &Config{
			Cache: "somedir",
			Root:  "someotherdir",
			SourceStore: &GoogleStorageStoreConfig{
				Bucket: "mybucket",
			},
			SourcePullSizeLimit: 5,
			Ignore:              []string{"one", "two"},
			ignoreSet:           utils.NewStringSet("one", "two"),
			configDir:           dir,
			tree:                TreeNodeTree{},
		})

		// modify tree, save, reload, verify
		config.StoreHash("hash1", 5, []string{"one", "two"})
		config.StoreHash("hash2", 6, []string{"one", "three"})
		config.StoreHash("hash3", 7, []string{"two", "three", "four"})
		test.That(t, commitFn(), test.ShouldBeNil)

		config, err = LoadConfigFromFile(confPath)
		test.That(t, err, test.ShouldBeNil)
		commitFn = config.commitFn
		test.That(t, commitFn, test.ShouldNotBeNil)
		config.commitFn = nil
		test.That(t, config, test.ShouldResemble, &Config{
			Cache: "somedir",
			Root:  "someotherdir",
			SourceStore: &GoogleStorageStoreConfig{
				Bucket: "mybucket",
			},
			SourcePullSizeLimit: 5,
			Ignore:              []string{"one", "two"},
			ignoreSet:           utils.NewStringSet("one", "two"),
			configDir:           dir,
			tree: TreeNodeTree{
				"one": &TreeNode{
					internal: TreeNodeTree{
						"three": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash2", Size: 6},
						},
						"two": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash1", Size: 5},
						},
					},
				},
				"two": &TreeNode{
					internal: TreeNodeTree{
						"three": &TreeNode{
							internal: TreeNodeTree{
								"four": &TreeNode{
									external: &TreeNodeExternal{Hash: "hash3", Size: 7},
								},
							},
						},
					},
				},
			},
		})
	})

	t.Run("with tree", func(t *testing.T) {
		dir := t.TempDir()
		confPath := filepath.Join(dir, "config.json")
		treePath := filepath.Join(dir, TreeName)
		test.That(t, ioutil.WriteFile(confPath, []byte(confRaw), 0644), test.ShouldBeNil)
		test.That(t, ioutil.WriteFile(treePath, []byte(treeRaw), 0644), test.ShouldBeNil)
		config, err := LoadConfigFromFile(confPath)
		test.That(t, err, test.ShouldBeNil)
		commitFn := config.commitFn
		test.That(t, commitFn, test.ShouldNotBeNil)
		config.commitFn = nil
		test.That(t, config, test.ShouldResemble, &Config{
			Cache: "somedir",
			Root:  "someotherdir",
			SourceStore: &GoogleStorageStoreConfig{
				Bucket: "mybucket",
			},
			SourcePullSizeLimit: 5,
			Ignore:              []string{"one", "two"},
			ignoreSet:           utils.NewStringSet("one", "two"),
			configDir:           dir,
			tree: TreeNodeTree{
				"one": &TreeNode{
					internal: TreeNodeTree{
						"three": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash2", Size: 6},
						},
						"two": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash1", Size: 5},
						},
					},
				},
				"two": &TreeNode{
					internal: TreeNodeTree{
						"three": &TreeNode{
							internal: TreeNodeTree{
								"four": &TreeNode{
									external: &TreeNodeExternal{Hash: "hash3", Size: 7},
								},
							},
						},
					},
				},
			},
		})

		// modify tree, save, reload, verify
		config.RemovePath("one/three")
		config.StoreHash("hash4", 8, []string{"new", "node"})
		test.That(t, commitFn(), test.ShouldBeNil)

		config, err = LoadConfigFromFile(confPath)
		test.That(t, err, test.ShouldBeNil)
		commitFn = config.commitFn
		test.That(t, commitFn, test.ShouldNotBeNil)
		config.commitFn = nil
		test.That(t, config, test.ShouldResemble, &Config{
			Cache: "somedir",
			Root:  "someotherdir",
			SourceStore: &GoogleStorageStoreConfig{
				Bucket: "mybucket",
			},
			SourcePullSizeLimit: 5,
			Ignore:              []string{"one", "two"},
			ignoreSet:           utils.NewStringSet("one", "two"),
			configDir:           dir,
			tree: TreeNodeTree{
				"one": &TreeNode{
					internal: TreeNodeTree{
						"two": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash1", Size: 5},
						},
					},
				},
				"two": &TreeNode{
					internal: TreeNodeTree{
						"three": &TreeNode{
							internal: TreeNodeTree{
								"four": &TreeNode{
									external: &TreeNodeExternal{Hash: "hash3", Size: 7},
								},
							},
						},
					},
				},
				"new": &TreeNode{
					internal: TreeNodeTree{
						"node": &TreeNode{
							external: &TreeNodeExternal{Hash: "hash4", Size: 8},
						},
					},
				},
			},
		})
	})
}
