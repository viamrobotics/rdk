package web

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// exemptMethodPrefixes are gRPC service namespaces that are connection plumbing
// (authentication handshakes, WebRTC signaling, and reflection) rather than
// viam-server endpoints. Enforcing user_permissions on them could prevent clients
// from authenticating.
var exemptMethodPrefixes = []string{
	"/proto.rpc.v1.",
	"/proto.rpc.webrtc.v1.",
	"/grpc.reflection.",
}

func isExemptMethod(fullMethod string) bool {
	for _, prefix := range exemptMethodPrefixes {
		if strings.HasPrefix(fullMethod, prefix) {
			return true
		}
	}
	return false
}

const (
	// machineResource is the special resources string granting methods not associated
	// with a single resource (e.g. RobotService methods and ListStreams). Resource
	// names must begin with a letter or number, so no real resource can claim it.
	machineResource = "_machine"

	// addStreamMethod is the method that creates WebRTC video streams. It is special
	// to authorization: after it succeeds, video flows on the peer connection with no
	// further gRPC calls, so when user_permissions change, active video streams are
	// re-evaluated against this method to decide whether they may keep flowing.
	addStreamMethod = "/proto.stream.v1.StreamService/AddStream"
)

// permSet maps an allowed method to the set of resource names it may be invoked on.
type permSet map[string]map[string]bool

// userPermsAuthorizer decides whether users may invoke endpoints based on the
// user_permissions section of the machine config. A nil *userPermsAuthorizer allows
// everything.
type userPermsAuthorizer struct {
	// identityPerms is keyed by partial identities: an api-key-id user is stored as
	// Identity{Entity: id} and an app-user-id user as Identity{AppUserID: id}. Lookups
	// probe both shapes for a connected client's identity.
	identityPerms map[rdkgrpc.Identity]permSet
	defaultPerms  permSet // permissions of the "default" user; nil if none
	logger        logging.Logger
}

// newUserPermsAuthorizer returns an authorizer for the given user permissions, or nil
// if userPerms is empty (all users unrestricted).
func newUserPermsAuthorizer(userPerms []config.UserPermission, logger logging.Logger) *userPermsAuthorizer {
	if len(userPerms) == 0 {
		return nil
	}
	ra := &userPermsAuthorizer{
		identityPerms: map[rdkgrpc.Identity]permSet{},
		logger:        logger,
	}

	buildPermSet := func(permissions []config.Permission) permSet {
		perms := permSet{}
		for _, perm := range permissions {
			for _, method := range perm.AllowedMethods {
				if perms[method] == nil {
					perms[method] = map[string]bool{}
				}
				for _, res := range perm.Resources {
					perms[method][res] = true
				}
			}
		}
		return perms
	}

	for _, userPerm := range userPerms {
		user := userPerm.User
		var key rdkgrpc.Identity
		switch user.Type {
		case config.UserTypeAPIKeyID:
			key = rdkgrpc.Identity{Entity: user.ID}
		case config.UserTypeAppUserID:
			key = rdkgrpc.Identity{AppUserID: user.ID}
		case config.UserTypeDefault:
			if ra.defaultPerms != nil {
				ra.logger.Error(
					"multiple user_permissions entries for default user; fully restricting default user until collision is fixed")
				// An empty permSet fully restricts.
				ra.defaultPerms = permSet{}
				continue
			}
			ra.defaultPerms = buildPermSet(userPerm.Permissions)
			continue
		default:
			ra.logger.Warnw("unknown user type in user_permissions; ignoring user",
				"type", user.Type, "id", user.ID)
			continue
		}
		if _, seen := ra.identityPerms[key]; seen {
			ra.logger.Errorw(
				"multiple user_permissions entries for user; fully restricting user until collision is fixed",
				"type", user.Type, "id", user.ID)
			// An empty permSet fully restricts.
			ra.identityPerms[key] = permSet{}
			continue
		}
		ra.identityPerms[key] = buildPermSet(userPerm.Permissions)
	}
	return ra
}

