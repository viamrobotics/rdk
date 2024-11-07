package app

import (
	"context"
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
)

var (
	logger       = logging.NewLogger("test")
	defaultURL   = "https://app.viam.com"
	testAPIKey   = "abcdefghijklmnopqrstuv0123456789"
	testAPIKeyID = "abcd0123-ef45-gh67-ij89-klmnopqr01234567"
)

type MockConn struct{}

func (m *MockConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return nil
}

func (m *MockConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return nil, nil
}
func (m *MockConn) PeerConn() *webrtc.PeerConnection { return nil }
func (m *MockConn) Close() error                     { return nil }
func mockDialDirectGRPC(
	ctx context.Context,
	address string,
	logger utils.ZapCompatibleLogger,
	opts ...rpc.DialOption,
) (rpc.ClientConn, error) {
	return &MockConn{}, nil
}

func TestCreateViamClientWithOptions(t *testing.T) {
	urlTests := []struct {
		name      string
		baseURL   string
		entity    string
		payload   string
		expectErr bool
	}{
		{"Default URL", defaultURL, testAPIKeyID, testAPIKey, false},
		{"Default URL", defaultURL, "", "", true},
		{"Default URL", defaultURL, "", testAPIKey, true},
		{"Default URL", defaultURL, testAPIKeyID, "", true},
		{"Empty URL", "", testAPIKeyID, testAPIKey, false},
		{"Valid URL", "https://test.com", testAPIKeyID, testAPIKey, false},
		{"Invalid URL", "test", testAPIKey, testAPIKey, true},
	}
	originalDialDirectGRPC := dialDirectGRPC
	dialDirectGRPC = mockDialDirectGRPC
	defer func() { dialDirectGRPC = originalDialDirectGRPC }()
	for _, tt := range urlTests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				baseURL: tt.baseURL,
				entity:  tt.entity,
				credentials: rpc.Credentials{
					Type:    rpc.CredentialsTypeAPIKey,
					Payload: tt.payload,
				},
			}
			client, err := CreateViamClientWithOptions(context.Background(), opts, logger)
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
			if !tt.expectErr {
				if client == nil {
					t.Error("Expected a valid client, got nil")
				} else {
					client.Close()
				}
			}
		})
	}
}

func TestCreateViamClientWithAPIKeyTests(t *testing.T) {
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
			options := APIKeyOptions{
				apiKey:   tt.apiKey,
				apiKeyID: tt.apiKeyID,
			}
			client, err := CreateViamClientWithAPIKey(context.Background(), options, logger)
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
			if !tt.expectErr {
				if client == nil {
					t.Error("Expected a valid client, got nil")
				} else {
					client.Close()
				}
			}
		})
	}
}
