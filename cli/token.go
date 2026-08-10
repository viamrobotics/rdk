package cli

import (
	"context"
	"io"

	"github.com/pkg/errors"
)

// ErrAPIKeyLogin is returned by TokenFromCache and (*Config).Token when the
// cached credentials are an API key rather than a user login. API keys have no
// bearer access token; authenticate them with (*Config).DialOptions instead.
var ErrAPIKeyLogin = errors.New("cached credentials are an API key, which has no bearer token; use Config.DialOptions instead")

// errTokenExpired is returned by the refresh helpers when the cached token has
// expired and holds no refresh token to renew it with.
var errTokenExpired = errors.New("cached token has expired and cannot be refreshed; run `viam login`")

// TokenFromCache loads the CLI's cached credentials (written by `viam login`)
// and returns a bearer access token for authenticating with app.viam.com — for
// example as an "authorization: Bearer <token>" gRPC/HTTP header.
//
// If the cached token has expired it is refreshed against the stored token
// endpoint and the refreshed credentials are written back to the cache, so
// callers get the same auto-refresh behavior as ordinary CLI commands without
// needing a *cli.Command. This lets tools reuse a developer's `viam login`
// instead of asking for a token or an API key.
//
// Only user logins are supported; API-key logins return ErrAPIKeyLogin — use
// ConfigFromCache together with (*Config).DialOptions (or ConnectToMachine)
// for those.
func TokenFromCache(ctx context.Context) (string, error) {
	conf, err := ConfigFromCache(nil)
	if err != nil {
		return "", err
	}
	return conf.Token(ctx)
}

// Token returns a bearer access token for a user login, refreshing and
// persisting the cached credentials if the current token has expired. It
// returns ErrAPIKeyLogin if the config holds an API key instead of a user
// token. TokenFromCache is the usual entry point; use this when you already
// hold a Config from ConfigFromCache.
func (conf *Config) Token(ctx context.Context) (string, error) {
	// refreshToken only uses the flow's HTTP client together with the token's
	// own token_url/client_id, so the (prod) auth-domain constants baked into
	// newCLIAuthFlow are not consulted here.
	newToken, err := conf.refreshTokenIfExpired(ctx, newCLIAuthFlow(io.Discard, true))
	if err != nil {
		if errors.Is(err, ErrAPIKeyLogin) || errors.Is(err, errTokenExpired) {
			return "", err
		}
		return "", errors.Wrap(err, "refreshing expired token (run `viam login` to re-authenticate)")
	}
	return newToken.AccessToken, nil
}

// refreshTokenIfExpired returns the config's cached user token, refreshing it
// against the given auth flow and persisting the result to the cache when it
// has expired. It returns ErrAPIKeyLogin when the config holds an API key
// rather than a user login, and errTokenExpired when the token has expired but
// carries no refresh token. This is the single place the CLI renews a cached
// login, shared by (*Config).Token and the connect/ensureLoggedIn paths.
func (conf *Config) refreshTokenIfExpired(ctx context.Context, flow *authFlow) (*token, error) {
	authToken, ok := conf.Auth.(*token)
	if !ok {
		return nil, ErrAPIKeyLogin
	}

	if !authToken.isExpired() {
		return authToken, nil
	}
	if !authToken.canRefresh() {
		return nil, errTokenExpired
	}

	newToken, err := flow.refreshToken(ctx, authToken)
	if err != nil {
		return nil, err
	}

	conf.Auth = newToken
	if err := storeConfigToCache(conf); err != nil {
		return nil, errors.Wrap(err, "caching refreshed token")
	}
	return newToken, nil
}

// ensureFreshToken refreshes and persists an expired cached user token so that
// DialOptions builds authentication material from a valid token. API-key
// configs need no refresh and are left untouched.
func (conf *Config) ensureFreshToken(ctx context.Context, flow *authFlow) error {
	_, err := conf.refreshTokenIfExpired(ctx, flow)
	if errors.Is(err, ErrAPIKeyLogin) {
		return nil
	}
	return err
}
