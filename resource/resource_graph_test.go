package resource

import (
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type fakeComponent struct {
	Name      Name
	DependsOn []Name
}

var commonCfg = []fakeComponent{
	{
		Name:      NewName("namespace", "atype", "asubtype", "A"),
		DependsOn: []Name{},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "B"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "C"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
	},
	{
		Name: NewName("namespace", "atype", "asubtype", "D"),
		DependsOn: []Name{
			NewName("namespace", "atype", "asubtype", "B"),
			NewName("namespace", "atype", "asubtype", "E"),
		},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "E"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
	},
	{
		Name: NewName("namespace", "atype", "asubtype", "F"),
		DependsOn: []Name{
			NewName("namespace", "atype", "asubtype", "A"),
			NewName("namespace", "atype", "asubtype", "C"),
			NewName("namespace", "atype", "asubtype", "E"),
		},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "G"),
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
					Name:      NewName("namespace", "atype", "asubtype", "A"),
					DependsOn: []Name{},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "B"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "C"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "D"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "C")},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "E"),
					DependsOn: []Name{},
				},
				{
					Name: NewName("namespace", "atype", "asubtype", "F"),
					DependsOn: []Name{
						NewName("namespace", "atype", "asubtype", "A"),
						NewName("namespace", "atype", "asubtype", "E"),
						NewName("namespace", "atype", "asubtype", "B"),
					},
				},
			},
			"",
		},
		{
			[]fakeComponent{
				{
					Name:      NewName("namespace", "atype", "asubtype", "A"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "B"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
				},
			},
			"circular dependency - \"A\" already depends on \"B\"",
		},
		{
			[]fakeComponent{
				{
					Name:      NewName("namespace", "atype", "asubtype", "A"),
					DependsOn: []Name{},
				},
				{
					Name:      NewName("namespace", "atype", "asubtype", "B"),
					DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
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
	out := g.GetAllChildrenOf(NewName("namespace", "atype", "asubtype", "A"))
	test.That(t, len(out), test.ShouldEqual, 2)
	test.That(t, out, test.ShouldContain,
		NewName("namespace", "atype", "asubtype", "F"),
	)
	test.That(t, out, test.ShouldContain,
		NewName("namespace", "atype", "asubtype", "B"),
	)
	out = g.GetAllParentsOf(NewName("namespace", "atype", "asubtype", "F"))
	test.That(t, len(out), test.ShouldEqual, 3)
	test.That(t, out, test.ShouldContain,
		NewName("namespace", "atype", "asubtype", "C"),
	)
	test.That(t, out, test.ShouldContain,
		NewName("namespace", "atype", "asubtype", "A"),
	)
	out = g.GetAllChildrenOf(NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, len(out), test.ShouldEqual, 1)
	test.That(t, out, test.ShouldContain,
		NewName("namespace", "atype", "asubtype", "F"),
	)
	g.RemoveChild(NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "C"))
	out = g.GetAllChildrenOf(NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, len(out), test.ShouldEqual, 0)

	test.That(t, g.GetAllParentsOf(NewName("namespace", "atype", "asubtype", "Z")),
		test.ShouldBeEmpty)

	test.That(t, g.IsNodeDependingOn(NewName("namespace", "atype", "asubtype", "A"),
		NewName("namespace", "atype", "asubtype", "F")), test.ShouldBeTrue)
	test.That(t, g.IsNodeDependingOn(NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "A")), test.ShouldBeFalse)
	test.That(t, g.IsNodeDependingOn(NewName("namespace", "atype", "asubtype", "Z"),
		NewName("namespace", "atype", "asubtype", "F")), test.ShouldBeFalse)
	test.That(t, g.IsNodeDependingOn(NewName("namespace", "atype", "asubtype", "A"),
		NewName("namespace", "atype", "asubtype", "Z")), test.ShouldBeFalse)

	for _, p := range g.GetAllParentsOf(NewName("namespace", "atype", "asubtype", "F")) {
		g.removeChild(NewName("namespace", "atype", "asubtype", "F"), p)
	}
	g.remove(NewName("namespace", "atype", "asubtype", "F"))
	out = g.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:3]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "C"),
			NewName("namespace", "atype", "asubtype", "D"),
		}...))
	test.That(t, newResourceNameSet(out[3]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "E"),
	}...))
	test.That(t, newResourceNameSet(out[4]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "B"),
	}...))
	test.That(t, newResourceNameSet(out[5]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "A"),
	}...))
}

