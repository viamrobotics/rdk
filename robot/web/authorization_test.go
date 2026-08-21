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
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
)

const (
	testKeyID     = "c6f77790-5405-488a-9c9c-9612402fb9b0"
	testAppUserID = "a1b2c3d4-0000-4000-8000-000000000001"
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

// ctxWithAppUserID simulates an SSO client as the auth layer presents it: the auth
// entity is the FusionAuth user ID, and the app user ID is on the entity's auth metadata.
func ctxWithAppUserID(subject, appUserID string) context.Context {
	return rpc.ContextWithAuthEntity(context.Background(), rpc.EntityInfo{
		Entity:       subject,
		AuthMetadata: map[string]string{"app_user_id": appUserID},
	})
}

func assertDenied(t *testing.T, err error) {
	t.Helper()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.PermissionDenied)
}

func apiKeyUser(id string) config.User {
	return config.User{Type: config.UserTypeAPIKeyID, ID: id}
}

func appUserIDUser(id string) config.User {
	return config.User{Type: config.UserTypeAppUserID, ID: id}
}

func TestUserPermsAuthorizerNoUserPerms(t *testing.T) {
	test.That(t, newUserPermsAuthorizer(nil, logging.NewTestLogger(t)), test.ShouldBeNil)
	test.That(t, newUserPermsAuthorizer([]config.UserPermission{}, logging.NewTestLogger(t)), test.ShouldBeNil)
}

func TestUserPermsAuthorizer(t *testing.T) {
	// The same person's API key ID and app user ID are two entries sharing permissions.
	sharedPerms := []config.Permission{
		{Resources: []string{"_machine"}, AllowedMethods: []string{resourceNames, listStreams}},
		{Resources: []string{"cam1", "cam2"}, AllowedMethods: []string{getImages, addStream}},
		{Resources: []string{"sensor1"}, AllowedMethods: []string{getReadings}},
	}
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{User: apiKeyUser(testKeyID), Permissions: sharedPerms},
		{User: appUserIDUser(testAppUserID), Permissions: sharedPerms},
	}, logging.NewTestLogger(t))
	test.That(t, ra, test.ShouldNotBeNil)

	appUserCtx := ctxWithAppUserID("95ae43d3-a007-4e75-afd0-ea9317758a48", testAppUserID)
	for _, ctx := range []context.Context{ctxWithUser(testKeyID), appUserCtx} {
		// resource-scoped grants apply to every listed resource and no others
		err := ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"})
		test.That(t, err, test.ShouldBeNil)
		err = ra.authorize(ctx, addStream, &streampb.AddStreamRequest{Name: "cam2"})
		test.That(t, err, test.ShouldBeNil)
		assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam3"}))
		assertDenied(t, ra.authorize(ctx, getReadings, &commonpb.GetReadingsRequest{Name: "cam1"}))

		// machine-scoped methods match only _machine grants
		err = ra.authorize(ctx, resourceNames, &robotpb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		err = ra.authorize(ctx, listStreams, &streampb.ListStreamsRequest{})
		test.That(t, err, test.ShouldBeNil)

		// ungranted machine-scoped methods are denied: no default endpoints
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

func TestUserPermsAuthorizerMachineSentinel(t *testing.T) {
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{
			User: apiKeyUser(testKeyID),
			Permissions: []config.Permission{
				// machine-scoped method granted under a plain resource name does nothing
				{Resources: []string{"robot"}, AllowedMethods: []string{resourceNames}},
				// resource method granted under _machine does nothing for named requests
				{Resources: []string{"_machine"}, AllowedMethods: []string{getImages}},
			},
		},
	}, logging.NewTestLogger(t))

	ctx := ctxWithUser(testKeyID)
	assertDenied(t, ra.authorize(ctx, resourceNames, &robotpb.ResourceNamesRequest{}))
	assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))
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
				{Resources: []string{"_machine"}, AllowedMethods: []string{resourceNames}},
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

	// duplicate app user IDs collide the same way api key IDs do
	ra = newUserPermsAuthorizer([]config.UserPermission{
		{User: appUserIDUser(testAppUserID), Permissions: camPerm},
		{User: appUserIDUser(testAppUserID), Permissions: sensorPerm},
	}, logging.NewTestLogger(t))
	appUserCtx := ctxWithAppUserID("95ae43d3-a007-4e75-afd0-ea9317758a48", testAppUserID)
	assertDenied(t, ra.authorize(appUserCtx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))
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

