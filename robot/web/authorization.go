package web

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	jwt "github.com/golang-jwt/jwt/v4"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// exemptMethodPrefixes are gRPC service namespaces that are connection plumbing
// (authentication handshakes, WebRTC signaling, and reflection) rather than
// viam-server endpoints. Enforcing user_permissions on them would prevent clients from ever
// authenticating.
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

	addStreamMethod = "/proto.stream.v1.StreamService/AddStream"
)

// permSet maps an allowed method to the set of resource names it may be invoked on.
type permSet map[string]map[string]bool

// identity is the authorization-relevant identity of a connected client.
type identity struct {
	// entity is the JWT subject: an API key ID or a FusionAuth user ID. Empty if the
	// client is unauthenticated.
	entity string
	// email is the e-mail from the token's auth metadata claim, if any.
	email string
}

func (id identity) authenticated() bool {
	return id.entity != ""
}

// identityFromContext extracts the caller's identity from a request context. The
// returned identity is zero-valued for unauthenticated clients.
func identityFromContext(ctx context.Context) identity {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return identity{}
	}
	return identity{entity: entity.Entity, email: emailFromContext(ctx)}
}

// emailFromContext extracts the e-mail from the auth metadata claim of the request's
// bearer token. The token's signature was already verified by the auth interceptor,
// so parsing it unverified here is safe.
func emailFromContext(ctx context.Context) string {
	token, err := rpc.TokenFromContext(ctx)
	if err != nil {
		return ""
	}
	var claims rpc.JWTClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return ""
	}
	return claims.Metadata()["email"]
}

// userPermsAuthorizer decides whether users may invoke endpoints based on the
// user_permissions section of the machine config. A nil *userPermsAuthorizer allows
// everything.
type userPermsAuthorizer struct {
	apiKeyPerms  map[string]permSet // keyed by API key ID
	emailPerms   map[string]permSet // keyed by e-mail address
	defaultPerms permSet            // permissions of the "default" user; nil if none
	logger       logging.Logger
}

// newUserPermsAuthorizer returns an authorizer for the given user permissions, or nil
// if userPerms is empty (all users unrestricted).
func newUserPermsAuthorizer(userPerms []config.UserPermission, logger logging.Logger) *userPermsAuthorizer {
	if len(userPerms) == 0 {
		return nil
	}
	ra := &userPermsAuthorizer{
		apiKeyPerms: map[string]permSet{},
		emailPerms:  map[string]permSet{},
		logger:      logger,
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
		var permsByID map[string]permSet
		switch user.Type {
		case config.UserTypeAPIKeyID:
			permsByID = ra.apiKeyPerms
		case config.UserTypeEmail:
			permsByID = ra.emailPerms
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
		if _, seen := permsByID[user.ID]; seen {
			ra.logger.Errorw(
				"multiple user_permissions entries for user; fully restricting user until collision is fixed",
				"type", user.Type, "id", user.ID)
			// An empty permSet fully restricts.
			permsByID[user.ID] = permSet{}
			continue
		}
		permsByID[user.ID] = buildPermSet(userPerm.Permissions)
	}
	return ra
}

// permsFor returns the permissions of the given identity, or nil if the identity is
// unauthenticated or unknown with no default user configured.
func (ra *userPermsAuthorizer) permsFor(id identity) permSet {
	if !id.authenticated() {
		// Unauthenticated (insecure connection): fully restrict.
		return nil
	}
	if perms, ok := ra.apiKeyPerms[id.entity]; ok {
		return perms
	}
	// The auth entity is an API key ID or a FusionAuth user ID, never an e-mail;
	// e-mails are matched via the token's auth metadata claim.
	if id.email != "" {
		if perms, ok := ra.emailPerms[id.email]; ok {
			return perms
		}
	}
	return ra.defaultPerms
}

// allowed returns whether the given identity may invoke fullMethod. Requests that do
// not address a named resource (e.g. RobotService methods, ListStreams) match only
// grants under the special "_machine" resources string; requests that do address one
// match only grants naming that resource.
func (ra *userPermsAuthorizer) allowed(id identity, fullMethod, resourceName string) bool {
	if isExemptMethod(fullMethod) {
		return true
	}
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
	if ra.allowed(identityFromContext(ctx), fullMethod, resourceName) {
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
	if isExemptMethod(info.FullMethod) {
		return handler(srv, ss)
	}
	ctx, cancel := context.WithCancel(ss.Context())
	defer cancel()
	authzStream := &authzServerStream{
		ServerStream: ss,
		svc:          svc,
		ctx:          ctx,
		cancel:       cancel,
		fullMethod:   info.FullMethod,
		id:           identityFromContext(ss.Context()),
	}
	svc.registerAuthzStream(authzStream)
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
	id         identity

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

	if first {
		if ra := ss.svc.userPermsAuth.Load(); ra != nil && !ra.allowed(ss.id, ss.fullMethod, resourceName) {
			ss.revoke()
			return status.Errorf(codes.PermissionDenied,
				"user is not authorized to invoke %q on resource %q", ss.fullMethod, resourceName)
		}
	}
	if ss.revoked.Load() {
		return ss.revokedErr()
	}
	return nil
}

func (ss *authzServerStream) SendMsg(m interface{}) error {
	if ss.revoked.Load() {
		return ss.revokedErr()
	}
	return ss.ServerStream.SendMsg(m)
}
