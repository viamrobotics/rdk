package resource

import (
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

type fakeComponent struct {
	Name      Name
	DependsOn []Name
}

var apiA = APINamespace("namespace").WithType("atype").WithSubtype("aapi")

var commonCfg = []fakeComponent{
	{
		Name:      NewName(apiA, "A"),
		DependsOn: []Name{},
	},
	{
		Name:      NewName(apiA, "B"),
		DependsOn: []Name{NewName(apiA, "A")},
	},
	{
		Name:      NewName(apiA, "C"),
		DependsOn: []Name{NewName(apiA, "B")},
	},
	{
		Name: NewName(apiA, "D"),
		DependsOn: []Name{
			NewName(apiA, "B"),
			NewName(apiA, "E"),
		},
	},
	{
		Name:      NewName(apiA, "E"),
		DependsOn: []Name{NewName(apiA, "B")},
	},
	{
		Name: NewName(apiA, "F"),
		DependsOn: []Name{
			NewName(apiA, "A"),
			NewName(apiA, "C"),
			NewName(apiA, "E"),
		},
	},
	{
		Name:      NewName(apiA, "G"),
		DependsOn: []Name{},
	},
}

func TestResourceGraphConstruct(t *testing.T) {
	for idx, c := range []struct {
		conf []fakeComponent
		err  string
	}{
		{
			[]fakeComponent{
				{
					Name:      NewName(apiA, "A"),
					DependsOn: []Name{},
				},
				{
					Name:      NewName(apiA, "B"),
					DependsOn: []Name{NewName(apiA, "A")},
				},
				{
					Name:      NewName(apiA, "C"),
					DependsOn: []Name{NewName(apiA, "B")},
				},
				{
					Name:      NewName(apiA, "D"),
					DependsOn: []Name{NewName(apiA, "C")},
				},
				{
					Name:      NewName(apiA, "E"),
					DependsOn: []Name{},
				},
				{
					Name: NewName(apiA, "F"),
					DependsOn: []Name{
						NewName(apiA, "A"),
						NewName(apiA, "E"),
						NewName(apiA, "B"),
					},
				},
			},
			"",
		},
		{
			[]fakeComponent{
				{
					Name:      NewName(apiA, "A"),
					DependsOn: []Name{NewName(apiA, "B")},
				},
				{
					Name:      NewName(apiA, "B"),
					DependsOn: []Name{NewName(apiA, "A")},
				},
			},
			"circular dependency - \"A\" already depends on \"B\"",
		},
		{
			[]fakeComponent{
				{
					Name:      NewName(apiA, "A"),
					DependsOn: []Name{},
				},
				{
					Name:      NewName(apiA, "B"),
					DependsOn: []Name{NewName(apiA, "B")},
				},
			},
			"\"B\" cannot depend on itself",
		},
	} {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			g := NewGraph()
			test.That(t, g, test.ShouldNotBeNil)
			for i, component := range c.conf {
				test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
				for _, dep := range component.DependsOn {
					err := g.AddChild(component.Name, dep)
					if i > 0 && c.err != "" {
						test.That(t, err.Error(), test.ShouldContainSubstring, c.err)
					} else {
						test.That(t, err, test.ShouldBeNil)
					}
				}
			}
		})
	}
}

