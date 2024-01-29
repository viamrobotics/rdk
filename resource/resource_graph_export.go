package resource

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

type blockWriter struct {
	indent int
	buf    bytes.Buffer
}

func (bw *blockWriter) NewBlock(header string) {
	bw.WriteString(fmt.Sprintf("%v {", header))
	bw.indent++
}

func (bw *blockWriter) NewBlockf(headerFormatStr string, args ...any) {
	bw.NewBlock(fmt.Sprintf(headerFormatStr, args...))
}

func (bw *blockWriter) EndBlock() {
	bw.indent--
	bw.WriteString("}")
}

func (bw *blockWriter) WriteString(str string) {
	bw.buf.WriteString(fmt.Sprintf("%s%s\n",
		strings.Repeat("    ", bw.indent),
		str))
}

func (bw *blockWriter) WriteStringf(formatStr string, args ...any) {
	bw.WriteString(fmt.Sprintf(formatStr, args...))
}

func (bw *blockWriter) WriteStrings(strs []string) {
	for _, str := range strs {
		bw.WriteString(str)
	}
}

func (bw *blockWriter) String() string {
	return bw.buf.String()
}

func isInternalService(name Name) bool {
	return name.API.Type.Name == "service" && name.Name == "builtin"
}

type nameNode struct {
	Name Name
	Node *GraphNode
}

func getRemotes(nodes graphNodes) []string {
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
	slices.SortFunc(ret, func(left, right nameNode) bool {
		return left.Name.String() < right.Name.String()
	})

	return ret
}

func genNodeName(name Name) string {
	return fmt.Sprintf("\"%s\\n%s\"", name.API, name.ShortName())
}

func exportNode(bw *blockWriter, name Name, node *GraphNode) {
	_, err := node.Resource()
	node.mu.RLock()
	model := node.currentModel
	updatedAt := node.updatedAt
	needsDepRes := node.needsDependencyResolution
	unresolvedDeps := strings.Join(node.unresolvedDependencies, ", ")
	node.mu.RUnlock()

	limboState := needsDepRes || len(unresolvedDeps) != 0
	tooltipNoError := fmt.Sprintf("Model: %s&#10;LogicalClock: %d&#10;NeedsDependencyResolution: %v&#10;UnresolvedDeps: [%s]",
		model.String(), updatedAt, needsDepRes, unresolvedDeps)
	switch {
	// Nodes without errors inherit the standard color. Nodes with errors show up as red with a
	// tooltip relaying the error string.
	case err == nil && !limboState:
		bw.WriteStringf("%s [tooltip=%q];", genNodeName(name), tooltipNoError)
	case err == nil && limboState:
		bw.WriteStringf("%s [color=salmon,tooltip=%q];", genNodeName(name), tooltipNoError)
	case errors.Is(err, errPendingRemoval):
		tooltip := fmt.Sprintf("State: Pending Removal\\n%s", tooltipNoError)
		bw.WriteStringf("%s [color=indianred,tooltip=%q];", genNodeName(name), tooltip)
	case errors.Is(err, errNotInitalized):
		tooltip := fmt.Sprintf("State: Not initialized\\n%s", tooltipNoError)
		bw.WriteStringf("%s [color=indianred,tooltip=%q];", genNodeName(name), tooltip)
	default:
		tooltip := fmt.Sprintf("Error: %v\\n%s", err.Error(), tooltipNoError)
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
			ret = append(ret, edge{childName, parentName})
		}
	}

	slices.SortFunc(ret, func(left, right edge) bool {
		if left.source == right.source {
			return left.dest.String() < right.dest.String()
		}

		return left.source.String() < right.source.String()
	})

	return ret
}

func exportEdge(bw *blockWriter, left, right Name) {
	bw.WriteStringf("%s -> %s", genNodeName(left), genNodeName(right))
}
