package module

import (
	"context"
	"os"
	"testing"

	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/app"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestQueryTabularDataForResource(t *testing.T) {
	grpcClient := &inject.DataServiceClient{}
	grpcClient.TabularDataByMQLFunc = func(
		ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption,
	) (*datapb.TabularDataByMQLResponse, error) {
		test.That(t, "my_org", test.ShouldEqual, in.OrganizationId)

		return &datapb.TabularDataByMQLResponse{}, nil
	}

	os.Setenv(rutils.PrimaryOrgIDEnvVar, "my_org")
	os.Setenv(rutils.MachinePartIDEnvVar, "part")

	dataConsumer := &ResourceDataConsumer{dataClient: app.CreateDataClientWithDataServiceClient(grpcClient)}
	dataConsumer.QueryTabularDataForResource(context.Background(), "resource", nil)
}
