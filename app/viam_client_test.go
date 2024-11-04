package app

import (
	"context"
	"testing"

	"go.viam.com/rdk/logging"
)

var (
	logger             = logging.NewLogger("test")
	defaultServiceHost = "https://app.viam.com"
	testAPIKey         = "abcdefghijklmnopqrstuv0123456789"
	testAPIKeyID       = "abcd0123-ef45-gh67-ij89-klmnopqr01234567"
)

func TestCreateViamClientWithURLTests(t *testing.T) {
	urlTests := []struct {
		name      string
		baseURL   string
		expectErr bool
	}{
		{"Default URL", defaultServiceHost, false},
		{"Valid URL", "https://test.com", false},
		{"Empty URL", "", false},
		{"Invalid URL", "invalid-url", true},
	}
	for _, tt := range urlTests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := CreateViamClient(context.Background(), tt.baseURL, testAPIKey, testAPIKeyID, logger)
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
			if !tt.expectErr && client == nil {
				t.Error("Expected a valid client, got nil")
			}
		})
	}
}

func TestCreateViamClientWithApiKeyTests(t *testing.T) {
	apiKeyTests := []struct {
		name      string
		apiKey    string
		apiKeyID  string
		expectErr bool
	}{
		{"Valid API Key", testAPIKey, testAPIKeyID, false},
		{"Empty API Key", "", testAPIKeyID, true},
		{"Empty API Key ID", testAPIKey, "", true},
		{"Invalid API Key", "fake", testAPIKeyID, true},
		{"Invalid API Key ID", testAPIKey, "fake", true},
	}
	for _, tt := range apiKeyTests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := CreateViamClient(context.Background(), defaultServiceHost, tt.apiKey, tt.apiKeyID, logger)
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
			if !tt.expectErr && client == nil {
				t.Error("Expected a valid client, got nil")
			}
		})
	}
}