func TestResourceGraphSubGraph(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "B"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "B"),
				NewName("namespace", "atype", "asubtype", "C"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "E"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "A"),
				NewName("namespace", "atype", "asubtype", "C"),
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
	sg, err := g.SubGraphFrom(NewName("namespace", "atype", "asubtype", "W"))
	test.That(t, sg, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		"cannot create sub-graph from non existing node \"W\" ")
	sg, err = g.SubGraphFrom(NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, sg, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	out := sg.TopologicalSort()
	test.That(t, newResourceNameSet(out...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "C"),
	}...))
}

func TestResourceGraphDepTree(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "B"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "B"),
				NewName("namespace", "atype", "asubtype", "E"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "E"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "E")},
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
	err := g.AddChild(NewName("namespace", "atype", "asubtype", "A"),
		NewName("namespace", "atype", "asubtype", "F"))
	test.That(t, err.Error(), test.ShouldEqual, "circular dependency - \"F\" already depends on \"A\"")
	test.That(t, g.AddChild(NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "F")), test.ShouldBeNil)
}

func TestResourceGraphTopologicalSort(t *testing.T) {
	cfg := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "B"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "C")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "E"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "D")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "A"),
				NewName("namespace", "atype", "asubtype", "E"),
				NewName("namespace", "atype", "asubtype", "B"),
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
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})

	outLevels := g.TopologicalSortInLevels()
	test.That(t, outLevels, test.ShouldHaveLength, 6)
	test.That(t, outLevels, test.ShouldResemble, [][]Name{
		{
			NewName("namespace", "atype", "asubtype", "F"),
		},
		{
			NewName("namespace", "atype", "asubtype", "E"),
		},
		{
			NewName("namespace", "atype", "asubtype", "D"),
		},
		{
			NewName("namespace", "atype", "asubtype", "C"),
		},
		{
			NewName("namespace", "atype", "asubtype", "B"),
		},
		{
			NewName("namespace", "atype", "asubtype", "A"),
		},
	})

	gNode, ok := g.Node(NewName("namespace", "atype", "asubtype", "F"))
	test.That(t, ok, test.ShouldBeTrue)
	gNode.MarkForRemoval()
	test.That(t, g.RemoveMarked(), test.ShouldHaveLength, 1)
	out = g.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
}