// permsFor returns the permissions of the given identity, or nil if the identity is
// unauthenticated or unknown with no default user configured.
func (ra *userPermsAuthorizer) permsFor(id rdkgrpc.Identity) permSet {
	if !id.Authenticated() {
		// Unauthenticated (insecure connection): fully restrict.
		return nil
	}
	if perms, ok := ra.identityPerms[rdkgrpc.Identity{Entity: id.Entity}]; ok {
		return perms
	}
	// The auth entity is an API key ID or a FusionAuth user ID, never an app user ID;
	// app user IDs are matched via the token's auth metadata claim.
	if id.AppUserID != "" {
		if perms, ok := ra.identityPerms[rdkgrpc.Identity{AppUserID: id.AppUserID}]; ok {
			return perms
		}
	}
	return ra.defaultPerms
}

// allowedOnAnyResource reports whether the given identity may invoke fullMethod on at
// least one resource (including the "_machine" sentinel). It gates stream creation: a
// user with no access to a method at all should not be able to open its stream and
// receive server-pushed messages before the per-resource check runs, since that check
// needs the stream's first client message to learn the resource name.
func (ra *userPermsAuthorizer) allowedOnAnyResource(id rdkgrpc.Identity, fullMethod string) bool {
	return len(ra.permsFor(id)[fullMethod]) > 0
}

// allowed returns whether the given identity may invoke fullMethod. Requests that do
// not address a named resource (e.g. RobotService methods, ListStreams) match only
// grants under the special "_machine" resources string; requests that do address one
// match only grants naming that resource. Exemptions for connection-plumbing methods
// are the caller's responsibility.
func (ra *userPermsAuthorizer) allowed(id rdkgrpc.Identity, fullMethod, resourceName string) bool {
	resources := ra.permsFor(id)[fullMethod]
	if resourceName == "" {
		return resources[machineResource]
	}
	return resources[resourceName]
}

// authorize returns nil if the user in ctx may invoke fullMethod.
func (ra *userPermsAuthorizer) authorize(ctx context.Context, fullMethod string, req interface{}) error {
	if isExemptMethod(fullMethod) {
		return nil
	}
	resourceName := resourceNameFromRequest(fullMethod, req)
	id := rdkgrpc.IdentityFromContext(ctx)
	if ra.allowed(id, fullMethod, resourceName) {
		return nil
	}
	logUnauthorized(ra.logger, id, fullMethod, resourceName)
	return status.Errorf(codes.PermissionDenied,
		"user is not authorized to invoke %q on resource %q", fullMethod, resourceName)
}

// logUnauthorized emits a structured warning for a denied request, identifying the
// method, the resource it addressed, and the caller's authorization method (type and ID,
// plus e-mail for an app-user-id caller). Pass no resource for the stream-creation gate,
// which fires before the resource is known; a resource-scoped denial always passes one
// (possibly "" for a _machine method).
func logUnauthorized(logger logging.Logger, id rdkgrpc.Identity, fullMethod string, resource ...string) {
	fields := []any{"method", fullMethod}
	if len(resource) > 0 {
		// A machine-scoped method (e.g. a RobotService method) addresses no named
		// resource; surface it as the "_machine" sentinel it was matched against.
		name := resource[0]
		if name == "" {
			name = machineResource
		}
		fields = append(fields, "resource", name)
	}
	fields = append(fields, "auth_type", id.AuthType(), "auth_id", id.AuthID())
	if id.Email != "" {
		fields = append(fields, "email", id.Email)
	}
	logger.Warnw("unauthorized request", fields...)
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

// userPermsUnaryInterceptor enforces the web service's current user_permissions on
// unary RPCs. The authorizer is looked up per request so that user_permissions
// reconfigurations apply to existing connections without a web service restart.
func (svc *webService) userPermsUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *googlegrpc.UnaryServerInfo,
	handler googlegrpc.UnaryHandler,
) (interface{}, error) {
	if ra := svc.userPermsAuth.Load(); ra != nil {
		if err := ra.authorize(ctx, info.FullMethod, req); err != nil {
			return nil, err
		}
	}
	return handler(ctx, req)
}

