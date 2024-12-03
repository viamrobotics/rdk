package app

import (
	"context"
	"testing"

	pb "go.viam.com/api/provisioning/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

const (
	hasSmartMachineCredentials = true
	isOnline                   = true
	errorString                = "error"
	modelStr                   = "model"
	manufacturer               = "manufacturer"
	networkType                = "network_type"
	ssid                       = "ssid"
	security                   = "security"
	signal                     = 4
	connected                  = true
	credentialsType            = "credentials_type"
	psk                        = "psk"
)

var (
	provisioningInfo = ProvisioningInfo{
		FragmentID:   fragmentID,
		Model:        modelStr,
		Manufacturer: manufacturer,
	}
	networkInfo = NetworkInfo{
		Type:      networkType,
		SSID:      ssid,
		Security:  security,
		Signal:    signal,
		Connected: connected,
		LastError: errorString,
	}
	pbNetworkInfo = pb.NetworkInfo{
		Type:      networkInfo.Type,
		Ssid:      networkInfo.SSID,
		Security:  networkInfo.Security,
		Signal:    int32(networkInfo.Signal),
		Connected: networkInfo.Connected,
		LastError: networkInfo.LastError,
	}
	errorList                     = []string{errorString}
	getSmartMachineStatusResponse = GetSmartMachineStatusResponse{
		ProvisioningInfo:           &provisioningInfo,
		HasSmartMachineCredentials: hasSmartMachineCredentials,
		IsOnline:                   isOnline,
		LastestConnectionAttempt:   &networkInfo,
		Errors:                     errorList,
	}
	cloudConfig = CloudConfig{
		ID:     partID,
		Secret: secret,
	}
)

func createProvisioningGrpcClient() *inject.ProvisioningServiceClient {
	return &inject.ProvisioningServiceClient{}
}

func TestProvisioningClient(t *testing.T) {
	grpcClient := createProvisioningGrpcClient()
	client := ProvisioningClient{client: grpcClient}

	t.Run("GetSmartMachineStatus", func(t *testing.T) {
		pbResponse := pb.GetSmartMachineStatusResponse{
			ProvisioningInfo: &pb.ProvisioningInfo{
				FragmentId:   getSmartMachineStatusResponse.ProvisioningInfo.FragmentID,
				Model:        getSmartMachineStatusResponse.ProvisioningInfo.Model,
				Manufacturer: getSmartMachineStatusResponse.ProvisioningInfo.Manufacturer,
			},
			HasSmartMachineCredentials: getSmartMachineStatusResponse.HasSmartMachineCredentials,
			IsOnline:                   getSmartMachineStatusResponse.IsOnline,
			LatestConnectionAttempt:    &pbNetworkInfo,
			Errors:                     getSmartMachineStatusResponse.Errors,
		}
		grpcClient.GetSmartMachineStatusFunc = func(
			ctx context.Context, in *pb.GetSmartMachineStatusRequest, opts ...grpc.CallOption,
		) (*pb.GetSmartMachineStatusResponse, error) {
			return &pbResponse, nil
		}
		resp, err := client.GetSmartMachineStatus(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &getSmartMachineStatusResponse)
	})

	t.Run("SetNetworkCredentials", func(t *testing.T) {
		grpcClient.SetNetworkCredentialsFunc = func(
			ctx context.Context, in *pb.SetNetworkCredentialsRequest, opts ...grpc.CallOption,
		) (*pb.SetNetworkCredentialsResponse, error) {
			test.That(t, in.Type, test.ShouldEqual, credentialsType)
			test.That(t, in.Ssid, test.ShouldEqual, ssid)
			test.That(t, in.Psk, test.ShouldEqual, psk)
			return &pb.SetNetworkCredentialsResponse{}, nil
		}
		err := client.SetNetworkCredentials(context.Background(), credentialsType, ssid, psk)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("SetSmartMachineCredentials", func(t *testing.T) {
		grpcClient.SetSmartMachineCredentialsFunc = func(
			ctx context.Context, in *pb.SetSmartMachineCredentialsRequest, opts ...grpc.CallOption,
		) (*pb.SetSmartMachineCredentialsResponse, error) {
			test.That(t, in.Cloud, test.ShouldResemble, cloudConfigToProto(&cloudConfig))
			return &pb.SetSmartMachineCredentialsResponse{}, nil
		}
		err := client.SetSmartMachineCredentials(context.Background(), &cloudConfig)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetNetworkList", func(t *testing.T) {
		expectedNetworks := []*NetworkInfo{&networkInfo}
		grpcClient.GetNetworkListFunc = func(
			ctx context.Context, in *pb.GetNetworkListRequest, opts ...grpc.CallOption,
		) (*pb.GetNetworkListResponse, error) {
			return &pb.GetNetworkListResponse{
				Networks: []*pb.NetworkInfo{&pbNetworkInfo},
			}, nil
		}
		resp, err := client.GetNetworkList(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedNetworks)
	})
}