func TestResourceGraphGetParentsAndChildren(t *testing.T) {
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range commonCfg {
		test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := g.GetAllChildrenOf(NewName(apiA, "A"))
	test.That(t, len(out), test.ShouldEqual, 2)
	test.That(t, out, test.ShouldContain,
		NewName(apiA, "F"),
	)
	test.That(t, out, test.ShouldContain,
		NewName(apiA, "B"),
	)
	out = g.GetAllParentsOf(NewName(apiA, "F"))
	test.That(t, len(out), test.ShouldEqual, 3)
	test.That(t, out, test.ShouldContain,
		NewName(apiA, "C"),
	)
	test.That(t, out, test.ShouldContain,
		NewName(apiA, "A"),
	)
	out = g.GetAllChildrenOf(NewName(apiA, "C"))
	test.That(t, len(out), test.ShouldEqual, 1)
	test.That(t, out, test.ShouldContain,
		NewName(apiA, "F"),
	)
	g.RemoveChild(NewName(apiA, "F"),
		NewName(apiA, "C"))
	out = g.GetAllChildrenOf(NewName(apiA, "C"))
	test.That(t, len(out), test.ShouldEqual, 0)

	test.That(t, g.GetAllParentsOf(NewName(apiA, "Z")),
		test.ShouldBeEmpty)

	test.That(t, g.IsNodeDependingOn(NewName(apiA, "A"),
		NewName(apiA, "F")), test.ShouldBeTrue)
	test.That(t, g.IsNodeDependingOn(NewName(apiA, "F"),
		NewName(apiA, "A")), test.ShouldBeFalse)
	test.That(t, g.IsNodeDependingOn(NewName(apiA, "Z"),
		NewName(apiA, "F")), test.ShouldBeFalse)
	test.That(t, g.IsNodeDependingOn(NewName(apiA, "A"),
		NewName(apiA, "Z")), test.ShouldBeFalse)

	for _, p := range g.GetAllParentsOf(NewName(apiA, "F")) {
		g.removeChild(NewName(apiA, "F"), p)
	}
	g.remove(NewName(apiA, "F"))
	out = g.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:3]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "C"),
			NewName(apiA, "D"),
		}...))
	test.That(t, newResourceNameSet(out[3]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "E"),
	}...))
	test.That(t, newResourceNameSet(out[4]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "B"),
	}...))
	test.That(t, newResourceNameSet(out[5]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "A"),
	}...))
}

func TestResourceGraphSubGraph(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName(apiA, "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "B"),
			DependsOn: []Name{NewName(apiA, "A")},
		},
		{
			Name:      NewName(apiA, "C"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name: NewName(apiA, "D"),
			DependsOn: []Name{
				NewName(apiA, "B"),
				NewName(apiA, "C"),
			},
		},
		{
			Name:      NewName(apiA, "E"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name: NewName(apiA, "F"),
			DependsOn: []Name{
				NewName(apiA, "A"),
				NewName(apiA, "C"),
			},
		},
	}
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	sg, err := g.SubGraphFrom(NewName(apiA, "W"))
	test.That(t, sg, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		"cannot create sub-graph from non existing node \"W\" ")
	sg, err = g.SubGraphFrom(NewName(apiA, "C"))
	test.That(t, sg, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	out := sg.TopologicalSort()
	test.That(t, newResourceNameSet(out...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "D"),
		NewName(apiA, "F"),
		NewName(apiA, "C"),
	}...))
}

func TestResourceGraphDepTree(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName(apiA, "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "B"),
			DependsOn: []Name{NewName(apiA, "A")},
		},
		{
			Name:      NewName(apiA, "C"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name: NewName(apiA, "D"),
			DependsOn: []Name{
				NewName(apiA, "B"),
				NewName(apiA, "E"),
			},
		},
		{
			Name:      NewName(apiA, "E"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name:      NewName(apiA, "F"),
			DependsOn: []Name{NewName(apiA, "E")},
		},
	}
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	err := g.AddChild(NewName(apiA, "A"),
		NewName(apiA, "F"))
	test.That(t, err.Error(), test.ShouldEqual, "circular dependency - \"F\" already depends on \"A\"")
	test.That(t, g.AddChild(NewName(apiA, "D"),
		NewName(apiA, "F")), test.ShouldBeNil)
}

func TestResourceGraphTopologicalSort(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName(apiA, "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "B"),
			DependsOn: []Name{NewName(apiA, "A")},
		},
		{
			Name:      NewName(apiA, "C"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name:      NewName(apiA, "D"),
			DependsOn: []Name{NewName(apiA, "C")},
		},
		{
			Name:      NewName(apiA, "E"),
			DependsOn: []Name{NewName(apiA, "D")},
		},
		{
			Name: NewName(apiA, "F"),
			DependsOn: []Name{
				NewName(apiA, "A"),
				NewName(apiA, "E"),
				NewName(apiA, "B"),
			},
		},
	}
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := g.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName(apiA, "F"),
		NewName(apiA, "E"),
		NewName(apiA, "D"),
		NewName(apiA, "C"),
		NewName(apiA, "B"),
		NewName(apiA, "A"),
	})

	outLevels := g.TopologicalSortInLevels()
	test.That(t, outLevels, test.ShouldHaveLength, 6)
	test.That(t, outLevels, test.ShouldResemble, [][]Name{
		{
			NewName(apiA, "F"),
		},
		{
			NewName(apiA, "E"),
		},
		{
			NewName(apiA, "D"),
		},
		{
			NewName(apiA, "C"),
		},
		{
			NewName(apiA, "B"),
		},
		{
			NewName(apiA, "A"),
		},
	})

	gNode, ok := g.Node(NewName(apiA, "F"))
	test.That(t, ok, test.ShouldBeTrue)
	gNode.MarkForRemoval()
	test.That(t, g.RemoveMarked(), test.ShouldHaveLength, 1)
	out = g.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName(apiA, "E"),
		NewName(apiA, "D"),
		NewName(apiA, "C"),
		NewName(apiA, "B"),
		NewName(apiA, "A"),
	})
}

