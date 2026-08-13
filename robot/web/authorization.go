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
// viam-server endpoints. Enforcing user_permissions on them would prevent clients from ever
// authenticating.
var exemptMethodPrefixes = []string{
	"/proto.rpc.v1.",
	"/proto.rpc.webrtc.v1.",
	"/grpc.reflection.",
}

const (
	// machineResource is the special resources string granting methods not associated
	// with a single resource (e.g. RobotService methods and ListStreams). Resource
	// names must begin with a letter or number, so no real resource can claim it.
	machineResource = "_machine"
)

// permSet maps an allowed method to the set of resource names it may be invoked on.
type permSet map[string]map[string]bool

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

// permsForUser returns the permissions of the user in ctx, or nil if the user is
// unauthenticated or unknown with no default user configured.
func (ra *userPermsAuthorizer) permsForUser(ctx context.Context) permSet {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		// Unauthenticated (insecure connection): fully restrict.
		return nil
	}
	if perms, ok := ra.apiKeyPerms[entity.Entity]; ok {
		return perms
	}
	// The auth entity is an API key ID or a FusionAuth user ID, never an e-mail;
	// e-mails are matched via the token's auth metadata claim.
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

// authorize returns nil if the user in ctx may invoke fullMethod. Requests that do
// not address a named resource (e.g. RobotService methods, ListStreams) match only
// grants under the special "_machine" resources string; requests that do address one
// match only grants naming that resource.
func (ra *userPermsAuthorizer) authorize(ctx context.Context, fullMethod string, req interface{}) error {
	for _, prefix := range exemptMethodPrefixes {
		if strings.HasPrefix(fullMethod, prefix) {
			return nil
		}
	}

	perms := ra.permsForUser(ctx)
	resources := perms[fullMethod]
	resourceName := resourceNameFromRequest(fullMethod, req)
	if resourceName == "" {
		if resources[machineResource] {
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

// UnaryInterceptor enforces configured user_permissions on unary RPCs.
func (ra *userPermsAuthorizer) UnaryInterceptor(
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

// StreamInterceptor enforces configured user_permissions on streaming RPCs.
// Authorization happens on receipt of the stream's first message, since that is
// where the resource name lives.
func (ra *userPermsAuthorizer) StreamInterceptor(
	srv interface{},
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	return handler(srv, &authzServerStream{ServerStream: ss, ra: ra, fullMethod: info.FullMethod})
}

type authzServerStream struct {
	googlegrpc.ServerStream
	ra         *userPermsAuthorizer
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
