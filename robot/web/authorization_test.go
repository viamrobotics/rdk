package web

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

const (
	testUser        = "c6f77790-5405-488a-9c9c-9612402fb9b0"
	getReadings     = "/viam.component.sensor.v1.SensorService/GetReadings"
	addStream       = "/proto.stream.v1.StreamService/AddStream"
	machineStatus   = "/viam.robot.v1.RobotService/GetMachineStatus"
	robotGetVersion = "/viam.robot.v1.RobotService/GetVersion"
)

func ctxWithUser(user string) context.Context {
	return rpc.ContextWithAuthEntity(context.Background(), rpc.EntityInfo{Entity: user})
}

func assertDenied(t *testing.T, err error) {
	t.Helper()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.PermissionDenied)
}

func TestRolesAuthorizerNoRoles(t *testing.T) {
	test.That(t, newRolesAuthorizer(nil, logging.NewTestLogger(t)), test.ShouldBeNil)
}

func TestRolesAuthorizer(t *testing.T) {
	ra := newRolesAuthorizer([]config.Role{
		{
			User: testUser,
			Permissions: []config.Permission{
				{Resource: "cam1", Methods: []string{getImagesMethod}},
				{Resource: "sensor1", Methods: []string{getReadings}},
			},
		},
	}, logging.NewTestLogger(t))
	test.That(t, ra, test.ShouldNotBeNil)

	ctx := ctxWithUser(testUser)

	// granted resource methods
	err := ra.authorize(ctx, getImagesMethod, &camerapb.GetImagesRequest{Name: "cam1"})
	test.That(t, err, test.ShouldBeNil)
	err = ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"})
	test.That(t, err, test.ShouldBeNil)

	// same method, ungranted resource
	assertDenied(t, ra.authorize(ctx, getImagesMethod, &camerapb.GetImagesRequest{Name: "cam2"}))
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "cam1"}))

	// default endpoints always allowed; other robot endpoints are not
	err = ra.authorize(ctx, machineStatus, nil)
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(ctx, robotGetVersion, nil))

	// GetImages implies name-scoped stream endpoints and ListStreams
	err = ra.authorize(ctx, addStream, &streampb.AddStreamRequest{Name: "cam1"})
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(ctx, addStream, &streampb.AddStreamRequest{Name: "cam2"}))
	err = ra.authorize(ctx, listStreamsMethod, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)

	// user without a role and no default role: default endpoints only
	otherCtx := ctxWithUser("some-other-user")
	err = ra.authorize(otherCtx, machineStatus, nil)
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(otherCtx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))

	// no auth entity in context means the method was exempt from auth
	err = ra.authorize(context.Background(), robotGetVersion, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestRolesAuthorizerDefaultUser(t *testing.T) {
	ra := newRolesAuthorizer([]config.Role{
		{User: testUser, Permissions: []config.Permission{{Resource: "cam1", Methods: []string{getImagesMethod}}}},
		{User: config.DefaultRoleUser, Permissions: []config.Permission{{Resource: "sensor1", Methods: []string{getReadings}}}},
	}, logging.NewTestLogger(t))

	// unknown users inherit the default role
	otherCtx := ctxWithUser("some-other-user")
	err := ra.authorize(otherCtx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"})
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(otherCtx, getImagesMethod, &camerapb.GetImagesRequest{Name: "cam1"}))

	// a user with their own role does not inherit the default role
	ctx := ctxWithUser(testUser)
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))
}

func TestRolesAuthorizerDuplicateUser(t *testing.T) {
	ra := newRolesAuthorizer([]config.Role{
		{User: testUser, Permissions: []config.Permission{{Resource: "cam1", Methods: []string{getImagesMethod}}}},
		{User: config.DefaultRoleUser, Permissions: []config.Permission{{Resource: "sensor1", Methods: []string{getReadings}}}},
		{User: testUser, Permissions: []config.Permission{{Resource: "sensor1", Methods: []string{getReadings}}}},
	}, logging.NewTestLogger(t))

	// colliding user is fully restricted: no granted methods, no default role
	// fallback, but default endpoints still work
	ctx := ctxWithUser(testUser)
	assertDenied(t, ra.authorize(ctx, getImagesMethod, &camerapb.GetImagesRequest{Name: "cam1"}))
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))
	err := ra.authorize(ctx, machineStatus, nil)
	test.That(t, err, test.ShouldBeNil)

	// a duplicate role after the collision does not resurrect permissions
	ra = newRolesAuthorizer([]config.Role{
		{User: testUser},
		{User: testUser},
		{User: testUser, Permissions: []config.Permission{{Resource: "cam1", Methods: []string{getImagesMethod}}}},
	}, logging.NewTestLogger(t))
	assertDenied(t, ra.authorize(ctx, getImagesMethod, &camerapb.GetImagesRequest{Name: "cam1"}))
}