func TestResourceGraphMergeAdd(t *testing.T) {
	cfgA := []fakeComponent{
		{
			Name:      NewName(apiA, "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "B"),
			DependsOn: []Name{NewName(apiA, "A")},
		},
		{
			Name:      NewName(apiA, "C"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
	}
	cfgB := []fakeComponent{
		{
			Name:      NewName(apiA, "D"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "E"),
			DependsOn: []Name{NewName(apiA, "D")},
		},
		{
			Name:      NewName(apiA, "F"),
			DependsOn: []Name{NewName(apiA, "E")},
		},
	}
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		test.That(t, gA.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName(apiA, "C"),
		NewName(apiA, "B"),
		NewName(apiA, "A"),
	})
	gB := NewGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	for _, component := range cfgB {
		test.That(t, gB.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gB.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out = gB.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName(apiA, "F"),
		NewName(apiA, "E"),
		NewName(apiA, "D"),
	})
	test.That(t, gA.MergeAdd(gB), test.ShouldBeNil)
	test.That(t, gA.AddChild(NewName(apiA, "D"),
		NewName(apiA, "C")), test.ShouldBeNil)
	out = gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName(apiA, "F"),
		NewName(apiA, "E"),
		NewName(apiA, "D"),
		NewName(apiA, "C"),
		NewName(apiA, "B"),
		NewName(apiA, "A"),
	})
}

func TestResourceGraphMergeRemove(t *testing.T) {
	cfgA := []fakeComponent{
		{
			Name:      NewName(apiA, "1"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "2"),
			DependsOn: []Name{NewName(apiA, "1")},
		},
		{
			Name: NewName(apiA, "3"),
			DependsOn: []Name{
				NewName(apiA, "1"),
				NewName(apiA, "11"),
			},
		},
		{
			Name:      NewName(apiA, "4"),
			DependsOn: []Name{NewName(apiA, "2")},
		},
		{
			Name:      NewName(apiA, "5"),
			DependsOn: []Name{NewName(apiA, "4")},
		},
		{
			Name:      NewName(apiA, "6"),
			DependsOn: []Name{NewName(apiA, "4")},
		},
		{
			Name:      NewName(apiA, "7"),
			DependsOn: []Name{NewName(apiA, "4")},
		},
		{
			Name: NewName(apiA, "8"),
			DependsOn: []Name{
				NewName(apiA, "3"),
				NewName(apiA, "2"),
			},
		},
		{
			Name:      NewName(apiA, "9"),
			DependsOn: []Name{NewName(apiA, "8")},
		},
		{
			Name: NewName(apiA, "10"),
			DependsOn: []Name{
				NewName(apiA, "12"),
				NewName(apiA, "8"),
			},
		},
		{
			Name:      NewName(apiA, "11"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(apiA, "12"),
			DependsOn: []Name{NewName(apiA, "11")},
		},
		{
			Name:      NewName(apiA, "13"),
			DependsOn: []Name{NewName(apiA, "11")},
		},
		{
			Name:      NewName(apiA, "14"),
			DependsOn: []Name{NewName(apiA, "11")},
		},
	}
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		test.That(t, gA.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := gA.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:7]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "5"),
			NewName(apiA, "6"),
			NewName(apiA, "7"),
			NewName(apiA, "9"),
			NewName(apiA, "10"),
			NewName(apiA, "13"),
			NewName(apiA, "14"),
		}...))
	test.That(t, newResourceNameSet(out[7:10]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "4"),
			NewName(apiA, "8"),
			NewName(apiA, "12"),
		}...))
	test.That(t, newResourceNameSet(out[10:12]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "2"),
			NewName(apiA, "3"),
		}...))
	test.That(t, newResourceNameSet(out[12:14]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "1"),
			NewName(apiA, "11"),
		}...))
	removalList := []Name{
		NewName(apiA, "5"),
		NewName(apiA, "7"),
		NewName(apiA, "12"),
		NewName(apiA, "2"),
		NewName(apiA, "13"),
	}
	gB := NewGraph()
	for _, comp := range removalList {
		gC, err := gA.SubGraphFrom(comp)
		test.That(t, err, test.ShouldBeNil)
		gB.MergeAdd(gC)
	}
	gA.MarkForRemoval(gB)
	test.That(t, gA.RemoveMarked(), test.ShouldHaveLength, 10)

	out = gA.TopologicalSort()
	test.That(t, len(out), test.ShouldEqual, 4)
	test.That(t, newResourceNameSet(out[0:2]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "14"),
			NewName(apiA, "3"),
		}...))
	test.That(t, newResourceNameSet(out[2:4]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "11"),
		NewName(apiA, "1"),
	}...))
}