func TestUpdateUserPermissionsRevocation(t *testing.T) {
	logger := logging.NewTestLogger(t)
	svc := &webService{logger: logger}

	// seed: benji may stream cam1 and cam2
	svc.UpdateUserPermissions([]config.UserPermission{
		{
			User: apiKeyUser(testKeyID),
			Permissions: []config.Permission{
				{Resources: []string{"cam1", "cam2"}, AllowedMethods: []string{addStream, getImages}},
			},
		},
	})

	newStream := func(id rdkgrpc.Identity, method, resourceName string) *authzServerStream {
		ctx, cancel := context.WithCancel(context.Background())
		ss := &authzServerStream{
			svc: svc, ctx: ctx, cancel: cancel, fullMethod: method,
			id: id, checked: true, resourceName: resourceName,
		}
		svc.registerAuthzStream(ss)
		return ss
	}
	cam1Stream := newStream(rdkgrpc.Identity{Entity: testKeyID}, getImages, "cam1")
	cam2Stream := newStream(rdkgrpc.Identity{Entity: testKeyID}, getImages, "cam2")

	// narrowing to cam1-only revokes exactly the cam2 stream
	svc.UpdateUserPermissions([]config.UserPermission{
		{
			User: apiKeyUser(testKeyID),
			Permissions: []config.Permission{
				{Resources: []string{"cam1"}, AllowedMethods: []string{addStream, getImages}},
			},
		},
	})
	test.That(t, cam1Stream.revoked.Load(), test.ShouldBeFalse)
	test.That(t, cam2Stream.revoked.Load(), test.ShouldBeTrue)
	test.That(t, cam2Stream.ctx.Err(), test.ShouldNotBeNil)
	assertDenied(t, cam2Stream.SendMsg(nil))

	// unary authorization reflects the new permissions immediately
	ra := svc.userPermsAuth.Load()
	test.That(t, ra, test.ShouldNotBeNil)
	ctx := ctxWithUser(testKeyID)
	err := ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"})
	test.That(t, err, test.ShouldBeNil)
	assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam2"}))

	// removing user_permissions entirely restores unrestricted access; no revocations
	svc.UpdateUserPermissions(nil)
	test.That(t, svc.userPermsAuth.Load(), test.ShouldBeNil)
	test.That(t, cam1Stream.revoked.Load(), test.ShouldBeFalse)

	// a no-op update does not log or re-sweep (same pointer retained)
	svc.UpdateUserPermissions(nil)
	test.That(t, svc.userPermsAuth.Load(), test.ShouldBeNil)
}

func TestIdentityString(t *testing.T) {
	test.That(t, rdkgrpc.Identity{}.String(), test.ShouldEqual, "unauthenticated client")
	test.That(t, rdkgrpc.Identity{Entity: "key-id"}.String(), test.ShouldEqual, "key-id")
	test.That(t, rdkgrpc.Identity{Entity: "sub", AppUserID: "11111"}.String(),
		test.ShouldEqual, "user_id 11111")
	test.That(t, rdkgrpc.Identity{Entity: "sub", Email: "steve@viam.com"}.String(),
		test.ShouldEqual, "steve@viam.com")
	test.That(t, rdkgrpc.Identity{Entity: "sub", AppUserID: "11111", Email: "steve@viam.com"}.String(),
		test.ShouldEqual, "steve@viam.com (user_id 11111)")
}

func TestUserPermsAuthorizerDenialLogging(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	// This authorizer grants nothing to our SSO caller, so the request is denied.
	ra := newUserPermsAuthorizer([]config.UserPermission{
		{User: apiKeyUser("someone-else"), Permissions: nil},
	}, logger)
	test.That(t, ra, test.ShouldNotBeNil)

	// An SSO caller as the auth layer presents it: FusionAuth subject as the entity,
	// app user ID and e-mail on the auth metadata.
	ctx := rpc.ContextWithAuthEntity(context.Background(), rpc.EntityInfo{
		Entity:       "95ae43d3-a007-4e75-afd0-ea9317758a48",
		AuthMetadata: map[string]string{"app_user_id": "11111", "email": "steve@viam.com"},
	})
	assertDenied(t, ra.authorize(ctx, getImages, &camerapb.GetImagesRequest{Name: "cam1"}))

	denials := logs.FilterMessageSnippet("unauthorized").All()
	test.That(t, denials, test.ShouldHaveLength, 1)
	test.That(t, denials[0].Message, test.ShouldEqual,
		getImages+" request from steve@viam.com (user_id 11111) unauthorized")
}

// fakeServerStream is a minimal googlegrpc.ServerStream whose only meaningful method is
// Context(); the stream-creation gate and our test handler use nothing else.
type fakeServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }

func TestUserPermsStreamInterceptorGate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	svc := &webService{logger: logger}
	// testKeyID may invoke getImages, but only on cam1.
	svc.UpdateUserPermissions([]config.UserPermission{
		{
			User:        apiKeyUser(testKeyID),
			Permissions: []config.Permission{{Resources: []string{"cam1"}, AllowedMethods: []string{getImages}}},
		},
	})

	// call runs the stream interceptor and reports whether the handler was reached.
	call := func(ctx context.Context, method string) (handlerRan bool, err error) {
		info := &googlegrpc.StreamServerInfo{FullMethod: method}
		handler := func(_ interface{}, _ googlegrpc.ServerStream) error {
			handlerRan = true
			return nil
		}
		err = svc.userPermsStreamInterceptor(nil, &fakeServerStream{ctx: ctx}, info, handler)
		return handlerRan, err
	}

	// user with access to the method on some resource: gate passes, handler runs (the
	// per-resource check happens later, on the first message)
	ran, err := call(ctxWithUser(testKeyID), getImages)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ran, test.ShouldBeTrue)

	// unknown user with no access to the method at all: rejected before the handler
	ran, err = call(ctxWithUser("nobody"), getImages)
	assertDenied(t, err)
	test.That(t, ran, test.ShouldBeFalse)

	// known user, but a method they have zero access to: rejected before the handler
	ran, err = call(ctxWithUser(testKeyID), getReadings)
	assertDenied(t, err)
	test.That(t, ran, test.ShouldBeFalse)

	// exempt plumbing methods always pass through, even unauthenticated
	ran, err = call(context.Background(), authenticate)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ran, test.ShouldBeTrue)
}
