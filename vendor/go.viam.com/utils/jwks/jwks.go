// Package jwks provides helpers for working with json key sets.
package jwks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// KeySet represents json key set object, a collection of jwk.Key objects.
// See jwk docs. github.com/lestrrat-go/jwx/jwk.
type KeySet jwk.Set

// KeyProvider provides an interface to lookup keys based on a key ID.
// Providers may have a background process to refresh keys and allows
// it to be closed.
type KeyProvider interface {
	// allow users to stop any background process in a key provider.
	io.Closer

	// LookupKey should return a public key based on the given key ID. Return an error if not
	// found or any other error.
	LookupKey(ctx context.Context, kid, alg string) (interface{}, error)

	// Fetch returns the full KeySet as a cloned keyset, any modifcations are only applied locally.
	Fetch(ctx context.Context) (KeySet, error)
}

// ParseKeySet parses a JSON keyset string into a KeySet.
func ParseKeySet(input string) (KeySet, error) {
	return jwk.ParseString(input)
}

// cachingKeyProvider is a key provider that looks up jwk's by their kid through the
// configured jwksURI. It auto refreshes in the background and caches the keys found.
type cachingKeyProvider struct {
	cancel  context.CancelFunc
	ar      *jwk.AutoRefresh
	jwksURI string
}

// Stop cancels the auto refresh.
func (cp *cachingKeyProvider) Close() error {
	cp.cancel()
	return nil
}

func (cp *cachingKeyProvider) LookupKey(ctx context.Context, kid, alg string) (interface{}, error) {
	// loads keys from cache or refreshes if needed.
	keyset, err := cp.ar.Fetch(ctx, cp.jwksURI)
	if err != nil {
		return nil, err
	}

	return publicKeyFromKeySet(keyset, kid, alg)
}

func (cp *cachingKeyProvider) Fetch(ctx context.Context) (KeySet, error) {
	// loads keys from cache or refreshes if needed.
	keyset, err := cp.ar.Fetch(ctx, cp.jwksURI)
	if err != nil {
		return nil, err
	}

	return keyset.Clone()
}

// ensure interface is met.
var _ KeyProvider = &cachingKeyProvider{}

// NewCachingOIDCJWKKeyProvider creates a CachingKeyProvider based on the issuer url
// base domain and starts the auto refresh. Call CachingKeyProvider.Stop() to stop any
// background goroutines.
func NewCachingOIDCJWKKeyProvider(ctx context.Context, issuer string) (KeyProvider, error) {
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpClient := &http.Client{
		Transport: httpTransport,
	}
	defer httpTransport.CloseIdleConnections()

	wellKnown := strings.TrimSuffix(issuer, "/") + oidc.DiscoveryEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}
	discoveryConfig := new(oidc.DiscoveryConfiguration)
	err = httphelper.HttpRequest(httpClient, req, &discoveryConfig)
	if err != nil {
		return nil, err
	}
	if discoveryConfig.Issuer != issuer {
		return nil, oidc.ErrIssuerInvalid
	}

	ctx, cancel := context.WithCancel(ctx)

	ar := jwk.NewAutoRefresh(ctx)

	// Tell *jwk.AutoRefresh that we only want to refresh this JWKS
	// when it needs to (based on Cache-Control or Expires header from
	// the HTTP response). If the calculated minimum refresh interval is less
	// than 15 minutes, don't go refreshing any earlier than 15 minutes.
	ar.Configure(discoveryConfig.JwksURI, jwk.WithMinRefreshInterval(15*time.Minute))

	// Refresh the JWKS once before we start our service.
	if _, err := ar.Refresh(ctx, discoveryConfig.JwksURI); err != nil {
		cancel()
		return nil, err
	}

	return &cachingKeyProvider{
		cancel:  cancel,
		ar:      ar,
		jwksURI: discoveryConfig.JwksURI,
	}, nil
}

// wraps a static KeySet.
type staticKeySet struct {
	keyset KeySet
}

// ensure interface is met.
var _ KeyProvider = &staticKeySet{}

func (p *staticKeySet) LookupKey(ctx context.Context, kid, alg string) (interface{}, error) {
	return publicKeyFromKeySet(p.keyset, kid, alg)
}

func (p *staticKeySet) Close() error {
	return nil
}

func (p *staticKeySet) Fetch(ctx context.Context) (KeySet, error) {
	// clone to avoid any consumers making changes to the underlying keyset.
	return p.keyset.Clone()
}

// NewStaticJWKKeyProvider create static key provider based on the keyset given.
func NewStaticJWKKeyProvider(keyset KeySet) KeyProvider {
	return &staticKeySet{
		keyset: keyset,
	}
}

func publicKeyFromKeySet(keyset KeySet, kid, alg string) (interface{}, error) {
	key, ok := keyset.LookupKeyID(kid)
	if !ok {
		return nil, errors.New("kid header does not exist in keyset")
	}

	if key.Algorithm() != alg {
		return nil, errors.New("key from kid has different signing alg")
	}

	var pubKey interface{}
	if err := key.Raw(&pubKey); err != nil {
		return nil, fmt.Errorf("error getting raw key: %w", err)
	}
	return pubKey, nil
}
