package app

import (
	"context"
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	datapb "go.viam.com/api/app/data/v1"
	setPb "go.viam.com/api/app/dataset/v1"
	syncPb "go.viam.com/api/app/datasync/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	apppb "go.viam.com/api/app/v1"
	provisioningpb "go.viam.com/api/provisioning/v1"
	"go.viam.com/test"
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

func (m *MockConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return nil, nil
}

func (m *MockConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return nil
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
		{name: "No URL", entity: testAPIKey, payload: testAPIKey, expectErr: false},
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
				BaseURL: tt.baseURL,
				Entity:  tt.entity,
				Credentials: rpc.Credentials{
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
	}
	for _, tt := range apiKeyTests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := CreateViamClientWithAPIKey(context.Background(), Options{}, tt.apiKey, tt.apiKeyID, logger)
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

func TestNewAppClients(t *testing.T) {
	originalDialDirectGRPC := dialDirectGRPC
	dialDirectGRPC = mockDialDirectGRPC
	defer func() { dialDirectGRPC = originalDialDirectGRPC }()
	opts := Options{
		BaseURL: defaultURL,
		Entity:  testAPIKey,
		Credentials: rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: testAPIKeyID,
		},
	}
	client, err := CreateViamClientWithOptions(context.Background(), opts, logger)
	test.That(t, err, test.ShouldBeNil)
	defer client.Close()

	appClient := client.AppClient()
	test.That(t, appClient, test.ShouldNotBeNil)
	test.That(t, appClient, test.ShouldHaveSameTypeAs, &AppClient{})
	test.That(t, appClient.client, test.ShouldImplement, (*apppb.AppServiceClient)(nil))

	// Testing that a second call to AppClient() returns the same instance
	appClient2 := client.AppClient()
	test.That(t, appClient2, test.ShouldNotBeNil)
	test.That(t, appClient, test.ShouldEqual, appClient2)

	billingClient := client.BillingClient()
	test.That(t, billingClient, test.ShouldNotBeNil)
	test.That(t, billingClient, test.ShouldHaveSameTypeAs, &BillingClient{})
	test.That(t, billingClient.client, test.ShouldImplement, (*apppb.BillingServiceClient)(nil))

	// Testing that a second call to Billingclient() returns the same instance
	billingClient2 := client.BillingClient()
	test.That(t, billingClient2, test.ShouldNotBeNil)
	test.That(t, billingClient, test.ShouldEqual, billingClient2)

	dataClient := client.DataClient()
	test.That(t, dataClient, test.ShouldNotBeNil)
	test.That(t, dataClient, test.ShouldHaveSameTypeAs, &DataClient{})
	test.That(t, dataClient.dataClient, test.ShouldImplement, (*datapb.DataServiceClient)(nil))
	test.That(t, dataClient.dataSyncClient, test.ShouldImplement, (*syncPb.DataSyncServiceClient)(nil))
	test.That(t, dataClient.datasetClient, test.ShouldImplement, (*setPb.DatasetServiceClient)(nil))

	// Testing that a second call to DataClient() returns the same instance
	dataClient2 := client.DataClient()
	test.That(t, dataClient2, test.ShouldNotBeNil)
	test.That(t, dataClient, test.ShouldEqual, dataClient2)

	mlTrainingClient := client.MLTrainingClient()
	test.That(t, mlTrainingClient, test.ShouldNotBeNil)
	test.That(t, mlTrainingClient, test.ShouldHaveSameTypeAs, &MLTrainingClient{})
	test.That(t, mlTrainingClient.client, test.ShouldImplement, (*mltrainingpb.MLTrainingServiceClient)(nil))

	// Testing that a second call to MLTrainingClient() returns the same instance
	mlTrainingClient2 := client.MLTrainingClient()
	test.That(t, mlTrainingClient2, test.ShouldNotBeNil)
	test.That(t, mlTrainingClient, test.ShouldEqual, mlTrainingClient2)

	provisioningClient := client.ProvisioningClient()
	test.That(t, provisioningClient, test.ShouldNotBeNil)
	test.That(t, provisioningClient, test.ShouldHaveSameTypeAs, &ProvisioningClient{})
	test.That(t, provisioningClient.client, test.ShouldImplement, (*provisioningpb.ProvisioningServiceClient)(nil))

	// Testing that a second call to ProvisioningClient() returns the same instance
	provisioningClient2 := client.ProvisioningClient()
	test.That(t, provisioningClient2, test.ShouldNotBeNil)
	test.That(t, provisioningClient, test.ShouldEqual, provisioningClient2)
}