// userPermsStreamInterceptor enforces the web service's current user_permissions on
// streaming RPCs. Authorization happens on receipt of the stream's first message,
// since that is where the resource name lives. Streams are tracked so that a
// user_permissions reconfiguration can revoke exactly the streams whose users lost
// access.
func (svc *webService) userPermsStreamInterceptor(
	srv interface{},
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	// Don't even make an authz server stream if this is an authz-exempt method.
	if isExemptMethod(info.FullMethod) {
		return handler(srv, ss)
	}
	id := rdkgrpc.IdentityFromContext(ss.Context())
	// Reject the stream up front if the user cannot invoke this method on any resource.
	// This blocks an unauthorized user from opening the stream and receiving server-pushed
	// messages before the per-resource check (which needs the first client message) has a
	// chance to run. Users with access to some resource pass here and are still checked
	// against the specific resource on their first message.
	if ra := svc.userPermsAuth.Load(); ra != nil && !ra.allowedOnAnyResource(id, info.FullMethod) {
		logUnauthorized(ra.logger, id, info.FullMethod)
		return status.Errorf(codes.PermissionDenied,
			"user is not authorized to invoke %q", info.FullMethod)
	}
	ctx, cancel := context.WithCancel(ss.Context())
	defer cancel()
	authzStream := &authzServerStream{
		ServerStream: ss,
		svc:          svc,
		ctx:          ctx,
		cancel:       cancel,
		fullMethod:   info.FullMethod,
		id:           id,
	}
	svc.registerAuthzStream(authzStream)
	// The handler invocation spans the stream's entire lifetime, so unregistration happens
	// once the stream is closed.
	defer svc.unregisterAuthzStream(authzStream)
	return handler(srv, authzStream)
}

// authzServerStream wraps a server stream to authorize its first message and to
// support revocation when user_permissions change mid-stream.
type authzServerStream struct {
	googlegrpc.ServerStream
	svc        *webService
	ctx        context.Context
	cancel     context.CancelFunc
	fullMethod string
	id         rdkgrpc.Identity

	mu           sync.Mutex
	checked      bool
	resourceName string

	revoked atomic.Bool
}

func (ss *authzServerStream) Context() context.Context {
	return ss.ctx
}

// revoke marks the stream as unauthorized and cancels its context. Handlers
// observing the context exit promptly; any later message on the stream errors.
func (ss *authzServerStream) revoke() {
	ss.revoked.Store(true)
	ss.cancel()
}

func (ss *authzServerStream) revokedErr() error {
	return status.Errorf(codes.PermissionDenied,
		"user is no longer authorized to invoke %q", ss.fullMethod)
}

// firstMessageInfo returns whether the first message has been authorized yet and
// the resource name it carried.
func (ss *authzServerStream) firstMessageInfo() (bool, string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.checked, ss.resourceName
}

func (ss *authzServerStream) RecvMsg(m interface{}) error {
	if ss.revoked.Load() {
		return ss.revokedErr()
	}
	if err := ss.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	ss.mu.Lock()
	first := !ss.checked
	if first {
		ss.checked = true
		ss.resourceName = resourceNameFromRequest(ss.fullMethod, m)
	}
	resourceName := ss.resourceName
	ss.mu.Unlock()

	// If this is the first message and the resource/method combo is not allowed for this
	// user, revoke the stream and return a permission denied error.
	if first {
		if ra := ss.svc.userPermsAuth.Load(); ra != nil && !ra.allowed(ss.id, ss.fullMethod, resourceName) {
			logUnauthorized(ra.logger, ss.id, ss.fullMethod, resourceName)
			ss.revoke()
			return status.Errorf(codes.PermissionDenied,
				"user is not authorized to invoke %q on resource %q", ss.fullMethod, resourceName)
		}
	}
	// If this is NOT the first message, the stream was initially authorized but a
	// revocation could have made it invalid.
	if ss.revoked.Load() {
		return ss.revokedErr()
	}
	return nil
}

func (ss *authzServerStream) SendMsg(m interface{}) error {
	// Do not allow sending on a revoked stream (the permission denied error or a context
	// canceled error should eventually reach the user).
	if ss.revoked.Load() {
		return ss.revokedErr()
	}
	return ss.ServerStream.SendMsg(m)
}
