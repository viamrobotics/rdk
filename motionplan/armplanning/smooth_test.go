package armplanning

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

type smoothNodeJSON struct {
	Inputs referenceframe.FrameSystemInputs `json:"inputs"`
	Corner bool                             `json:"corner"`
}

func loadTestSmoothNodes(t testing.TB) []*node {
	t.Helper()
	data, err := os.ReadFile("data/smooth-nodes.json")
	test.That(t, err, test.ShouldBeNil)

	var raw []smoothNodeJSON
	test.That(t, json.Unmarshal(data, &raw), test.ShouldBeNil)

	nodes := make([]*node, len(raw))
	for i, r := range raw {
		nodes[i] = &node{
			inputs: r.Inputs.ToLinearInputs(),
			corner: r.Corner,
		}
	}
	return nodes
}

func TestSmoothPlans1(t *testing.T) {
	t.Parallel()
	testSmoothNodes := loadTestSmoothNodes(t)
	test.That(t, len(testSmoothNodes), test.ShouldEqual, 62)

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)
	req.myTestOptions.doNotCloseObstacles = true

	pc, err := newPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(t, err, test.ShouldBeNil)

	// Convert []*node to []referenceframe.FrameSystemInputs for smoothPath
	inputSlice := make([]*referenceframe.LinearInputs, len(testSmoothNodes))
	for i, n := range testSmoothNodes {
		inputSlice[i] = n.inputs
	}

	nodes, err := smoothPath(ctx, psc, inputSlice)
	test.That(t, err, test.ShouldBeNil)

	for idx, n := range nodes {
		logger.Infof("%d : %v", idx, n.Get("arm-left"))
	}
	// Smoothing reduces 62 waypoints to 3, then addCloseObstacleWaypoints adds 5 more
	// where the path comes within 5mm of obstacles
	test.That(t, len(nodes), test.ShouldEqual, 3)
}

func BenchmarkSmoothPlans1(b *testing.B) {
	// go test -bench Smooth -benchtime 5s -cpuprofile cpu.out && go tool pprof cpu.out
	testSmoothNodes := loadTestSmoothNodes(b)
	test.That(b, len(testSmoothNodes), test.ShouldEqual, 62)

	ctx := context.Background()
	logger := logging.NewTestLogger(b)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	req.myTestOptions.doNotCloseObstacles = true
	test.That(b, err, test.ShouldBeNil)

	pc, err := newPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(b, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(b, err, test.ShouldBeNil)

	// Convert []*node to []referenceframe.FrameSystemInputs for smoothPath
	inputSlice := make([]*referenceframe.LinearInputs, len(testSmoothNodes))
	for i, n := range testSmoothNodes {
		inputSlice[i] = n.inputs
	}

	b.ResetTimer()
	for b.Loop() {
		nodes, err := smoothPath(ctx, psc, inputSlice)
		test.That(b, err, test.ShouldBeNil)
		test.That(b, len(nodes), test.ShouldEqual, 5)
	}
}
