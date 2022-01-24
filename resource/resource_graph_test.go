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
