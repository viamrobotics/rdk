package web

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	robotpb "go.viam.com/api/robot/v1"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

const (
	testKeyID     = "c6f77790-5405-488a-9c9c-9612402fb9b0"
	testEmail     = "benji@viam.com"
	getImages     = "/viam.component.camera.v1.CameraService/GetImages"
	getReadings   = "/viam.component.sensor.v1.SensorService/GetReadings"
	listStreams   = "/proto.stream.v1.StreamService/ListStreams"
	addStream     = "/proto.stream.v1.StreamService/AddStream"
	resourceNames = "/viam.robot.v1.RobotService/ResourceNames"
	machineStatus = "/viam.robot.v1.RobotService/GetMachineStatus"
	authenticate  = "/proto.rpc.v1.AuthService/Authenticate"
)

func ctxWithUser(user string) context.Context {
	return rpc.ContextWithAuthEntity(context.Background(), rpc.EntityInfo{Entity: user})
}

func assertDenied(t *testing.T, err error) {
	t.Helper()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.PermissionDenied)
}

func apiKeyUser(id string) config.User {
	return config.User{Type: config.UserTypeAPIKeyID, ID: id}
}

func emailUser(id string) config.User {
	return config.User{Type: config.UserTypeEmail, ID: id}
}

func TestUserPermsAuthorizerNoUserPerms(t *testing.T) {
	test.That(t, newUserPermsAuthorizer(nil, logging.NewTestLogger(t)), test.ShouldBeNil)
}

func TestUserPermsAuthorizer(t *testing.T) {
	// The same person's API key ID and e-mail are two entries sharing permissions.
	sharedPerms := []config.Permission{
		{Resources: []string{"robot"}, AllowedMethods: []string{resourceNames}},
		{Resources: []string{"streams"}, AllowedMethods: []string{listStreams}},
		{Resources: []string{"cam1", "cam2"}, AllowedMethods: []string{getImages, addStream}},
		{Resources: []string{"sensor1"}, AllowedMethods: []string{getReadings}},
	}
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{User: apiKeyUser(testKeyID), Permissions: sharedPerms},
		{User: emailUser(testEmail), Permissions: sharedPerms},
	}, logging.NewTestLogger(t))
	test.That(t, ra, test.ShouldNotBeNil)

	for _, ctx := range []context.Context{ctxWithUser(testKeyID), ctxWithUser(testEmail)} {
		// resource-scoped grants apply to every listed resource and no others
		err := ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"})
		test.That(t, err, test.ShouldBeNil)
		err = ra.authorize(ctx, addStream, &streampb.AddStreamRequest{Name: "cam2"})
		test.That(t, err, test.ShouldBeNil)
		assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam3"}))
		assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "cam1"}))

		// machine-scoped methods work when granted under any resource name
		err = ra.authorize(ctx, resourceNames, &robotpb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		err = ra.authorize(ctx, listStreams, &streampb.ListStreamsRequest{})
		test.That(t, err, test.ShouldBeNil)

		// no default endpoints: ungranted robot methods are denied
		assertDenied(t, ra.authorize(ctx, machineStatus, &robotpb.GetMachineStatusRequest{}))

		// auth handshake plumbing is always exempt
		err = ra.authorize(ctx, authenticate, nil)
		test.That(t, err, test.ShouldBeNil)
	}

	// unknown user with no default entry: fully restricted
	otherCtx := ctxWithUser("someone-else")
	assertDenied(t, ra.authorize(otherCtx, resourceNames, &robotpb.ResourceNamesRequest{}))
	assertDenied(t, ra.authorize(otherCtx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))

	// unauthenticated: fully restricted, but plumbing still exempt
	assertDenied(t, ra.authorize(context.Background(), resourceNames, &robotpb.ResourceNamesRequest{}))
	err := ra.authorize(context.Background(), authenticate, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestUserPermsAuthorizerDefaultUser(t *testing.T) {
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{
			User:        apiKeyUser(testKeyID),
			Permissions: []config.Permission{{Resources: []string{"cam1"}, AllowedMethods: []string{getImages}}},
		},
		{
			User: config.User{Type: config.UserTypeDefault},
			Permissions: []config.Permission{
				{Resources: []string{"robot"}, AllowedMethods: []string{resourceNames}},
				{Resources: []string{"sensor1"}, AllowedMethods: []string{getReadings}},
			},
		},
	}, logging.NewTestLogger(t))

	// unknown authenticated users inherit the default user's permissions
	otherCtx := ctxWithUser("someone-else")
	err := ra.authorize(otherCtx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"})
	test.That(t, err, test.ShouldBeNil)
	err = ra.authorize(otherCtx, resourceNames, &robotpb.ResourceNamesRequest{})
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(otherCtx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))

	// a user with their own entry does not inherit the default user's permissions
	ctx := ctxWithUser(testKeyID)
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))

	// unauthenticated users do not get the default user's permissions
	assertDenied(t, ra.authorize(context.Background(), resourceNames, &robotpb.ResourceNamesRequest{}))
}

func TestUserPermsAuthorizerDuplicateUser(t *testing.T) {
	camPerm := []config.Permission{{Resources: []string{"cam1"}, AllowedMethods: []string{getImages}}}
	sensorPerm := []config.Permission{{Resources: []string{"sensor1"}, AllowedMethods: []string{getReadings}}}

	ra := newUserPermsAuthorizer([]config.UserPermission{
		{User: apiKeyUser(testKeyID), Permissions: camPerm},
		{User: config.User{Type: config.UserTypeDefault}, Permissions: sensorPerm},
		{User: apiKeyUser(testKeyID), Permissions: sensorPerm},
	}, logging.NewTestLogger(t))

	// colliding user is fully restricted: no granted methods and no default fallback
	ctx := ctxWithUser(testKeyID)
	assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))

	// a duplicate after the collision does not resurrect permissions
	ra = newUserPermsAuthorizer([]config.UserPermission{
		{User: apiKeyUser(testKeyID)},
		{User: apiKeyUser(testKeyID)},
		{User: apiKeyUser(testKeyID), Permissions: camPerm},
	}, logging.NewTestLogger(t))
	assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))

	// duplicate default users fully restrict unknown users
	ra = newUserPermsAuthorizer([]config.UserPermission{
		{User: config.User{Type: config.UserTypeDefault}, Permissions: sensorPerm},
		{User: config.User{Type: config.UserTypeDefault}, Permissions: camPerm},
	}, logging.NewTestLogger(t))
	otherCtx := ctxWithUser("someone-else")
	assertDenied(t, ra.authorize(otherCtx, getReadings, &commonpb.GetReadingsRequest{Name: "sensor1"}))
	assertDenied(t, ra.authorize(otherCtx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))

	// duplicate emails collide the same way api key IDs do
	ra = newUserPermsAuthorizer([]config.UserPermission{
		{User: emailUser(testEmail), Permissions: camPerm},
		{User: emailUser(testEmail), Permissions: sensorPerm},
	}, logging.NewTestLogger(t))
	emailCtx := ctxWithUser(testEmail)
	assertDenied(t, ra.authorize(emailCtx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))
}

func TestUserPermsAuthorizerSameNameDifferentAPI(t *testing.T) {
	// a permission naming "cam1" is harmless for a same-named resource of another
	// API: the method's service prefix disambiguates
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{
			User:        apiKeyUser(testKeyID),
			Permissions: []config.Permission{{Resources: []string{"cam1"}, AllowedMethods: []string{getImages}}},
		},
	}, logging.NewTestLogger(t))
	ctx := ctxWithUser(testKeyID)
	err := ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"})
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "cam1"}))
}