func newResourceNameSet(resourceNames ...Name) map[Name]*GraphNode {
	set := make(map[Name]*GraphNode, len(resourceNames))
	for _, val := range resourceNames {
		set[val] = &GraphNode{}
	}
	return set
}

func TestResourceGraphFindNodeByName(t *testing.T) {
	cfgA := []fakeComponent{
		{
			Name:      NewName(APINamespaceRDK.WithComponentType("aapi"), "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName(APINamespaceRDK.WithComponentType("aapi"), "B"),
			DependsOn: []Name{NewName(APINamespaceRDK.WithComponentType("aapi"), "A")},
		},
		{
			Name:      NewName(APINamespaceRDK.WithComponentType("aapi"), "C"),
			DependsOn: []Name{NewName(APINamespaceRDK.WithComponentType("aapi"), "B")},
		},
	}
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		test.That(t, gA.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	names := gA.findNodesByShortName("A")
	test.That(t, names, test.ShouldHaveLength, 1)
	names = gA.findNodesByShortName("B")
	test.That(t, names, test.ShouldHaveLength, 1)
	names = gA.findNodesByShortName("C")
	test.That(t, names, test.ShouldHaveLength, 1)
	names = gA.findNodesByShortName("D")
	test.That(t, names, test.ShouldHaveLength, 0)
}

var cfgA = []fakeComponent{
	{
		Name:      NewName(apiA, "A"),
		DependsOn: []Name{},
	},
	{
		Name:      NewName(apiA, "B"),
		DependsOn: []Name{NewName(apiA, "A")},
	},
	{
		Name:      NewName(apiA, "C"),
		DependsOn: []Name{NewName(apiA, "B")},
	},
	{
		Name: NewName(apiA, "D"),
		DependsOn: []Name{
			NewName(apiA, "A"),
			NewName(apiA, "B"),
		},
	},
	{
		Name:      NewName(apiA, "E"),
		DependsOn: []Name{NewName(apiA, "D")},
	},
	{
		Name:      NewName(apiA, "F"),
		DependsOn: []Name{NewName(apiA, "A")},
	},
	{
		Name:      NewName(apiA, "G"),
		DependsOn: []Name{NewName(apiA, "F")},
	},
	{
		Name:      NewName(apiA, "H"),
		DependsOn: []Name{NewName(apiA, "F")},
	},
}

func TestResourceGraphReplaceNodesParents(t *testing.T) {
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		test.That(t, gA.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := gA.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:4]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "H"),
			NewName(apiA, "E"),
			NewName(apiA, "C"),
		}...))
	test.That(t, newResourceNameSet(out[4:6]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "F"),
		NewName(apiA, "D"),
	}...))
	test.That(t, newResourceNameSet(out[6]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "B"),
	}...))
	test.That(t, newResourceNameSet(out[7]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "A"),
	}...))

	cfgB := []fakeComponent{
		{
			Name:      NewName(apiA, "F"),
			DependsOn: []Name{},
		},
		{
			Name: NewName(apiA, "B"),
			DependsOn: []Name{
				NewName(apiA, "A"),
				NewName(apiA, "F"),
			},
		},
		{
			Name:      NewName(apiA, "C"),
			DependsOn: []Name{NewName(apiA, "B")},
		},
		{
			Name:      NewName(apiA, "D"),
			DependsOn: []Name{NewName(apiA, "A")},
		},
		{
			Name:      NewName(apiA, "G"),
			DependsOn: []Name{NewName(apiA, "C")},
		},
		{
			Name:      NewName(apiA, "H"),
			DependsOn: []Name{NewName(apiA, "D")},
		},
	}
	gB := NewGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	for _, component := range cfgB {
		test.That(t, gB.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gB.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	for n := range gB.nodes {
		test.That(t, gA.ReplaceNodesParents(n, gB), test.ShouldBeNil)
	}
	out = gA.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:3]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "H"),
			NewName(apiA, "E"),
		}...))
	test.That(t, newResourceNameSet(out[3:5]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "C"),
		NewName(apiA, "D"),
	}...))
	test.That(t, newResourceNameSet(out[5]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "B"),
	}...))
	test.That(t, newResourceNameSet(out[6:8]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "A"),
		NewName(apiA, "F"),
	}...))

	cfgC := []fakeComponent{
		{
			Name:      NewName(apiA, "W"),
			DependsOn: []Name{},
		},
	}
	gC := NewGraph()
	test.That(t, gC, test.ShouldNotBeNil)
	for _, component := range cfgC {
		test.That(t, gC.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gC.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	test.That(t, gA.ReplaceNodesParents(NewName(apiA, "W"), gC), test.ShouldNotBeNil)
}

func TestResourceGraphCopyNodeAndChildren(t *testing.T) {
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		test.That(t, gA.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChild(component.Name, dep), test.ShouldBeNil)
		}
	}
	gB := NewGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	test.That(t, gB.CopyNodeAndChildren(NewName(apiA, "F"), gA), test.ShouldBeNil)
	out := gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:2]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "H"),
		}...))
	test.That(t, newResourceNameSet(out[2]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "F"),
	}...))

	test.That(t, gB.CopyNodeAndChildren(NewName(apiA, "D"), gA), test.ShouldBeNil)
	out = gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:3]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "H"),
			NewName(apiA, "E"),
		}...))
	test.That(t, newResourceNameSet(out[3:5]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "F"),
		NewName(apiA, "D"),
	}...))

	for n := range gA.nodes {
		test.That(t, gB.CopyNodeAndChildren(n, gA), test.ShouldBeNil)
	}
	out = gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:4]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName(apiA, "G"),
			NewName(apiA, "H"),
			NewName(apiA, "E"),
			NewName(apiA, "C"),
		}...))
	test.That(t, newResourceNameSet(out[4:6]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "F"),
		NewName(apiA, "D"),
	}...))
	test.That(t, newResourceNameSet(out[6]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "B"),
	}...))
	test.That(t, newResourceNameSet(out[7]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName(apiA, "A"),
	}...))
}

