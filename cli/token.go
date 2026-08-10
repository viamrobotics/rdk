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
	authToken, ok := conf.Auth.(*token)
	if !ok {
		return "", ErrAPIKeyLogin
	}

	if !authToken.isExpired() {
		return authToken.AccessToken, nil
	}
	if !authToken.canRefresh() {
		return "", errors.New("cached token has expired and cannot be refreshed; run `viam login`")
	}

	// refreshToken only uses the flow's HTTP client together with the token's
	// own token_url/client_id, so the (prod) auth-domain constants baked into
	// newCLIAuthFlow are not consulted here.
	newToken, err := newCLIAuthFlow(io.Discard, true).refreshToken(ctx, authToken)
	if err != nil {
		return "", errors.Wrap(err, "refreshing expired token (run `viam login` to re-authenticate)")
	}

	conf.Auth = newToken
	if err := storeConfigToCache(conf); err != nil {
		return "", errors.Wrap(err, "caching refreshed token")
	}
	return newToken.AccessToken, nil
}
