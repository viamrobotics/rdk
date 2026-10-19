package resource

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// robotSizedGraph builds a graph shaped like a robot with one remote: the
// remote and 50 resources on it, plus 60 local resources in three layers
// (components, services on components and remote resources, and services on
// services). It returns the graph, the remote's name and a component with a
// dozen dependents.
func robotSizedGraph(tb testing.TB) (*Graph, Name, Name) {
	tb.Helper()
	g := NewGraph(logging.NewTestLogger(tb))
	add := func(name Name, deps ...Name) {
		test.That(tb, g.AddNode(name, &GraphNode{}), test.ShouldBeNil)
		for _, dep := range deps {
			test.That(tb, g.AddChild(name, dep), test.ShouldBeNil)
		}
	}

	remote := NewName(APINamespace("rdk").WithType("component").WithSubtype("remote"), "remote1")
	add(remote)
	remoteRes := make([]Name, 50)
	for i := range remoteRes {
		remoteRes[i] = NewName(apiA, fmt.Sprintf("r%d", i)).PrependRemote(remote.Name)
		add(remoteRes[i], remote)
	}

	components := make([]Name, 20)
	for i := range components {
		components[i] = NewName(apiA, fmt.Sprintf("c%d", i))
		if i < 5 {
			add(components[i])
		} else {
			add(components[i], components[i%5])
		}
	}
	services := make([]Name, 25)
	for i := range services {
		services[i] = NewName(apiA, fmt.Sprintf("s%d", i))
		deps := []Name{components[i%20], components[(i*7+1)%20], components[(i*13+2)%20]}
		if i%2 == 0 {
			deps = append(deps, remoteRes[i%50], remoteRes[(i*3+1)%50])
		}
		add(services[i], deps...)
	}
	for i := 0; i < 15; i++ {
		add(NewName(apiA, fmt.Sprintf("h%d", i)), services[i%25], services[(i*7+3)%25], components[(i*11)%20])
	}
	return g, remote, components[3]
}

// The remote refresh marks the remote and the remote resources depending on it
// reachable on every pass.
func BenchmarkGraphMarkReachability(b *testing.B) {
	g, remote, _ := robotSizedGraph(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := g.MarkReachability(remote, true); err != nil {
			b.Fatal(err)
		}
	}
}

// Marking a resource's dependents for update starts from its sub-graph.
func BenchmarkGraphSubGraphFrom(b *testing.B) {
	g, _, component := robotSizedGraph(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.SubGraphFrom(component); err != nil {
			b.Fatal(err)
		}
	}
}
