package web

import (
	"context"
	"strings"

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
// viam-server endpoints. Enforcing roles on them would prevent clients from ever
// authenticating.
var exemptMethodPrefixes = []string{
	"/proto.rpc.v1.",
	"/proto.rpc.webrtc.v1.",
	"/grpc.reflection.",
}

// permSet maps an allowed method to the set of resource names it may be invoked on.
type permSet map[string]map[string]bool

// rolesAuthorizer decides whether users may invoke endpoints based on the
// user_permissions section of the machine config. A nil *rolesAuthorizer allows
// everything.
type rolesAuthorizer struct {
	apiKeyPerms  map[string]permSet // keyed by API key ID
	emailPerms   map[string]permSet // keyed by e-mail address
	defaultPerms permSet            // permissions of the "default" user; nil if none
	logger       logging.Logger
}

// newRolesAuthorizer returns an authorizer for the given user permissions, or nil
// if userPerms is empty (all users unrestricted).
func newRolesAuthorizer(userPerms []config.UserPermission, logger logging.Logger) *rolesAuthorizer {
	if len(userPerms) == 0 {
		return nil
	}
	ra := &rolesAuthorizer{
		apiKeyPerms: map[string]permSet{},
		emailPerms:  map[string]permSet{},
		logger:      logger,
	}

	restricted := map[config.User]bool{}
	for _, userPerm := range userPerms {
		perms := permSet{}
		for _, perm := range userPerm.Permissions {
			for _, method := range perm.AllowedMethods {
				if perms[method] == nil {
					perms[method] = map[string]bool{}
				}
				for _, res := range perm.Resources {
					perms[method][res] = true
				}
			}
		}

		user := userPerm.User
		var permsByID map[string]permSet
		switch user.Type {
		case config.UserTypeAPIKeyID:
			permsByID = ra.apiKeyPerms
		case config.UserTypeEmail:
			permsByID = ra.emailPerms
		case config.UserTypeDefault:
			if ra.defaultPerms != nil || restricted[user] {
				ra.logger.Error(
					"multiple user_permissions entries for default user; fully restricting until collision is fixed")
				ra.defaultPerms = permSet{}
				restricted[user] = true
				continue
			}
			ra.defaultPerms = perms
			continue
		default:
			ra.logger.Errorw("unknown user type in user_permissions; ignoring user",
				"type", user.Type, "id", user.ID)
			continue
		}
		if _, seen := permsByID[user.ID]; seen || restricted[user] {
			ra.logger.Errorw(
				"multiple user_permissions entries for user; fully restricting user until collision is fixed",
				"type", user.Type, "id", user.ID)
			// An empty permSet fully restricts: the user matches an entry, so they do
			// not fall back to the default user's permissions.
			permsByID[user.ID] = permSet{}
			restricted[user] = true
			continue
		}
		permsByID[user.ID] = perms
	}
	return ra
}

// permsForUser returns the permissions of the user in ctx, or nil if the user is
// unauthenticated or unknown with no default user configured.
func (ra *rolesAuthorizer) permsForUser(ctx context.Context) permSet {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		// Unauthenticated (insecure connection): fully restrict.
		return nil
	}
	if perms, ok := ra.apiKeyPerms[entity.Entity]; ok {
		return perms
	}
	if perms, ok := ra.emailPerms[entity.Entity]; ok {
		return perms
	}
	if email := emailFromContext(ctx); email != "" {
		if perms, ok := ra.emailPerms[email]; ok {
			return perms
		}
	}
	return ra.defaultPerms
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

// authorize returns nil if the user in ctx may invoke fullMethod. req may be nil for
// streaming RPCs whose first message has not yet been received; a nil req performs
// only the method-level (any-resource) check.
func (ra *rolesAuthorizer) authorize(ctx context.Context, fullMethod string, req interface{}) error {
	for _, prefix := range exemptMethodPrefixes {
		if strings.HasPrefix(fullMethod, prefix) {
			return nil
		}
	}

	perms := ra.permsForUser(ctx)
	resources := perms[fullMethod]
	resourceName := resourceNameFromRequest(fullMethod, req)
	if resourceName == "" {
		// Methods not associated with a single resource (e.g. RobotService methods,
		// ListStreams) are allowed when granted under any resource name.
		if len(resources) != 0 {
			return nil
		}
	} else if resources[resourceName] {
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

// StreamInterceptor enforces configured roles on streaming RPCs. The method-level
// check happens up front; resource-level authorization happens on receipt of the
// stream's first message, since that is where the resource name lives.
func (ra *rolesAuthorizer) StreamInterceptor(
	srv interface{},
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	if err := ra.authorize(ss.Context(), info.FullMethod, nil); err != nil {
		return err
	}
	return handler(srv, &authzServerStream{ServerStream: ss, ra: ra, fullMethod: info.FullMethod})
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
