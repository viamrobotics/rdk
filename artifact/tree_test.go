package artifact

import (
	"encoding/json"
	"testing"

	"go.viam.com/test"
)

func TestTree(t *testing.T) {
	root := TreeNode{
		internal: TreeNodeTree{
			"one": &TreeNode{
				internal: TreeNodeTree{
					"one": &TreeNode{external: &TreeNodeExternal{Size: 232, Hash: "hash1"}},
					"two": &TreeNode{external: &TreeNodeExternal{Size: 1451, Hash: "hash2"}},
				},
			},
			"two": &TreeNode{external: &TreeNodeExternal{Size: 1293, Hash: "hash3"}},
			"three": &TreeNode{
				internal: TreeNodeTree{
					"one": &TreeNode{
						internal: TreeNodeTree{
							"two": &TreeNode{
								external: &TreeNodeExternal{Size: 55, Hash: "hash4"},
							},
						},
					},
				},
			},
		},
	}

	test.That(t, root.IsInternal(), test.ShouldBeTrue)

	_, found := root.internal.lookup(nil)
	test.That(t, found, test.ShouldBeFalse)

	_, found = root.internal.lookup([]string{"four", "five"})
	test.That(t, found, test.ShouldBeFalse)

	_, found = root.internal.lookup([]string{"one", "two", "three"})
	test.That(t, found, test.ShouldBeFalse)

	node, found := root.internal.lookup([]string{"one", "two"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Size, test.ShouldEqual, 1451)
	test.That(t, node.external.Hash, test.ShouldEqual, "hash2")

	node, found = root.internal.lookup([]string{"one"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeTrue)
	node, found = node.internal.lookup([]string{"one"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Size, test.ShouldEqual, 232)
	test.That(t, node.external.Hash, test.ShouldEqual, "hash1")

	node, found = root.internal.lookup([]string{"three", "one", "two"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Size, test.ShouldEqual, 55)
	test.That(t, node.external.Hash, test.ShouldEqual, "hash4")

	root.internal.storeHash("newhash", 5, []string{"four", "five", "six"})
	node, found = root.internal.lookup([]string{"four", "five", "six"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Size, test.ShouldEqual, 5)
	test.That(t, node.external.Hash, test.ShouldEqual, "newhash")

	root.internal.removePath([]string{"four", "five", "six"})
	_, found = root.internal.lookup([]string{"four", "five", "six"})
	test.That(t, found, test.ShouldBeFalse)

	root.internal.removePath([]string{"three", "one"})
	_, found = root.internal.lookup([]string{"three", "one", "two"})
	test.That(t, found, test.ShouldBeFalse)

	root.internal.storeHash("newhash2", 6, []string{"one"})
	node, found = root.internal.lookup([]string{"one"})
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, node.IsInternal(), test.ShouldBeFalse)
	test.That(t, node.external.Size, test.ShouldEqual, 6)
	test.That(t, node.external.Hash, test.ShouldEqual, "newhash2")

	_, found = root.internal.lookup([]string{"one", "two"})
	test.That(t, found, test.ShouldBeFalse)
}

func TestTreeJSONRoundTrip(t *testing.T) {
	tree := TreeNodeTree{
		"one": &TreeNode{
			internal: TreeNodeTree{
				"one": &TreeNode{external: &TreeNodeExternal{Size: 232, Hash: "hash1"}},
				"two": &TreeNode{external: &TreeNodeExternal{Size: 1451, Hash: "hash2"}},
			},
		},
		"two": &TreeNode{external: &TreeNodeExternal{Size: 1293, Hash: "hash3"}},
		"three": &TreeNode{
			internal: TreeNodeTree{
				"one": &TreeNode{
					internal: TreeNodeTree{
						"two": &TreeNode{
							external: &TreeNodeExternal{Size: 55, Hash: "hash4"},
						},
					},
				},
			},
		},
	}

	md, err := json.Marshal(tree)
	test.That(t, err, test.ShouldBeNil)

	var rt TreeNodeTree
	test.That(t, json.Unmarshal(md, &rt), test.ShouldBeNil)

	test.That(t, rt, test.ShouldResemble, tree)
}