func TestResourceGraphRandomRemoval(t *testing.T) {
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range commonCfg {
		test.That(t, g.AddNode(component.Name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			err := g.AddChild(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
		}
	}

	name := NewName(apiA, "B")

	for _, c := range commonCfg {
		if c.Name == name {
			continue
		}
		for _, dep := range c.DependsOn {
			if dep == name {
				test.That(t, g.GetAllParentsOf(c.Name), test.ShouldContain, name)
				break
			}
		}
	}

	g.remove(name)
	test.That(t, g.GetAllParentsOf(name), test.ShouldBeEmpty)
	test.That(t, g.GetAllChildrenOf(name), test.ShouldBeEmpty)
}

func TestResourceGraphMarkForRemoval(t *testing.T) {
	g := NewGraph()

	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range commonCfg {
		res := &someResource{Named: component.Name.AsNamed()}
		test.That(t, g.AddNode(component.Name, NewConfiguredGraphNode(
			Config{},
			res,
			DefaultModelFamily.WithModel("foo"),
		)), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			err := g.AddChild(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
		}
	}

	name := NewName(apiA, "B")

	for _, c := range commonCfg {
		if c.Name == name {
			continue
		}
		for _, dep := range c.DependsOn {
			if dep == name {
				test.That(t, g.GetAllParentsOf(c.Name), test.ShouldContain, name)
				break
			}
		}
	}

	subG, err := g.SubGraphFrom(name)
	test.That(t, err, test.ShouldBeNil)
	g.MarkForRemoval(subG)

	toClose := g.RemoveMarked()
	test.That(t, toClose, test.ShouldHaveLength, 5)
	namesToClose := make(map[Name]struct{}, len(toClose))
	for _, res := range toClose {
		namesToClose[res.Name()] = struct{}{}
	}
	test.That(t, namesToClose, test.ShouldResemble, map[Name]struct{}{
		NewName(apiA, "B"): {},
		NewName(apiA, "F"): {},
		NewName(apiA, "D"): {},
		NewName(apiA, "C"): {},
		NewName(apiA, "E"): {},
	})

	test.That(t, g.GetAllParentsOf(name), test.ShouldBeEmpty)
	test.That(t, g.GetAllChildrenOf(name), test.ShouldBeEmpty)
}

func TestResourceGraphClock(t *testing.T) {
	g := NewGraph()

	test.That(t, g.CurrLogicalClockValue(), test.ShouldEqual, 0)

	name1 := NewName(apiA, "a")
	name2 := NewName(apiA, "b")
	node1 := &GraphNode{}
	test.That(t, g.AddNode(name1, node1), test.ShouldBeNil)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 0)
	node2 := &GraphNode{}
	test.That(t, g.AddNode(name1, node2), test.ShouldBeNil)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 0)
	test.That(t, node2.UpdatedAt(), test.ShouldEqual, 0)
	n, ok := g.Node(name1)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, n, test.ShouldNotEqual, node2) // see docs of AddNode/GraphNode.replace
	test.That(t, n, test.ShouldEqual, node1)    // see docs of AddNode/GraphNode.replace

	res1 := &someResource{Named: name1.AsNamed()}
	node1.SwapResource(res1, DefaultModelFamily.WithModel("foo"))
	test.That(t, g.CurrLogicalClockValue(), test.ShouldEqual, 1)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 1)
	test.That(t, node2.UpdatedAt(), test.ShouldEqual, 0)
	node1.SwapResource(res1, DefaultModelFamily.WithModel("foo"))
	test.That(t, g.CurrLogicalClockValue(), test.ShouldEqual, 2)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 2)

	node2 = &GraphNode{}
	test.That(t, g.AddNode(name2, node2), test.ShouldBeNil)
	node2.SwapResource(res1, DefaultModelFamily.WithModel("foo"))
	test.That(t, g.CurrLogicalClockValue(), test.ShouldEqual, 3)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 2)
	test.That(t, node2.UpdatedAt(), test.ShouldEqual, 3)
}

