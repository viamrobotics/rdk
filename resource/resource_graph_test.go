package resource

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

type fakeComponent struct {
	Name      Name
	DependsOn []Name
}

func TestGraphConstruct(t *testing.T) {
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
				g.AddNode(component.Name, struct{}{})
				for _, dep := range component.DependsOn {
					err := g.AddChildren(component.Name, dep)
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

func TestGetParentsAndChildren(t *testing.T) {
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
			Name: NewName("namespace", "atype", "asubtype", "F"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A"),
				NewName("namespace", "atype", "asubtype", "C")},
		},
	}
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		g.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChildren(component.Name, dep), test.ShouldBeNil)
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
	test.That(t, len(out), test.ShouldEqual, 2)
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
	g.RemoveChildren(NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "C"))
	out = g.GetAllChildrenOf(NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, len(out), test.ShouldEqual, 0)
}

func TestGraphSubGraph(t *testing.T) {
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
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "A"),
				NewName("namespace", "atype", "asubtype", "C")},
		},
	}
	g := NewGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		g.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	sg, err := g.SubGraphFrom(NewName("namespace", "atype", "asubtype", "W"))
	test.That(t, sg, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		"cannot create subgraph from non existing node \"W\" ")
	sg, err = g.SubGraphFrom(NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, sg, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	out := sg.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "C"),
	})
}

func TestGraphDepTree(t *testing.T) {
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
		g.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	err := g.AddChildren(NewName("namespace", "atype", "asubtype", "A"),
		NewName("namespace", "atype", "asubtype", "F"))
	test.That(t, err.Error(), test.ShouldEqual, "circular dependency - \"F\" already depends on \"A\"")
	test.That(t, g.AddChildren(NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "F")), test.ShouldBeNil)
}

func TestGraphTopologicalSort(t *testing.T) {
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
		g.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, g.AddChildren(component.Name, dep), test.ShouldBeNil)
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
	g.Remove(NewName("namespace", "atype", "asubtype", "F"))
	out = g.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
}

func TestGraphMergeAdd(t *testing.T) {
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
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChildren(component.Name, dep), test.ShouldBeNil)
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
		gB.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gB.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	out = gB.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
	})
	test.That(t, gA.MergeAdd(gB), test.ShouldBeNil)
	test.That(t, gA.AddChildren(NewName("namespace", "atype", "asubtype", "D"),
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

func TestGraphMergeRemove(t *testing.T) {
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
		{
			Name:      NewName("namespace", "atype", "asubtype", "D"),
			DependsOn: []Name{NewName("namespace", "atype", "asubtype", "C")},
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
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	out := gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
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
	gB := NewGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	for _, component := range cfgB {
		gB.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gB.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	gA.MergeRemove(gB)
	out = gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
}

func newResourceNameSet(resourceNames ...Name) map[Name]struct{} {
	set := make(map[Name]struct{}, len(resourceNames))
	for _, val := range resourceNames {
		set[val] = struct{}{}
	}
	return set
}

func TestFindNodeByName(t *testing.T) {
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
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	_, ok := gA.FindNodeByName("A")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = gA.FindNodeByName("B")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = gA.FindNodeByName("C")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = gA.FindNodeByName("D")
	test.That(t, ok, test.ShouldBeFalse)
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

func TestReplaceNodesParents(t *testing.T) {
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChildren(component.Name, dep), test.ShouldBeNil)
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
		gB.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gB.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	for n := range gB.Nodes {
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
		gC.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gC.AddChildren(component.Name, dep), test.ShouldBeNil)
		}
	}
	test.That(t, gA.ReplaceNodesParents(NewName("namespace", "atype", "asubtype", "W"), gC), test.ShouldNotBeNil)
}

func TestCopyNodeAndChildren(t *testing.T) {
	gA := NewGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			test.That(t, gA.AddChildren(component.Name, dep), test.ShouldBeNil)
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

	for n := range gA.Nodes {
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

func debugTransClosPrint(m transitiveClosureMatrix) {
	names := make([]Name, 0)
	keys := make([]string, 0)
	fmt.Print(" \t")
	for k := range m {
		names = append(names, k)
		keys = append(keys, k.Name)
	}
	for _, k := range keys {
		fmt.Printf("%s\t", k)
	}
	fmt.Print("\r\n")
	for i, k := range keys {
		fmt.Printf("%s\t", k)
		for j := range keys {
			fmt.Printf("%d\t", m[names[i]][names[j]])
		}
		fmt.Print("\r\n")
	}
}
