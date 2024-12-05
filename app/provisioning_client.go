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

// NetworkInfo holds network information.
type NetworkInfo struct {
	Type      string
	SSID      string
	Security  string
	Signal    int
	Connected bool
	LastError string
}

// GetSmartMachineStatusResponse contains smart machine status information.
type GetSmartMachineStatusResponse struct {
	ProvisioningInfo           *ProvisioningInfo
	HasSmartMachineCredentials bool
	IsOnline                   bool
	LastestConnectionAttempt   *NetworkInfo
	Errors                     []string
}

// CloudConfig is the minimal config to create a /etc/viam.json, containing the smart machine's part ID and secret.
type CloudConfig struct {
	ID         string
	Secret     string
	AppAddress string
}

// ProvisioningClient is a gRPC client for method calls to the Provisioning API.
type ProvisioningClient struct {
	client pb.ProvisioningServiceClient
}

func newProvisioningClient(conn rpc.ClientConn) *ProvisioningClient {
	return &ProvisioningClient{client: pb.NewProvisioningServiceClient(conn)}
}

// GetSmartMachineStatus gets the status of the smart machine including networking.
func (c *ProvisioningClient) GetSmartMachineStatus(ctx context.Context) (*GetSmartMachineStatusResponse, error) {
	resp, err := c.client.GetSmartMachineStatus(ctx, &pb.GetSmartMachineStatusRequest{})
	if err != nil {
		return nil, err
	}
	return getSmartMachineStatusResponseFromProto(resp), nil
}

// SetNetworkCredentials sets the wifi credentials.
func (c *ProvisioningClient) SetNetworkCredentials(ctx context.Context, credentialsType, ssid, psk string) error {
	_, err := c.client.SetNetworkCredentials(ctx, &pb.SetNetworkCredentialsRequest{
		Type: credentialsType,
		Ssid: ssid,
		Psk:  psk,
	})
	return err
}

// SetSmartMachineCredentials sets the smart machine credentials.
func (c *ProvisioningClient) SetSmartMachineCredentials(ctx context.Context, cloud *CloudConfig) error {
	_, err := c.client.SetSmartMachineCredentials(ctx, &pb.SetSmartMachineCredentialsRequest{
		Cloud: cloudConfigToProto(cloud),
	})
	return err
}

// GetNetworkList gets the list of networks that are visible to the smart machine.
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

func provisioningInfoFromProto(info *pb.ProvisioningInfo) *ProvisioningInfo {
	if info == nil {
		return nil
	}
	return &ProvisioningInfo{
		FragmentID:   info.FragmentId,
		Model:        info.Model,
		Manufacturer: info.Manufacturer,
	}
}

func networkInfoFromProto(info *pb.NetworkInfo) *NetworkInfo {
	if info == nil {
		return nil
	}
	return &NetworkInfo{
		Type:      info.Type,
		SSID:      info.Ssid,
		Security:  info.Security,
		Signal:    int(info.Signal),
		Connected: info.Connected,
		LastError: info.LastError,
	}
}

func getSmartMachineStatusResponseFromProto(resp *pb.GetSmartMachineStatusResponse) *GetSmartMachineStatusResponse {
	if resp == nil {
		return nil
	}
	return &GetSmartMachineStatusResponse{
		ProvisioningInfo:           provisioningInfoFromProto(resp.ProvisioningInfo),
		HasSmartMachineCredentials: resp.HasSmartMachineCredentials,
		IsOnline:                   resp.IsOnline,
		LastestConnectionAttempt:   networkInfoFromProto(resp.LatestConnectionAttempt),
		Errors:                     resp.Errors,
	}
}

func cloudConfigToProto(config *CloudConfig) *pb.CloudConfig {
	if config == nil {
		return nil
	}
	return &pb.CloudConfig{
		Id:         config.ID,
		Secret:     config.Secret,
		AppAddress: config.AppAddress,
	}
}
