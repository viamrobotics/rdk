package datamanager

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestNewDataManager(t *testing.T) {
	// Empty config at initialization.
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: &Config{},
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	const arm1Key = "arm1"
	arm1 := &inject.Arm{}
	arm1.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1Key): arm1}
	r.MockResourcesFromMap(rs)

	dataManager, err := New(context.Background(), r, cfgService, logger)
	svc := dataManager.(*Service)
	test.That(t, err, test.ShouldBeNil)

	// Set data manager parameters in Update.
	conf, err := config.Read(
		context.Background(), utils.ResolveFile("robots/configs/fake_robot_with_data_manager.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	svc.Update(context.Background(), conf)

	sleepTime := time.Millisecond * 5
	time.Sleep(sleepTime)

	// Check that a collector is running.
	test.That(t, len(svc.collectors), test.ShouldEqual, 1)

	expectedComponentMethodMetadata := componentMethodMetadata{
		"arm1", data.MethodMetadata{Subtype: resource.SubtypeName("arm"), MethodName: "GetEndPosition"},
	}
	_, present := svc.collectors[expectedComponentMethodMetadata]
	test.That(t, present, test.ShouldBeTrue)

	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Check that collector is closed.
	time.Sleep(sleepTime)
	test.That(t, svc.collectors, test.ShouldBeEmpty)
}
