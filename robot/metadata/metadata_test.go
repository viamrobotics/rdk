package metadata_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/metadata"
	"go.viam.com/rdk/testutils/inject"
)

type mock struct {
	metadata.Service
}

func (m *mock) Resources(ctx context.Context) ([]resource.Name, error) {
	return []resource.Name{}, nil
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc := &mock{}
	robot := &inject.Robot{}
	robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc, nil
	}
	return robot, svc
}

func TestNew(t *testing.T) {
	svc := metadata.New()
	test.That(t, svc, test.ShouldNotBeNil)

	resources, err := svc.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldHaveLength, 0)
}