func TestResourceGraphMergeAdd(t *testing.T) {
	cfgA := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "B"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
	}
	cfgB := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "E"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "D")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "E")},
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
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
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
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
	})
	test.That(t, gA.MergeAdd(gB), test.ShouldBeNil)
	test.That(t, gA.AddChild(NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C")), test.ShouldBeNil)
	out = gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
}

func TestResourceGraphMergeRemove(t *testing.T) {
	cfgA := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "1"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "2"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "1")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "3"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "1"),
				NewName("namespace", "atype", "asubtype", "11"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "4"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "2")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "5"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "4")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "6"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "4")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "7"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "4")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "8"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "3"),
				NewName("namespace", "atype", "asubtype", "2"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "9"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "8")},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "10"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "12"),
				NewName("namespace", "atype", "asubtype", "8"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "11"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "12"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "11")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "13"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "11")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "14"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "11")},
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
			NewName("namespace", "atype", "asubtype", "5"),
			NewName("namespace", "atype", "asubtype", "6"),
			NewName("namespace", "atype", "asubtype", "7"),
			NewName("namespace", "atype", "asubtype", "9"),
			NewName("namespace", "atype", "asubtype", "10"),
			NewName("namespace", "atype", "asubtype", "13"),
			NewName("namespace", "atype", "asubtype", "14"),
		}...))
	test.That(t, newResourceNameSet(out[7:10]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "4"),
			NewName("namespace", "atype", "asubtype", "8"),
			NewName("namespace", "atype", "asubtype", "12"),
		}...))
	test.That(t, newResourceNameSet(out[10:12]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "2"),
			NewName("namespace", "atype", "asubtype", "3"),
		}...))
	test.That(t, newResourceNameSet(out[12:14]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "1"),
			NewName("namespace", "atype", "asubtype", "11"),
		}...))
	removalList := []Name{
		NewName("namespace", "atype", "asubtype", "5"),
		NewName("namespace", "atype", "asubtype", "7"),
		NewName("namespace", "atype", "asubtype", "12"),
		NewName("namespace", "atype", "asubtype", "2"),
		NewName("namespace", "atype", "asubtype", "13"),
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
			NewName("namespace", "atype", "asubtype", "14"),
			NewName("namespace", "atype", "asubtype", "3"),
		}...))
	test.That(t, newResourceNameSet(out[2:4]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "11"),
		NewName("namespace", "atype", "asubtype", "1"),
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
			Name:      NewName("namespace", ResourceTypeComponent, "asubtype", "A"),
			DependsOn: []Name{},
		},
		{
			Name:      NewName("namespace", ResourceTypeService, "asubtype", "B"),
			DependsOn: []Name{NewName("namespace", ResourceTypeComponent, "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", ResourceTypeComponent, "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", ResourceTypeService, "asubtype", "B")},
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
		Name:      NewName("namespace", "atype", "asubtype", "A"),
		DependsOn: []Name{},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "B"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "C"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
	},
	{
		Name: NewName("namespace", "atype", "asubtype", "D"),
		DependsOn: []Name{
			NewName("namespace", "atype", "asubtype", "A"),
			NewName("namespace", "atype", "asubtype", "B"),
		},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "E"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "D")},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "F"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "G"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "F")},
	},
	{
		Name:      NewName("namespace", "atype", "asubtype", "H"),
		DependsOn: []Name{NewName("namespace", "atype", "asubtype", "F")},
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
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "H"),
			NewName("namespace", "atype", "asubtype", "E"),
			NewName("namespace", "atype", "asubtype", "C"),
		}...))
	test.That(t, newResourceNameSet(out[4:6]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "D"),
	}...))
	test.That(t, newResourceNameSet(out[6]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "B"),
	}...))
	test.That(t, newResourceNameSet(out[7]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "A"),
	}...))

	cfgB := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{},
		},
		{
			Name: NewName("namespace", "atype", "asubtype", "B"),
			DependsOn: []Name{
				NewName("namespace", "atype", "asubtype", "A"),
				NewName("namespace", "atype", "asubtype", "F"),
			},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "C"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "B")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "G"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "C")},
		},
		{
			Name:      NewName("namespace", "atype", "asubtype", "H"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "D")},
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
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "H"),
			NewName("namespace", "atype", "asubtype", "E"),
		}...))
	test.That(t, newResourceNameSet(out[3:5]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "D"),
	}...))
	test.That(t, newResourceNameSet(out[5]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "B"),
	}...))
	test.That(t, newResourceNameSet(out[6:8]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "A"),
		NewName("namespace", "atype", "asubtype", "F"),
	}...))

	cfgC := []fakeComponent{
		{
			Name:      NewName("namespace", "atype", "asubtype", "W"),
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
	test.That(t, gA.ReplaceNodesParents(NewName("namespace", "atype", "asubtype", "W"), gC), test.ShouldNotBeNil)
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
	test.That(t, gB.CopyNodeAndChildren(NewName("namespace", "atype", "asubtype", "F"), gA), test.ShouldBeNil)
	out := gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:2]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "H"),
		}...))
	test.That(t, newResourceNameSet(out[2]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "F"),
	}...))

	test.That(t, gB.CopyNodeAndChildren(NewName("namespace", "atype", "asubtype", "D"), gA), test.ShouldBeNil)
	out = gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:3]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "H"),
			NewName("namespace", "atype", "asubtype", "E"),
		}...))
	test.That(t, newResourceNameSet(out[3:5]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "D"),
	}...))

	for n := range gA.nodes {
		test.That(t, gB.CopyNodeAndChildren(n, gA), test.ShouldBeNil)
	}
	out = gB.TopologicalSort()
	test.That(t, newResourceNameSet(out[0:4]...), test.ShouldResemble,
		newResourceNameSet([]Name{
			NewName("namespace", "atype", "asubtype", "G"),
			NewName("namespace", "atype", "asubtype", "H"),
			NewName("namespace", "atype", "asubtype", "E"),
			NewName("namespace", "atype", "asubtype", "C"),
		}...))
	test.That(t, newResourceNameSet(out[4:6]...), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "D"),
	}...))
	test.That(t, newResourceNameSet(out[6]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "B"),
	}...))
	test.That(t, newResourceNameSet(out[7]), test.ShouldResemble, newResourceNameSet([]Name{
		NewName("namespace", "atype", "asubtype", "A"),
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

	name := NewName("namespace", "atype", "asubtype", "B")

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
			NewDefaultModel("foo"),
		)), test.ShouldBeNil)
		for _, dep := range component.DependsOn {
			err := g.AddChild(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
		}
	}

	name := NewName("namespace", "atype", "asubtype", "B")

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
		NewName("namespace", "atype", "asubtype", "B"): {},
		NewName("namespace", "atype", "asubtype", "F"): {},
		NewName("namespace", "atype", "asubtype", "D"): {},
		NewName("namespace", "atype", "asubtype", "C"): {},
		NewName("namespace", "atype", "asubtype", "E"): {},
	})

	test.That(t, g.GetAllParentsOf(name), test.ShouldBeEmpty)
	test.That(t, g.GetAllChildrenOf(name), test.ShouldBeEmpty)
}