func TestResourceGraphLastReconfigured(t *testing.T) {
	g := NewGraph()

	name1 := NewName(apiA, "a")
	node1 := &GraphNode{}
	test.That(t, g.AddNode(name1, node1), test.ShouldBeNil)
	// Assert that uninitialized node has a nil lastReconfigured value.
	test.That(t, node1.LastReconfigured(), test.ShouldBeNil)

	res1 := &someResource{Named: name1.AsNamed()}
	node1.SwapResource(res1, DefaultModelFamily.WithModel("foo"))
	lr := node1.LastReconfigured()
	test.That(t, lr, test.ShouldNotBeNil)
	// Assert that after SwapResource, node's lastReconfigured time is between
	// 50ms ago and now.
	test.That(t, *lr, test.ShouldHappenBetween,
		time.Now().Add(-50*time.Millisecond), time.Now())

	// Mock a mutation with another SwapResource. Assert that lastReconfigured
	// value changed.
	node1.SwapResource(res1, DefaultModelFamily.WithModel("foo"))
	newLR := node1.LastReconfigured()
	test.That(t, newLR, test.ShouldNotBeNil)
	// Assert that after another SwapResource, node's lastReconfigured time is
	// after old lr value and between 50ms ago and now.
	test.That(t, *newLR, test.ShouldHappenAfter, *lr)
	test.That(t, *newLR, test.ShouldHappenBetween,
		time.Now().Add(-50*time.Millisecond), time.Now())
}

