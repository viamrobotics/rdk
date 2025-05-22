package artifact

import "encoding/json"

// A TreeNode represents a node in an artifact tree. The tree
// is a hierarchy of artifacts that mimics a file system.
type TreeNode struct {
	internal TreeNodeTree
	external *TreeNodeExternal
}

// IsInternal returns if this node is an internal node.
func (tn *TreeNode) IsInternal() bool {
	return tn.internal != nil
}

// A TreeNodeExternal is an external node representing the location
// of an artifact identified by its content hash.
type TreeNodeExternal struct {
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

// A TreeNodeTree is an internal node with mappings to other
// nodes.
type TreeNodeTree map[string]*TreeNode

// lookup attempts to find a node by its path looking downwards.
func (tnt TreeNodeTree) lookup(path []string) (*TreeNode, bool) {
	if tnt == nil || len(path) == 0 {
		return nil, false
	}
	node, ok := tnt[path[0]]
	if !ok {
		return nil, false
	}
	if len(path) == 1 {
		return node, true
	}
	return node.internal.lookup(path[1:])
}

// storeHash stores a node hash by traversing down the tree to the destination
// creating nodes along the way.
func (tnt TreeNodeTree) storeHash(nodeHash string, nodeSize int, path []string) {
	if tnt == nil || len(path) == 0 {
		return
	}
	if len(path) == 1 {
		tnt[path[0]] = &TreeNode{external: &TreeNodeExternal{Hash: nodeHash, Size: nodeSize}}
		return
	}
	node, ok := tnt[path[0]]
	if !ok {
		next := TreeNodeTree{}
		tnt[path[0]] = &TreeNode{internal: next}
		next.storeHash(nodeHash, nodeSize, path[1:])
		return
	}
	if !node.IsInternal() {
		node = &TreeNode{internal: TreeNodeTree{}}
		tnt[path[0]] = node
	}
	node.internal.storeHash(nodeHash, nodeSize, path[1:])
}

// removePath removes nodes that fall into the given path.
func (tnt TreeNodeTree) removePath(path []string) {
	if tnt == nil || len(path) == 0 {
		return
	}
	if len(path) == 1 {
		delete(tnt, path[0])
		return
	}
	node, ok := tnt[path[0]]
	if !ok {
		return
	}
	node.internal.removePath(path[1:])
}

// UnmarshalJSON unmarshals JSON into a specific tree node
// that may be internal or external.
func (tn *TreeNode) UnmarshalJSON(data []byte) error {
	var temp struct {
		Size *int `json:"size"`
	}
	if err := json.Unmarshal(data, &temp); err == nil && temp.Size != nil {
		tn.external = &TreeNodeExternal{}
		return json.Unmarshal(data, tn.external)
	}
	return json.Unmarshal(data, &tn.internal)
}

// MarshalJSON marshals the node out into JSON.
func (tn *TreeNode) MarshalJSON() ([]byte, error) {
	if tn.IsInternal() {
		return json.Marshal(tn.internal)
	}
	return json.Marshal(tn.external)
}
