package metadata_test

import (
	"context"
	"testing"

	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/metadata"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

type mock struct {
	metadata.Service
}

func (m *mock) Resources(ctx context.Context) ([]resource.Name, error) {
	return []resource.Name{metadata.Name}, nil
}

var metadataName = []resource.Name{metadata.Name}

func newServer(injectMetadata *inject.Metadata) (pb.MetadataServiceServer, error) {
	subtypeSvcMap := map[resource.Name]interface{}{
		metadata.Name: injectMetadata,
	}

	return metadata.NewServerFromMap(subtypeSvcMap)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc := &mock{}
	robot := &inject.Robot{}
	robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc, nil
	}
	return robot, svc
}

func TestFromRobot(t *testing.T) {
	robot, svc := setupInjectRobot()
	resources, err := svc.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, metadataName)

	svc1, err := metadata.FromRobot(robot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc1, test.ShouldNotBeNil)
	resources1, err := svc1.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources1, test.ShouldResemble, metadataName)

	robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not metadata", nil
	}

	svc2, err := metadata.FromRobot(robot)
	test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("metadata.Service", "string"))
	test.That(t, svc2, test.ShouldBeNil)
}

func TestNew(t *testing.T) {
	svc, err := metadata.New()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	resources, err := svc.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, metadataName)
}
