package cli

import (
	"net/http"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/testutils/inject"
)

func TestAppendAuthHeadersTokenAuth(t *testing.T) {
	cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, "token")

	testReq, err := http.NewRequestWithContext(cCtx.Context, http.MethodGet, "/test", nil)
	test.That(t, err, test.ShouldBeNil)

	ac.appendAuthHeaders(testReq)
	test.That(t, len(testReq.Header), test.ShouldEqual, 1)
	test.That(t, testReq.Header.Get("Authorization"), test.ShouldEqual, "Bearer "+testToken)
}

func TestAppendAuthHeadersAPIKeyAuth(t *testing.T) {
	cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, "apiKey")

	testReq, err := http.NewRequestWithContext(cCtx.Context, http.MethodGet, "/test", nil)
	test.That(t, err, test.ShouldBeNil)

	ac.appendAuthHeaders(testReq)
	test.That(t, len(testReq.Header), test.ShouldEqual, 2)
	test.That(t, testReq.Header.Get("key_id"), test.ShouldEqual, testKeyID)
	test.That(t, testReq.Header.Get("key"), test.ShouldEqual, testKeyCrypto)
}