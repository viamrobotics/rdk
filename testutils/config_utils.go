// Package testutils implements test utilities.
package testutils

import (
	"context"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// FakeConvertedAttributes is a helper for testing if validation works.
type FakeConvertedAttributes struct {
	Thing string
}

// Validate validates that the single fake attribute Thing exists properly
// in the struct, meant to implement the validator interface in component.go.
func (convAttr *FakeConvertedAttributes) Validate(path string) ([]string, error) {
	if convAttr.Thing == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "Thing")
	}
	return nil, nil
}

// ConfigFromJSON creates a fully-processed config from a JSON-string. This function will
// fail the current if it encounters any errors.
func ConfigFromJSON(tb testing.TB, jsonData string) *config.Config {
	tb.Helper()
	logger := logging.NewTestLogger(tb)
	conf, err := config.FromReader(context.Background(), "", strings.NewReader(jsonData), logger)
	test.That(tb, err, test.ShouldBeNil)
	return conf
}
