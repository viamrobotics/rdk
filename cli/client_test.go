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
	"go.viam.com/rdk/utils"
)

var (
	testEmail     = "grogu@viam.com"
	testToken     = "thisistheway"
	testKeyID     = "testkeyid"
	testKeyCrypto = "testkeycrypto"
)

type testWriter struct {
	messages []string
}

// Write implements io.Writer.
func (tw *testWriter) Write(b []byte) (int, error) {
	tw.messages = append(tw.messages, string(b))
	return len(b), nil
}

// setup creates a new cli.Context and viamClient with fake auth and the passed
// in AppServiceClient and DataServiceClient. It also returns testWriters that capture Stdout and
// Stdin.
func setup(asc apppb.AppServiceClient, dataClient datapb.DataServiceClient,
	defaultFlags *map[string]string, authMethod string,
) (*cli.Context, *viamClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	flags := &flag.FlagSet{}
	// init all the default flags from the input

	if defaultFlags != nil {
		for name, val := range *defaultFlags {
			flags.String(name, val, "")
		}
	}

	if dataClient != nil {
		// these flags are only relevant when testing a dataClient
		flags.String(dataFlagDataType, dataTypeTabular, "")
		flags.String(dataFlagDestination, utils.ResolveFile(""), "")
	}

	cCtx := cli.NewContext(NewApp(out, errOut), flags, nil)
	conf := &config{}
	if authMethod == "token" {
		conf.Auth = &token{
			AccessToken: testToken,
			ExpiresAt:   time.Now().Add(time.Hour),
			User: userData{
				Email: testEmail,
			},
		}
	} else if authMethod == "apiKey" {
		conf.Auth = &apiKey{
			KeyID:     testKeyID,
			KeyCrypto: testKeyCrypto,
		}
	}
	ac := &viamClient{
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
		orgs := []*apppb.Organization{{Name: "jedi", PublicNamespace: "anakin"}, {Name: "mandalorians"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: listOrganizationsFunc,
	}
	cCtx, ac, out, errOut := setup(asc, nil, nil, "token")

	test.That(t, ac.listOrganizationsAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)
	test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("Organizations for %q:\n", testEmail))
	test.That(t, out.messages[1], test.ShouldContainSubstring, "jedi")
	test.That(t, out.messages[1], test.ShouldContainSubstring, "anakin")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "mandalorians")
}

func TestTabularDataByFilterAction(t *testing.T) {
	pbStruct, err := protoutils.StructToStructPb(map[string]interface{}{"bool": true, "string": "true", "float": float64(1)})
	test.That(t, err, test.ShouldBeNil)

	// calls to `TabularDataByFilter` will repeat so long as data continue to be returned,
	// so we need a way of telling our injected method when data has already been sent so we
	// can send an empty response
	var dataRequested bool
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

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, "token")

	test.That(t, ac.dataExportAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 4)
	test.That(t, len(out.messages), test.ShouldEqual, 0)
	test.That(t, errOut.messages[0], test.ShouldEqual, "Downloading..")
	test.That(t, errOut.messages[1], test.ShouldEqual, ".")
	test.That(t, errOut.messages[2], test.ShouldEqual, ".")
	test.That(t, errOut.messages[3], test.ShouldEqual, "\n")

	// expectedDataSize is the expected string length of the data returned by the injected call
	expectedDataSize := 98
	b := make([]byte, expectedDataSize)

	// `data.ndjson` is the standardized name of the file data is written to in the `tabularData` call
	filePath := utils.ResolveFile("data/data.ndjson")
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
	filePath = utils.ResolveFile("metadata/0.json")
	file, err = os.Open(filePath)
	test.That(t, err, test.ShouldBeNil)

	metadataSize, err := file.Read(b)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, metadataSize, test.ShouldEqual, expectedMetadataSize)

	savedMetadata := string(b)
	test.That(t, savedMetadata, test.ShouldEqual, "{\"locationId\":\"loc-id\"}")
}

func TestBaseURLParsing(t *testing.T) {
	// Test basic parsing
	url, rpcOpts, err := parseBaseURL("https://app.viam.com:443", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "443")
	test.That(t, url.Scheme, test.ShouldEqual, "https")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")
	test.That(t, rpcOpts, test.ShouldBeNil)

	// Test parsing without a port
	url, _, err = parseBaseURL("https://app.viam.com", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "443")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")

	// Test parsing locally
	url, rpcOpts, err = parseBaseURL("http://127.0.0.1:8081", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Scheme, test.ShouldEqual, "http")
	test.That(t, url.Port(), test.ShouldEqual, "8081")
	test.That(t, url.Hostname(), test.ShouldEqual, "127.0.0.1")
	test.That(t, rpcOpts, test.ShouldHaveLength, 2)

	// Test localhost:8080
	url, _, err = parseBaseURL("http://localhost:8080", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "8080")
	test.That(t, url.Hostname(), test.ShouldEqual, "localhost")
	test.That(t, rpcOpts, test.ShouldHaveLength, 2)

	// Test no scheme remote
	url, _, err = parseBaseURL("app.viam.com", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Scheme, test.ShouldEqual, "https")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")

	// Test invalid url
	_, _, err = parseBaseURL(":5", false)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "missing protocol scheme")
}
