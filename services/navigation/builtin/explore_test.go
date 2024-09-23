package builtin

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils/inject"
)

func TestExploreMode(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_no_map_cfg.json")

	var points []r3.Vector
	mockExploreMotionService := &inject.MotionService{}
	mockExploreMotionService.MoveFunc = func(ctx context.Context, req motion.MoveReq) (bool, error) {
		points = append(points, req.Destination.Pose().Point())
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
