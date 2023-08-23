package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/urfave/cli/v2"
	datapb "go.viam.com/api/app/data/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

var (
	testEmail = "grogu@viam.com"
	testToken = "thisistheway"
)

type testWriter struct {
	messages []string
}

// Write implements io.Writer.
func (tw *testWriter) Write(b []byte) (int, error) {
	tw.messages = append(tw.messages, string(b))
	return len(b), nil
}

// setup creates a new cli.Context and appClient with fake auth and the passed
// in AppServiceClient. It also returns testWriters that capture Stdout and
// Stdin.
func setup(asc apppb.AppServiceClient, dataClient datapb.DataServiceClient) (*cli.Context, *appClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	flags := &flag.FlagSet{}
	if dataClient != nil {
		// these flags are only relevant when testing a dataClient
		flags.String(dataFlagDataType, dataTypeTabular, "")
		flags.String(dataFlagDestination, os.TempDir(), "")
	}
	cCtx := cli.NewContext(NewApp(out, errOut), flags, nil)
	conf := &config{
		Auth: &token{
			AccessToken: testToken,
			ExpiresAt:   time.Now().Add(time.Hour),
			User: userData{
				Email: testEmail,
			},
		},
	}
	ac := &appClient{
		client:     asc,
		conf:       conf,
		c:          cCtx,
		dataClient: dataClient,
	}
	return cCtx, ac, out, errOut
}

func TestListOrganizationsAction(t *testing.T) {
	listOrganizationsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		orgs := []*apppb.Organization{{Name: "jedi"}, {Name: "mandalorians"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: listOrganizationsFunc,
	}
	cCtx, ac, out, errOut := setup(asc, nil)

	test.That(t, ac.listOrganizationsAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)
	test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("organizations for %q:\n", testEmail))
	test.That(t, out.messages[1], test.ShouldContainSubstring, "jedi")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "mandalorians")
}

func TestTabularDataByFilterAction(t *testing.T) {
	testMap := map[string]interface{}{"bool": true, "string": "true", "float": float64(1)}
	pbStruct, err := protoutils.StructToStructPb(testMap)
	test.That(t, err, test.ShouldBeNil)

	// calls to `TabularDataByFilter` will repeat so long as data continue to be returned,
	// so we need a way of telling our injected method when data has already been sent so we
	// can send an empty response
	dataRequested := false
	tabularDataByFilterFunc := func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption,
	) (*datapb.TabularDataByFilterResponse, error) {
		if dataRequested {
			return &datapb.TabularDataByFilterResponse{}, nil
		}
		dataRequested = true
		return &datapb.TabularDataByFilterResponse{
			Data:     []*datapb.TabularData{{Data: pbStruct}},
			Metadata: []*datapb.CaptureMetadata{{LocationId: "loc-id"}},
		}, nil
	}

	dsc := &inject.DataServiceClient{
		TabularDataByFilterFunc: tabularDataByFilterFunc,
	}
	resp, err := dsc.TabularDataByFilter(context.Background(), &datapb.TabularDataByFilterRequest{})

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp.Data), test.ShouldEqual, 1)

	protoMap := resp.Data[0].Data.AsMap()
	test.That(t, protoMap, test.ShouldResemble, testMap)

	// dataRequested was set to true during the `TabularDataByFilter` call above, so we need
	// to reset it here
	dataRequested = false

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc)

	test.That(t, ac.dataExportAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 4)
	test.That(t, out.messages[0], test.ShouldEqual, "downloading..")
	test.That(t, out.messages[1], test.ShouldEqual, ".")
	test.That(t, out.messages[2], test.ShouldEqual, ".")
	test.That(t, out.messages[3], test.ShouldEqual, "\n")

	// expectedDataSize is the expected string length of the data returned by the injected call
	expectedDataSize := 98
	b := make([]byte, expectedDataSize)

	// `data.ndjson` is the standardized name of the file data is written to in the `tabularData` call
	filePath := fmt.Sprintf("%sdata/data.ndjson", os.TempDir())
	file, err := os.Open(filePath)
	test.That(t, err, test.ShouldBeNil)

	dataSize, err := file.Read(b)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dataSize, test.ShouldEqual, expectedDataSize)

	savedData := string(b)
	expectedData := "{\"MetadataIndex\":0,\"TimeReceived\":null,\"TimeRequested\":null,\"bool\":true,\"float\":1,\"string\":\"true\"}"
	test.That(t, savedData, test.ShouldEqual, expectedData)

	expectedMetadataSize := 23
	b = make([]byte, expectedMetadataSize)

	// metadata is named `0.json` based on its index in the metadata array
	filePath = fmt.Sprintf("%smetadata/0.json", os.TempDir())
	file, err = os.Open(filePath)
	test.That(t, err, test.ShouldBeNil)

	metadataSize, err := file.Read(b)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, metadataSize, test.ShouldEqual, expectedMetadataSize)

	savedMetadata := string(b)
	test.That(t, savedMetadata, test.ShouldEqual, "{\"locationId\":\"loc-id\"}")
}