func TestResourceGraphResolveDependencies(t *testing.T) {
	logger := logging.NewTestLogger(t)
	g := NewGraph()
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)

	name1 := NewName(APINamespaceRDK.WithComponentType("aapi"), "a")
	node1 := NewUnconfiguredGraphNode(Config{}, []string{"a", "b", "c", "d"})
	test.That(t, g.AddNode(name1, node1), test.ShouldBeNil)
	err := g.ResolveDependencies(logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, name1.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, "depend on itself")
	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"a", "b", "c", "d"})
	node1.setUnresolvedDependencies("b", "c", "d")

	name2 := NewName(APINamespaceRDK.WithComponentType("aapi"), "b")
	node2 := NewUnconfiguredGraphNode(Config{}, []string{"z"})
	test.That(t, g.AddNode(name2, node2), test.ShouldBeNil)

	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)
	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"c", "d"})
	test.That(t, node2.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})

	name3 := NewName(APINamespaceRDK.WithComponentType("aapi"), "rem1:c")
	node3 := NewUnconfiguredGraphNode(Config{}, []string{"z"})
	name4 := NewName(APINamespaceRDK.WithComponentType("aapi"), "rem2:c")
	node4 := NewUnconfiguredGraphNode(Config{}, []string{"z"})
	test.That(t, g.AddNode(name3, node3), test.ShouldBeNil)
	test.That(t, g.AddNode(name4, node4), test.ShouldBeNil)

	err = g.ResolveDependencies(logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "conflicting names")
	test.That(t, err.Error(), test.ShouldContainSubstring, name1.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, name3.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, name4.String())

	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"c", "d"})
	test.That(t, node2.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})
	test.That(t, node3.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})
	test.That(t, node4.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})

	test.That(t, node1.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node2.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node3.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node4.hasUnresolvedDependencies(), test.ShouldBeTrue)

	g.remove(name3)
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)

	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"d"})
	test.That(t, node2.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})
	test.That(t, node4.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})

	test.That(t, node1.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node2.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node4.hasUnresolvedDependencies(), test.ShouldBeTrue)

	name5 := NewName(APINamespaceRDK.WithComponentType("aapi"), "z")
	node5 := NewUnconfiguredGraphNode(Config{}, []string{"rdk:component:foo/bar", "d"})
	test.That(t, g.AddNode(name5, node5), test.ShouldBeNil)
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)

	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"d"})
	test.That(t, node2.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node4.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node5.UnresolvedDependencies(), test.ShouldResemble, []string{"d"})

	test.That(t, node1.hasUnresolvedDependencies(), test.ShouldBeTrue)
	test.That(t, node2.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node4.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node5.hasUnresolvedDependencies(), test.ShouldBeTrue)

	name6 := NewName(APINamespaceRDK.WithComponentType("aapi"), "d")
	node6 := NewUnconfiguredGraphNode(Config{}, []string{})
	test.That(t, g.AddNode(name6, node6), test.ShouldBeNil)
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)
	test.That(t, node1.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node2.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node4.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node5.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, node6.UnresolvedDependencies(), test.ShouldBeEmpty)

	test.That(t, node1.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node2.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node4.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node5.hasUnresolvedDependencies(), test.ShouldBeFalse)
	test.That(t, node6.hasUnresolvedDependencies(), test.ShouldBeFalse)
}

type someResource struct {
	Named
	TriviallyReconfigurable
	TriviallyCloseable
}
