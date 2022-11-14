package octree

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
)

func TestNodeCreation(t *testing.T) {
	t.Run("Create empty leaf node", func(t *testing.T) {
		basicOct := newLeafNodeEmpty()

		test.That(t, basicOct.nodeType, test.ShouldResemble, LeafNodeEmpty)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{})
		test.That(t, basicOct.tree, test.ShouldBeNil)
	})

	t.Run("Create filled leaf node", func(t *testing.T) {
		p := r3.Vector{X: 0, Y: 0, Z: 0}
		d := pc.NewValueData(1.0)
		basicOct := newLeafNodeFilled(p, d)

		test.That(t, basicOct.nodeType, test.ShouldResemble, LeafNodeFilled)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{P: p, D: d})
		test.That(t, basicOct.tree, test.ShouldBeNil)
	})

	t.Run("internal node", func(t *testing.T) {
		var children []basicOctree
		basicOct := newInternalNode(children)

		test.That(t, basicOct.nodeType, test.ShouldResemble, InternalNode)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{})
		test.That(t, basicOct.tree, test.ShouldResemble, children)
	})
}

func TestSplitIntoOctants(t *testing.T) {
	// internal node
	// leaf child empty
	// leaf child filled
}

func TestCheckPointPlacement(t *testing.T) {
	// valid point
	// invalid point
}

// do we need to check if node is valid??????????
