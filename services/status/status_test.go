package status_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	buttonSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("button"))
	button1       = resource.NameFromSubtype(buttonSubtype, "button1")
	button2       = resource.NameFromSubtype(buttonSubtype, "button2")
	button3       = resource.NameFromSubtype(buttonSubtype, "button3")

	workingSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	working1       = resource.NameFromSubtype(workingSubtype, "working1")

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	fail1       = resource.NameFromSubtype(failSubtype, "fail1")

	workingStatus = map[string]interface{}{"position": "up"}
	errFailed     = errors.New("can't get status")
)

func init() {
	registry.RegisterResourceSubtype(
		workingSubtype,
		registry.ResourceSubtype{
			Status: func(ctx context.Context, resource interface{}) (interface{}, error) { return workingStatus, nil },
		},
	)

	registry.RegisterResourceSubtype(
		failSubtype,
		registry.ResourceSubtype{
			Status: func(ctx context.Context, resource interface{}) (interface{}, error) { return nil, errFailed },
		},
	)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	t.Run("found status service", func(t *testing.T) {
		svc, err := status.FromRobot(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)

		test.That(t, svc1.statusCount, test.ShouldEqual, 0)
		result, err := svc.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, statuses)
		test.That(t, svc1.statusCount, test.ShouldEqual, 1)
	})

	t.Run("not status service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "not status", nil
		}

		svc, err := status.FromRobot(r)
		test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("status.Service", "string"))
		test.That(t, svc, test.ShouldBeNil)
	})

	t.Run("no status service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, utils.NewResourceNotFoundError(status.Name)
		}

		svc, err := status.FromRobot(r)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(status.Name))
		test.That(t, svc, test.ShouldBeNil)
	})
}

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("no error", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)
	})
}

func TestGetStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)
	resourceNames := []resource.Name{working1, button1, fail1}
	resourceMap := map[resource.Name]interface{}{working1: "resource", button1: "resource", fail1: "resource"}

	t.Run("not found", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))
	})

	t.Run("no CreateStatus", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, statuses)
	})

	t.Run("failing resource", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), []resource.Name{fail1})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get status from %q", fail1))
	})

	t.Run("many status", func(t *testing.T) {
		expected := map[resource.Name]interface{}{
			working1: workingStatus,
			button1:  struct{}{},
		}
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))

		resp, err := svc.GetStatus(context.Background(), []resource.Name{working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		s := resp[0]
		test.That(t, s.Name, test.ShouldResemble, working1)
		test.That(t, s.Status, test.ShouldResemble, workingStatus)

		resp, err = svc.GetStatus(context.Background(), []resource.Name{working1, working1, working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		s = resp[0]
		test.That(t, s.Name, test.ShouldResemble, working1)
		test.That(t, s.Status, test.ShouldResemble, workingStatus)

		resp, err = svc.GetStatus(context.Background(), []resource.Name{working1, button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])
		test.That(t, resp[1].Status, test.ShouldResemble, expected[resp[1].Name])

		_, err = svc.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get status from %q", fail1))
	})

	t.Run("get all status", func(t *testing.T) {
		workingResourceMap := map[resource.Name]interface{}{working1: "resource", button1: "resource"}
		expected := map[resource.Name]interface{}{
			working1: workingStatus,
			button1:  struct{}{},
		}
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), workingResourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])
		test.That(t, resp[1].Status, test.ShouldResemble, expected[resp[1].Name])
	})
}

func TestUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	resourceNames := []resource.Name{button1, button2}
	resourceMap := map[resource.Name]interface{}{button1: "resource", button2: "resource"}

	t.Run("update with no resources", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, statuses)

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button1))
	})

	t.Run("update with one resource", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Status, test.ShouldResemble, struct{}{})

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{button1: "resource"})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))

		resp, err = svc.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp, test.ShouldResemble, statuses)
	})

	t.Run("update with same resources", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Status, test.ShouldResemble, struct{}{})

		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err = svc.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Status, test.ShouldResemble, struct{}{})
	})

	t.Run("update with diff resources", func(t *testing.T) {
		svc, err := status.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetStatus(context.Background(), []resource.Name{button3})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button3))

		err = svc.(resource.Updateable).Update(
			context.Background(),
			map[resource.Name]interface{}{button3: "resource"},
		)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.GetStatus(context.Background(), []resource.Name{button3})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Name, test.ShouldResemble, button3)
		test.That(t, resp[0].Status, test.ShouldResemble, struct{}{})
	})
}

var statuses = []status.Status{{Name: button1, Status: struct{}{}}}

type mock struct {
	status.Service

	statusCount int
}

func (m *mock) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
	m.statusCount++
	return statuses, nil
}
