package builtin

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestExploreMode(t *testing.T) {
	t.Skip()
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_no_map_cfg.json")

	var points []r3.Vector
	mockExploreMotionService := &inject.MotionService{}
	mockExploreMotionService.MoveFunc = func(ctx context.Context, componentName resource.Name,
		destination *frame.PoseInFrame, worldState *frame.WorldState, constraints *v1.Constraints,
		extra map[string]interface{},
	) (bool, error) {
		points = append(points, destination.Pose().Point())
		return false, errors.New("expected error")
	}

	nsStruct := ns.(*builtIn)
	nsStruct.exploreMotionService = mockExploreMotionService

	ctxTimeout, cancelFunc := context.WithTimeout(cancelCtx, 50*time.Millisecond)
	defer cancelFunc()
	nsStruct.startExploreMode(ctxTimeout)
	<-ctxTimeout.Done()
	teardown()
	test.That(t, len(points), test.ShouldBeGreaterThan, 2)
}
