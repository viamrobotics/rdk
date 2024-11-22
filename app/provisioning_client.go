package app

import (
	"context"

	pb "go.viam.com/api/provisioning/v1"
	"go.viam.com/utils/rpc"
)

// ProvisioningInfo holds provisioning info.
type ProvisioningInfo struct {
	FragmentID   string
	Model        string
	Manufacturer string
}

func provisioningInfoFromProto(info *pb.ProvisioningInfo) *ProvisioningInfo {
	return &ProvisioningInfo{
		FragmentID:   info.FragmentId,
		Model:        info.Model,
		Manufacturer: info.Manufacturer,
	}
}

// NetworkInfo holds network information.
type NetworkInfo struct {
	Type      string
	SSID      string
	Security  string
	Signal    int32
	Connected bool
	LastError string
}

func networkInfoFromProto(info *pb.NetworkInfo) *NetworkInfo {
	return &NetworkInfo{
		Type:      info.Type,
		SSID:      info.Ssid,
		Security:  info.Security,
		Signal:    info.Signal,
		Connected: info.Connected,
		LastError: info.LastError,
	}
}

// GetSmartMachineStatusResponse contains smart machine status information.
type GetSmartMachineStatusResponse struct {
	ProvisioningInfo           *ProvisioningInfo
	HasSmartMachineCredentials bool
	IsOnline                   bool
	LastestConnectionAttempt   *NetworkInfo
	Errors                     []string
}

func getSmartMachineStatusResponseFromProto(resp *pb.GetSmartMachineStatusResponse) *GetSmartMachineStatusResponse {
	return &GetSmartMachineStatusResponse{
		ProvisioningInfo:           provisioningInfoFromProto(resp.ProvisioningInfo),
		HasSmartMachineCredentials: resp.HasSmartMachineCredentials,
		IsOnline:                   resp.IsOnline,
		LastestConnectionAttempt:   networkInfoFromProto(resp.LatestConnectionAttempt),
		Errors:                     resp.Errors,
	}
}

// CloudConfig is the minimal config to create a /etc/viam.json, containing the smart machine's part ID and secret.
type CloudConfig struct {
	ID         string
	Secret     string
	AppAddress string
}

func cloudConfigToProto(config *CloudConfig) *pb.CloudConfig {
	return &pb.CloudConfig{
		Id:         config.ID,
		Secret:     config.Secret,
		AppAddress: config.AppAddress,
	}
}

// ProvisioningClient is a gRPC client for method calls to the Provisioning API.
type ProvisioningClient struct {
	client pb.ProvisioningServiceClient
}

// NewProvisioningClient constructs a new ProvisioningClient using the connection passed in by the ViamClient.
func NewProvisioningClient(conn rpc.ClientConn) *ProvisioningClient {
	return &ProvisioningClient{client: pb.NewProvisioningServiceClient(conn)}
}

// GetSmartMachineStatus is for retrieving the status of the smart machine including networking.
func (c *ProvisioningClient) GetSmartMachineStatus(ctx context.Context) (*GetSmartMachineStatusResponse, error) {
	resp, err := c.client.GetSmartMachineStatus(ctx, &pb.GetSmartMachineStatusRequest{})
	if err != nil {
		return nil, err
	}
	return getSmartMachineStatusResponseFromProto(resp), nil
}

// SetNetworkCredentials is to set the wifi credentials.
func (c *ProvisioningClient) SetNetworkCredentials(ctx context.Context, credentialsType, ssid, psk string) error {
	_, err := c.client.SetNetworkCredentials(ctx, &pb.SetNetworkCredentialsRequest{
		Type: credentialsType,
		Ssid: ssid,
		Psk:  psk,
	})
	return err
}

// SetSmartMachineCredentials is to set the smart machine credentials.
func (c *ProvisioningClient) SetSmartMachineCredentials(ctx context.Context, cloud *CloudConfig) error {
	_, err := c.client.SetSmartMachineCredentials(ctx, &pb.SetSmartMachineCredentialsRequest{
		Cloud: cloudConfigToProto(cloud),
	})
	return err
}

// GetNetworkList is to retrieve the list of networks that are visible to the smart machine.
func (c *ProvisioningClient) GetNetworkList(ctx context.Context) ([]*NetworkInfo, error) {
	resp, err := c.client.GetNetworkList(ctx, &pb.GetNetworkListRequest{})
	if err != nil {
		return nil, err
	}
	var networks []*NetworkInfo
	for _, network := range resp.Networks {
		networks = append(networks, networkInfoFromProto(network))
	}
	return networks, nil
}
