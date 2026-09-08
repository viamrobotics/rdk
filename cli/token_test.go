package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

// makeIDToken builds an unsigned JWT whose claims carry the given email/sub, so
// refreshToken -> buildToken -> userDataFromIDToken can extract a user from it.
func makeIDToken(t *testing.T, email, sub string) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		test.That(t, err, test.ShouldBeNil)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := enc(map[string]string{"alg": "none", "typ": "JWT"})
	payload := enc(map[string]string{"email": email, "sub": sub})
	return header + "." + payload + "."
}

func TestConfigTokenNotExpired(t *testing.T) {
	conf := &Config{Auth: &token{
		AccessToken: "live-access-token",
		TokenType:   tokenTypeUserOAuthToken,
		ExpiresAt:   time.Now().Add(time.Hour),
	}}

	got, err := conf.Token(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "live-access-token")
}

func TestConfigTokenAPIKey(t *testing.T) {
	conf := &Config{Auth: &apiKey{KeyID: "abc", KeyCrypto: "def"}}

	_, err := conf.Token(context.Background())
	test.That(t, err, test.ShouldBeError, ErrAPIKeyLogin)
}

func TestConfigTokenExpiredNoRefresh(t *testing.T) {
	conf := &Config{Auth: &token{
		AccessToken: "stale",
		TokenType:   tokenTypeUserOAuthToken,
		ExpiresAt:   time.Now().Add(-time.Hour),
		// no refresh_token/token_url/client_id -> canRefresh() is false
	}}

	_, err := conf.Token(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "viam login")
}

func TestConfigTokenRefreshAndPersist(t *testing.T) {
	// Redirect the CLI cache to a temp dir so the refreshed token is persisted
	// there instead of the developer's real ~/.viam.
	origViamDotDir := utils.ViamDotDir
	utils.ViamDotDir = t.TempDir()
	t.Cleanup(func() { utils.ViamDotDir = origViamDotDir })

	var hits atomic.Int32
	var gotGrant, gotRefresh string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		test.That(t, r.ParseForm(), test.ShouldBeNil)
		gotGrant = r.Form.Get("grant_type")
		gotRefresh = r.Form.Get("refresh_token")
		resp := tokenResponse{
			AccessToken:  "fresh-access-token",
			RefreshToken: "fresh-refresh-token",
			IDToken:      makeIDToken(t, "who@example.com", "user-123"),
			ExpiresIn:    3600,
			TokenType:    tokenTypeUserOAuthToken,
		}
		test.That(t, json.NewEncoder(w).Encode(resp), test.ShouldBeNil)
	}))
	defer srv.Close()

	conf := &Config{
		BaseURL: "https://app.viam.com:443",
		Auth: &token{
			AccessToken:  "stale-access-token",
			RefreshToken: "old-refresh-token",
			TokenType:    tokenTypeUserOAuthToken,
			TokenURL:     srv.URL,
			ClientID:     "client-123",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	}

	got, err := conf.Token(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "fresh-access-token")
	test.That(t, hits.Load(), test.ShouldEqual, 1)
	test.That(t, gotGrant, test.ShouldEqual, "refresh_token")
	test.That(t, gotRefresh, test.ShouldEqual, "old-refresh-token")

	// The refreshed token must have been written back to the cache: reloading
	// it and asking again returns the fresh token without a second refresh.
	reloaded, err := ConfigFromCache(nil)
	test.That(t, err, test.ShouldBeNil)
	got2, err := reloaded.Token(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got2, test.ShouldEqual, "fresh-access-token")
	test.That(t, hits.Load(), test.ShouldEqual, 1)
}

func TestTokenFromCache(t *testing.T) {
	origViamDotDir := utils.ViamDotDir
	utils.ViamDotDir = t.TempDir()
	t.Cleanup(func() { utils.ViamDotDir = origViamDotDir })

	conf := &Config{
		BaseURL: "https://app.viam.com:443",
		Auth: &token{
			AccessToken: "cached-access-token",
			TokenType:   tokenTypeUserOAuthToken,
			ExpiresAt:   time.Now().Add(time.Hour),
			// User.Email must be set for ConfigFromCache to parse it as a token.
			User: userData{Email: "who@example.com"},
		},
	}
	test.That(t, storeConfigToCache(conf), test.ShouldBeNil)

	got, err := TokenFromCache(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "cached-access-token")
}