func TestResourceGraphClock(t *testing.T) {
	g := NewGraph()

	test.That(t, g.LastUpdatedTime(), test.ShouldEqual, 0)

	name1 := NewName("namespace", "atype", "asubtype", "a")
	name2 := NewName("namespace", "atype", "asubtype", "b")
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
	node1.SwapResource(res1, NewDefaultModel("foo"))
	test.That(t, g.LastUpdatedTime(), test.ShouldEqual, 1)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 1)
	test.That(t, node2.UpdatedAt(), test.ShouldEqual, 0)
	node1.SwapResource(res1, NewDefaultModel("foo"))
	test.That(t, g.LastUpdatedTime(), test.ShouldEqual, 2)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 2)

	node2 = &GraphNode{}
	test.That(t, g.AddNode(name2, node2), test.ShouldBeNil)
	node2.SwapResource(res1, NewDefaultModel("foo"))
	test.That(t, g.LastUpdatedTime(), test.ShouldEqual, 3)
	test.That(t, node1.UpdatedAt(), test.ShouldEqual, 2)
	test.That(t, node2.UpdatedAt(), test.ShouldEqual, 3)
}

func TestResourceGraphResolveDependencies(t *testing.T) {
	logger := golog.NewTestLogger(t)
	g := NewGraph()
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)

	name1 := NewName("namespace", ResourceTypeComponent, "asubtype", "a")
	node1 := NewUnconfiguredGraphNode(Config{}, []string{"a", "b", "c", "d"})
	test.That(t, g.AddNode(name1, node1), test.ShouldBeNil)
	err := g.ResolveDependencies(logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, name1.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, "depend on itself")
	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"a", "b", "c", "d"})
	node1.setUnresolvedDependencies("b", "c", "d")

	name2 := NewName("namespace", ResourceTypeService, "asubtype", "b")
	node2 := NewUnconfiguredGraphNode(Config{}, []string{"z"})
	test.That(t, g.AddNode(name2, node2), test.ShouldBeNil)

	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)
	test.That(t, node1.UnresolvedDependencies(), test.ShouldResemble, []string{"c", "d"})
	test.That(t, node2.UnresolvedDependencies(), test.ShouldResemble, []string{"z"})

	name3 := NewName("namespace", ResourceTypeComponent, "asubtype", "rem1:c")
	node3 := NewUnconfiguredGraphNode(Config{}, []string{"z"})
	name4 := NewName("namespace", ResourceTypeComponent, "asubtype", "rem2:c")
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

	name5 := NewName("namespace", ResourceTypeComponent, "asubtype", "z")
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

	name6 := NewName("namespace", ResourceTypeComponent, "asubtype", "d")
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
