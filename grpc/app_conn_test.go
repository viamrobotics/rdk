package grpc

import (
	"context"
	"os"
	"sync/atomic"
	"testing"

	"go.viam.com/test"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// redirectViamDotDir points the on-disk JWT cache at a temp dir for the duration of a test.
func redirectViamDotDir(t *testing.T) {
	t.Helper()
	orig := rutils.ViamDotDir
	rutils.ViamDotDir = t.TempDir()
	t.Cleanup(func() { rutils.ViamDotDir = orig })
}

// TestJWTCache exercises the on-disk JWT cache helpers directly (no network).
func TestJWTCache(t *testing.T) {
	redirectViamDotDir(t)

	const (
		partID = "part-abc"
		host   = "app.viam.com:443"
		token  = "the-jwt-token"
	)

	t.Run("write then read round trips", func(t *testing.T) {
		test.That(t, jwtCacheWrite(partID, host, token), test.ShouldBeNil)
		got, err := jwtCacheRead(partID, host)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, token)
	})

	t.Run("JWT file is 0600 and its dir is 0700", func(t *testing.T) {
		test.That(t, jwtCacheWrite(partID, host, token), test.ShouldBeNil)
		cacheDir, jwtPath := jwtInfo(partID, host)

		fi, err := os.Stat(jwtPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fi.Mode().Perm(), test.ShouldEqual, os.FileMode(0o600))

		di, err := os.Stat(cacheDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, di.Mode().Perm(), test.ShouldEqual, os.FileMode(0o700))
	})

	t.Run("reading a missing token errors", func(t *testing.T) {
		_, err := jwtCacheRead("no-such-part", "nowhere:443")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("overwrite keeps the latest token", func(t *testing.T) {
		test.That(t, jwtCacheWrite(partID, host, "first"), test.ShouldBeNil)
		test.That(t, jwtCacheWrite(partID, host, "second"), test.ShouldBeNil)
		got, err := jwtCacheRead(partID, host)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, "second")
	})

	t.Run("cache is keyed by both partID and host", func(t *testing.T) {
		_, pathBaseline := jwtInfo(partID, host)
		_, pathOtherPart := jwtInfo("other-part", host)
		_, pathOtherHost := jwtInfo(partID, "other-host:443")
		// distinct partID or host must yield distinct files so co-located parts and the
		// app vs signaling endpoints never clobber each other.
		test.That(t, pathOtherPart, test.ShouldNotEqual, pathBaseline)
		test.That(t, pathOtherHost, test.ShouldNotEqual, pathBaseline)

		test.That(t, jwtCacheWrite(partID, host, "mine"), test.ShouldBeNil)
		test.That(t, jwtCacheWrite("other-part", host, "theirs"), test.ShouldBeNil)
		mine, err := jwtCacheRead(partID, host)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mine, test.ShouldEqual, "mine")
		theirs, err := jwtCacheRead("other-part", host)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, theirs, test.ShouldEqual, "theirs")
	})
}

// gatedAuthHandler accepts a single (entity, payload) pair, but once `broken` is set it fails every
// /Authenticate with Unavailable — simulating a token-minting (KMS) outage. Token *verification*
// happens against the server's own signing key, independent of this handler, so tokens minted
// before the break still verify — exactly the property the cache fallback relies on.
type gatedAuthHandler struct {
	broken  atomic.Bool
	entity  string
	payload string
}

func (h *gatedAuthHandler) Authenticate(_ context.Context, entity, payload string) (map[string]string, error) {
	if h.broken.Load() {
		return nil, status.Error(codes.Unavailable, "simulated minting outage")
	}
	if entity != h.entity || payload != h.payload {
		return nil, status.Error(codes.Unauthenticated, "bad credentials")
	}
	return map[string]string{}, nil
}

// newFakeAppServer stands up an in-process goutils RPC server (our stand-in for app/signaling) that
// authenticates robot-secret credentials via handler and serves an echo service behind that auth.
func newFakeAppServer(t *testing.T, handler rpc.AuthHandler) (addr string) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	server, err := rpc.NewServer(logger, rpc.WithAuthHandler(rutils.CredentialsTypeRobotSecret, handler))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, server.RegisterServiceServer(
		context.Background(),
		&echopb.EchoService_ServiceDesc,
		&echoserver.Server{},
		echopb.RegisterEchoServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	test.That(t, server.Start(), test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, server.Stop(), test.ShouldBeNil) })

	return server.InternalAddr().String()
}

func robotSecretDialOpts(entity, secret string) []rpc.DialOption {
	return []rpc.DialOption{
		rpc.WithInsecure(),
		rpc.WithEntityCredentials(entity, rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotSecret,
			Payload: secret,
		}),
	}
}

// TestAuthedDialDirectGRPC covers the two behaviors that matter: a successful auth caches a token,
// and — with minting broken — a later dial recovers by presenting that cached token.
func TestAuthedDialDirectGRPC(t *testing.T) {
	redirectViamDotDir(t)

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	const (
		partID = "part-xyz"
		entity = "secret-entity"
		secret = "secret-payload"
	)
	handler := &gatedAuthHandler{entity: entity, payload: secret}
	addr := newFakeAppServer(t, handler)
	dialOpts := robotSecretDialOpts(entity, secret)

	// --- happy path: a successful auth writes the token to disk and yields a usable conn ---
	conn, err := authedDialDirectGRPC(ctx, partID, addr, logger, dialOpts...)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn, test.ShouldNotBeNil)

	_, jwtPath := jwtInfo(partID, addr)
	fi, err := os.Stat(jwtPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fi.Size(), test.ShouldBeGreaterThan, 0)

	resp, err := echopb.NewEchoServiceClient(conn).Echo(ctx, &echopb.EchoRequest{Message: "hello"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetMessage(), test.ShouldEqual, "hello")
	test.That(t, conn.Close(), test.ShouldBeNil)

	// --- fallback: break minting, then a fresh dial must recover via the cached token ---
	handler.broken.Store(true)
	conn2, err := authedDialDirectGRPC(ctx, partID, addr, logger, dialOpts...)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn2, test.ShouldNotBeNil)

	// This RPC can only succeed if the conn is authenticated by the cached token: fresh
	// /Authenticate now fails, but the previously minted token still verifies.
	resp, err = echopb.NewEchoServiceClient(conn2).Echo(ctx, &echopb.EchoRequest{Message: "recovered"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetMessage(), test.ShouldEqual, "recovered")
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

// TestAuthedDialDirectGRPCNoCacheOnBrokenAuth verifies the degraded no-cache path: when minting is
// down and nothing was cached, the dial does not error or panic — it returns the (unauthenticated)
// connection so the caller's retry loop keeps trying, and RPCs over it simply fail.
func TestAuthedDialDirectGRPCNoCacheOnBrokenAuth(t *testing.T) {
	redirectViamDotDir(t)

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	handler := &gatedAuthHandler{entity: "e", payload: "p"}
	handler.broken.Store(true) // minting broken from the start; no token was ever cached
	addr := newFakeAppServer(t, handler)

	conn, err := authedDialDirectGRPC(ctx, "part-none", addr, logger, robotSecretDialOpts("e", "p")...)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn, test.ShouldNotBeNil)

	// no cached token + broken minting: the connection is unauthenticated, so the RPC fails.
	_, echoErr := echopb.NewEchoServiceClient(conn).Echo(ctx, &echopb.EchoRequest{Message: "hi"})
	test.That(t, echoErr, test.ShouldNotBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}
