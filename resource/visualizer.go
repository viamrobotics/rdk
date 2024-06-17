package resource

import (
	"bytes"
	"cmp"
	"container/list"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const snapshotLimit = 500

// Visualizer stores a history resource graph DOT snapshots.
type Visualizer struct {
	snapshots list.List
}

// Snapshot contains a DOT snapshot string along with capture metadata.
type Snapshot struct {
	Dot       string
	CreatedAt time.Time
}

// GetSnapshotInfo contains a Snapshot string along with metadata about the snapshot
// collection.
type GetSnapshotInfo struct {
	Snapshot Snapshot
	Index    int
	Count    int
}

// SaveSnapshot takes a DOT snapshot of a resource graph.
func (viz *Visualizer) SaveSnapshot(g *Graph) error {
	dot, err := g.ExportDot()
	if err != nil {
		return err
	}
	snapshot := Snapshot{
		Dot:       dot,
		CreatedAt: time.Now(),
	}
	latestSnapshot := viz.snapshots.Front()

	// We rely on `ExportDot` to generate the exact same string given the same input
	// resource graph.
	if latestSnapshot != nil && dot == latestSnapshot.Value.(Snapshot).Dot {
		// Nothing changed since the last snapshot
		return nil
	}
	viz.snapshots.PushFront(snapshot)
	if viz.snapshots.Len() > snapshotLimit {
		viz.snapshots.Remove(viz.snapshots.Back())
	}
	return nil
}

// Count returns the number of snapshots currents stored.
func (viz *Visualizer) Count() int { return viz.snapshots.Len() }

// GetSnapshot returns a DOT snapshot at a given index, where index 0 is the latest
// snapshot.
func (viz *Visualizer) GetSnapshot(index int) (GetSnapshotInfo, error) {
	result := GetSnapshotInfo{Index: index, Count: viz.snapshots.Len()}

	if result.Count == 0 {
		return result, errors.New("no snapshots")
	}
	if index < 0 || index >= result.Count {
		return result, errors.New("out of range")
	}

	snapshot := viz.snapshots.Front()
	for i := 0; i < index; i++ {
		// Guards against race with deletion of snapshots.
		if snapshot = snapshot.Next(); snapshot == nil {
			return result, errors.New("out of range")
		}
	}
	result.Snapshot = snapshot.Value.(Snapshot)
	return result, nil
}

// blockWriter wraps a bytes.Buffer and adds some structured methods (`NewBlock`/`EndBlock`) for
// keeping indentation state.
type blockWriter struct {
	indent int
	buf    bytes.Buffer
}

// NewBlock will write some header followed by a curly brace. Indentation will be incremented for
// human legibility.
func (bw *blockWriter) NewBlock(header string) {
	bw.WriteString(fmt.Sprintf("%v {", header))
	bw.indent++
}

func (bw *blockWriter) NewBlockf(headerFormatStr string, args ...any) {
	bw.NewBlock(fmt.Sprintf(headerFormatStr, args...))
}

// EndBlock decrements indentation and writes a closing curly brace.
func (bw *blockWriter) EndBlock() {
	bw.indent--
	bw.WriteString("}")
}

// WriteString outputs the string with indentation followed by a newline.
func (bw *blockWriter) WriteString(str string) {
	bw.buf.WriteString(fmt.Sprintf("%s%s\n",
		strings.Repeat("    ", bw.indent),
		str))
}

func (bw *blockWriter) WriteStringf(formatStr string, args ...any) {
	bw.WriteString(fmt.Sprintf(formatStr, args...))
}

// WriteStrings outputs each string separately. Meaning they will all output one line per string,
// each indented.
func (bw *blockWriter) WriteStrings(strs []string) {
	for _, str := range strs {
		bw.WriteString(str)
	}
}

func (bw *blockWriter) String() string {
	return bw.buf.String()
}

// isInternalService is a rough heuristic for dividing resources into ones that users have control
// over vs those they don't.
func isInternalService(name Name) bool {
	return name.API.Type.Name == "service" && name.Name == "builtin"
}

type nameNode struct {
	Name Name
	Node *GraphNode
}

func getRemoteNames(nodes graphNodes) []string {
	uniqueRemotes := make(map[string]bool)
	//nolint
	for name, _ := range nodes {
		if name.Remote != "" {
			uniqueRemotes[name.Remote] = true
		}
	}

	var ret []string
	//nolint
	for remote, _ := range uniqueRemotes {
		ret = append(ret, remote)
	}

	slices.Sort(ret)
	return ret
}

func nodesSortedByName(nodes graphNodes) []nameNode {
	var ret []nameNode
	for name, node := range nodes {
		ret = append(ret, nameNode{name, node})
	}
	slices.SortFunc(ret, func(left, right nameNode) int {
		return cmp.Compare(left.Name.String(), right.Name.String())
	})

	return ret
}

//nolint:godot
/**
 * E.g:
 *	    MotorName
 *	rdk:component:motor
 */
func genNodeName(name Name) string {
	return fmt.Sprintf("\"%s\\n%s\"", name.ShortName(), name.API)
}

func exportNode(bw *blockWriter, name Name, node *GraphNode) {
	// Capture node state up front.
	_, err := node.Resource()
	node.mu.RLock()
	model := node.currentModel
	updatedAt := node.updatedAt
	needsDepRes := node.needsDependencyResolution
	unresolvedDepsStr := strings.Join(node.unresolvedDependencies, ", ")
	node.mu.RUnlock()

	// Dan: I'm unsure if this state can happen, but it'd be worthy to highlight if a resource has
	// no errors, but its dependencies are unresolved.
	limboState := needsDepRes || len(unresolvedDepsStr) != 0

	// Dot tooltips can get newlines by using html escape codes for \n.
	const newline = "&#10;"
	// Every tooltip contains this information. Tooltips for nodes in an error state will also
	// include error information.
	tooltipNoError := fmt.Sprintf("Model: %s%vLogicalClock: %d%vNeedsDependencyResolution: %v%vUnresolvedDeps: [%s]",
		model.String(), newline, updatedAt, newline, needsDepRes, newline, unresolvedDepsStr)

	// Color nodes based on error state.
	switch {
	case err == nil && !limboState:
		bw.WriteStringf("%s [color=bisque,tooltip=%q];", genNodeName(name), tooltipNoError)
	case err == nil && limboState:
		bw.WriteStringf("%s [color=salmon,tooltip=%q];", genNodeName(name), tooltipNoError)
	case errors.Is(err, errPendingRemoval):
		tooltip := fmt.Sprintf("State: Pending Removal%v%v", newline, tooltipNoError)
		bw.WriteStringf("%s [color=indianred,tooltip=%q];", genNodeName(name), tooltip)
	case errors.Is(err, errNotInitalized):
		tooltip := fmt.Sprintf("State: Not initialized%v%v", newline, tooltipNoError)
		bw.WriteStringf("%s [color=indianred,tooltip=%q];", genNodeName(name), tooltip)
	default:
		tooltip := fmt.Sprintf("Error: %v%v%v", err.Error(), newline, tooltipNoError)
		bw.WriteStringf("%s [color=indianred,tooltip=%q];", genNodeName(name), tooltip)
	}
}

type edge struct {
	source Name
	dest   Name
}

func edgesSortedByName(deps resourceDependencies) []edge {
	var ret []edge
	for parentName, children := range deps {
		for childName := range children {
			// In our vernacular, children depend on parents. Edges are drawn leaving children and
			// arriving at parents.
			ret = append(ret, edge{childName, parentName})
		}
	}

	slices.SortFunc(ret, func(left, right edge) int {
		if left.source == right.source {
			return cmp.Compare(left.dest.String(), right.dest.String())
		}

		return cmp.Compare(left.source.String(), right.source.String())
	})

	return ret
}

func exportEdge(bw *blockWriter, left, right Name) {
	bw.WriteStringf("%s -> %s", genNodeName(left), genNodeName(right))
}

// ExportDot exports the resource graph as a DOT representation for visualization.
// DOT reference: https://graphviz.org/doc/info/lang.html.
// This function will output the exact same string given the same input resource graph.
// If not called inside a resourceGraphLock, there is a chance
// of the graph changing as the snapshot is being taken.
func (g *Graph) ExportDot() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// The text output for a graph is organized in the following sections:
	// - Resource nodes for "internal" local resources (e.g: data manager).
	// - Resource nodes for other local resources (i.e: user-configured components/services).
	// - For each remote, we use the same clustering.
	// - Lastly, we print out all edges.
	//
	// We also desire that if the resource graph doesn't change between two calls to `ExportDot`,
	// the same text output is produced. We desire this such that a diff in the output implies the
	// resource graph has logically changed. To achieve this, we sort all of the inputs (nodes and
	// edges) for a predictable iteration order.
	//
	// As graph nodes are not all locked during the duration of this function, nodes could change
	// before it gets exported. This is known and expected as the graph output is "best-effort".
	nodesSortedByName := nodesSortedByName(g.nodes)

	writer := &blockWriter{}
	writer.NewBlock("digraph")
	writer.WriteStrings([]string{
		"rankdir=LR;",
		"bgcolor=azure;",
		"node [style=filled];",
	})

	// "Internal" local resources.
	{
		// `cluster` is a special name for subgraphs that results in Dot drawing a border around its
		// nodes. The `label` is the name displayed on the output graph.
		writer.NewBlock("subgraph cluster_internal")
		writer.WriteStrings([]string{
			"style=filled;",
			"color=lightblue;",
			"label=Internal",
		})

		for _, nameNode := range nodesSortedByName {
			name, node := nameNode.Name, nameNode.Node
			if isInternalService(name) && name.Remote == "" {
				exportNode(writer, name, node)
			}
		}
		writer.EndBlock()
	}

	// Local user-configured components/services.
	for _, nameNode := range nodesSortedByName {
		name, node := nameNode.Name, nameNode.Node
		if !isInternalService(name) && name.Remote == "" {
			exportNode(writer, name, node)
		}
	}

	for idx, remote := range getRemoteNames(g.nodes) {
		writer.NewBlockf("subgraph cluster_remote_%d", idx)
		writer.WriteStrings([]string{
			"color=lightblue;",
			"style=filled;",
			fmt.Sprintf("label=%q", remote),
		})

		// Remote "internal" nodes
		{
			writer.NewBlockf("subgraph cluster_remote_%d_internal", idx)
			writer.WriteStrings([]string{
				"style=solid;",
				"color=black;",
				fmt.Sprintf("label=\"%v Internal\"", remote),
			})

			for _, nameNode := range nodesSortedByName {
				name, node := nameNode.Name, nameNode.Node
				if isInternalService(name) && name.Remote == remote {
					exportNode(writer, name, node)
				}
			}
			writer.EndBlock()
		}

		// Remote user-configured components/services.
		for _, nameNode := range nodesSortedByName {
			name, node := nameNode.Name, nameNode.Node
			if !isInternalService(name) && name.Remote == remote {
				exportNode(writer, name, node)
			}
		}

		writer.EndBlock() // end of remote block
	}

	for _, edge := range edgesSortedByName(g.children) {
		exportEdge(writer, edge.source, edge.dest)
	}
	writer.EndBlock() // digraph

	return writer.String(), nil
}
