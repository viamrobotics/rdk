package web

import (
	"context"
	"strings"

	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// defaultEndpoints are invocable by any authenticated user, even fully restricted
// ones. They are the endpoints required to establish and maintain a standard SDK
// connection with the machine and no more.
var defaultEndpoints = map[string]bool{
	"/viam.robot.v1.RobotService/GetSessions":          true,
	"/viam.robot.v1.RobotService/ResourceNames":        true,
	"/viam.robot.v1.RobotService/ResourceRPCSubtypes":  true,
	"/viam.robot.v1.RobotService/StartSession":         true,
	"/viam.robot.v1.RobotService/SendSessionHeartbeat": true,
	"/viam.robot.v1.RobotService/GetMachineStatus":     true,
}

const (
	getImagesMethod   = "/viam.component.camera.v1.CameraService/GetImages"
	listStreamsMethod = "/proto.stream.v1.StreamService/ListStreams"
)

// nameScopedStreamMethods are granted for a camera's name when a user may invoke
// GetImages on that name; without them a user cannot view live feeds.
var nameScopedStreamMethods = []string{
	"/proto.stream.v1.StreamService/AddStream",
	"/proto.stream.v1.StreamService/RemoveStream",
	"/proto.stream.v1.StreamService/GetStreamOptions",
	"/proto.stream.v1.StreamService/SetStreamOptions",
}

// rolesAuthorizer decides whether authenticated users may invoke endpoints based on
// the roles section of the machine config. A nil *rolesAuthorizer allows everything.
type rolesAuthorizer struct {
	// perms maps user -> method -> resource names allowed for that method.
	perms  map[string]map[string]map[string]bool
	logger logging.Logger
}

// newRolesAuthorizer returns an authorizer for the given roles, or nil if roles is
// empty (all users unrestricted).
func newRolesAuthorizer(roles []config.Role, logger logging.Logger) *rolesAuthorizer {
	if len(roles) == 0 {
		return nil
	}
	ra := &rolesAuthorizer{perms: map[string]map[string]map[string]bool{}, logger: logger}

	restricted := map[string]bool{}
	for _, role := range roles {
		if _, seen := ra.perms[role.User]; seen || restricted[role.User] {
			ra.logger.Errorw(
				"multiple role definitions for user; fully restricting user until collision is fixed",
				"user", role.User)
			// An explicit empty grant map fully restricts: the user matches a role, so
			// they do not fall back to the default user's permissions.
			ra.perms[role.User] = map[string]map[string]bool{}
			restricted[role.User] = true
			continue
		}
		userPerms := map[string]map[string]bool{}
		for _, perm := range role.Permissions {
			for _, method := range perm.Methods {
				if userPerms[method] == nil {
					userPerms[method] = map[string]bool{}
				}
				userPerms[method][perm.Resource] = true
			}
		}
		// Live feeds require the stream service, so GetImages on a camera implies the
		// stream endpoints for that camera's name (and ListStreams for any name).
		for camName := range userPerms[getImagesMethod] {
			for _, method := range nameScopedStreamMethods {
				if userPerms[method] == nil {
					userPerms[method] = map[string]bool{}
				}
				userPerms[method][camName] = true
			}
			if userPerms[listStreamsMethod] == nil {
				userPerms[listStreamsMethod] = map[string]bool{"": true}
			}
		}
		ra.perms[role.User] = userPerms
	}
	return ra
}

// authorize returns nil if the authenticated user in ctx may invoke fullMethod. req
// may be nil for streaming RPCs whose first message has not yet been received.
func (ra *rolesAuthorizer) authorize(ctx context.Context, fullMethod string, req interface{}) error {
	if defaultEndpoints[fullMethod] {
		return nil
	}

	// Methods reachable without authentication (e.g. rpc auth/signaling handshakes)
	// carry no auth entity; anything else was already gated by the auth interceptor.
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return nil
	}

	userPerms, ok := ra.perms[entity.Entity]
	if !ok {
		userPerms = ra.perms[config.DefaultRoleUser]
	}
	resourceName := resourceNameFromRequest(fullMethod, req)
	if userPerms != nil && userPerms[fullMethod][resourceName] {
		return nil
	}
	return status.Errorf(codes.PermissionDenied,
		"user is not authorized to invoke %q on resource %q", fullMethod, resourceName)
}

func resourceNameFromRequest(fullMethod string, req interface{}) string {
	if req == nil {
		return ""
	}
	parts := strings.SplitN(strings.TrimPrefix(fullMethod, "/"), "/", 2)
	if len(parts) != 2 {
		return ""
	}
	return resource.GetResourceNameFromRequest(parts[0], parts[1], req)
}

// UnaryInterceptor enforces configured roles on unary RPCs.
func (ra *rolesAuthorizer) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *googlegrpc.UnaryServerInfo,
	handler googlegrpc.UnaryHandler,
) (interface{}, error) {
	if err := ra.authorize(ctx, info.FullMethod, req); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// StreamInterceptor enforces configured roles on streaming RPCs. Resource-level
// authorization happens on receipt of the stream's first message, since that is
// where the resource name lives.
func (ra *rolesAuthorizer) StreamInterceptor(
	srv interface{},
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	// Machine-scoped check (empty resource name) up front; it covers non-message-bound
	// grants and rejects users with no grant on this method for any resource early.
	if err := ra.authorize(ss.Context(), info.FullMethod, nil); err == nil {
		return handler(srv, ss)
	} else if !ra.methodGranted(ss.Context(), info.FullMethod) {
		return err
	}
	return handler(srv, &authzServerStream{ServerStream: ss, ra: ra, fullMethod: info.FullMethod})
}

// methodGranted reports whether the user has the method granted for at least one
// resource name, meaning first-message inspection could still authorize the stream.
func (ra *rolesAuthorizer) methodGranted(ctx context.Context, fullMethod string) bool {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return false
	}
	userPerms, ok := ra.perms[entity.Entity]
	if !ok {
		userPerms = ra.perms[config.DefaultRoleUser]
	}
	return len(userPerms[fullMethod]) != 0
}

type authzServerStream struct {
	googlegrpc.ServerStream
	ra         *rolesAuthorizer
	fullMethod string
	checked    bool
}

func (ss *authzServerStream) RecvMsg(m interface{}) error {
	if err := ss.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if !ss.checked {
		ss.checked = true
		if err := ss.ra.authorize(ss.Context(), ss.fullMethod, m); err != nil {
			return err
		}
	}
	return nil
}
