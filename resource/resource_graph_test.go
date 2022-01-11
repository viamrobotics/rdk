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

func TestResourceGraphConstruct(t *testing.T) {
	for j, c := range []struct {
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
			"circular dependency - A already depends on B",
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
			"B cannot depend on itself",
		},
	} {
		t.Run(fmt.Sprintf("Test Graph Buiding %d", j), func(t *testing.T) {
			g := NewResourceGraph()
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
	g := NewResourceGraph()
	test.That(t, g, test.ShouldNotBeNil)
	for _, component := range cfg {
		g.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			err := g.AddChildren(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
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
	gA := NewResourceGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			err := gA.AddChildren(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
		}
	}
	out := gA.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "C"),
		NewName("namespace", "atype", "asubtype", "B"),
		NewName("namespace", "atype", "asubtype", "A"),
	})
	gB := NewResourceGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	for _, component := range cfgB {
		gB.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			err := gB.AddChildren(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
		}
	}
	out = gB.TopologicalSort()
	test.That(t, out, test.ShouldResemble, []Name{
		NewName("namespace", "atype", "asubtype", "F"),
		NewName("namespace", "atype", "asubtype", "E"),
		NewName("namespace", "atype", "asubtype", "D"),
	})
	err := gA.MergeAdd(gB)
	test.That(t, err, test.ShouldBeNil)
	err = gA.AddChildren(NewName("namespace", "atype", "asubtype", "D"), NewName("namespace", "atype", "asubtype", "C"))
	test.That(t, err, test.ShouldBeNil)
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
	gA := NewResourceGraph()
	test.That(t, gA, test.ShouldNotBeNil)
	for _, component := range cfgA {
		gA.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			err := gA.AddChildren(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
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
	gB := NewResourceGraph()
	test.That(t, gB, test.ShouldNotBeNil)
	for _, component := range cfgB {
		gB.AddNode(component.Name, struct{}{})
		for _, dep := range component.DependsOn {
			err := gB.AddChildren(component.Name, dep)
			test.That(t, err, test.ShouldBeNil)
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
