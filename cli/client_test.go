package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap/zapcore"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	robotconfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/shell"
	_ "go.viam.com/rdk/services/shell/register"
	shelltestutils "go.viam.com/rdk/services/shell/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
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

// populateFlags populates a FlagSet from a map.
func populateFlags(m map[string]any, args ...string) *flag.FlagSet {
	flags := &flag.FlagSet{}
	// init all the default flags from the input
	for name, val := range m {
		switch v := val.(type) {
		case int:
			flags.Int(name, v, "")
		case string:
			flags.String(name, v, "")
		case bool:
			flags.Bool(name, v, "")
		default:
			// non-int and non-string flags not yet supported
			continue
		}
	}
	if err := flags.Parse(args); err != nil {
		panic(err)
	}
	return flags
}

func newTestContext(t *testing.T, flags map[string]any) *cli.Context {
	t.Helper()
	out := &testWriter{}
	errOut := &testWriter{}
	return cli.NewContext(NewApp(out, errOut), populateFlags(flags), nil)
}

// setup creates a new cli.Context and viamClient with fake auth and the passed
// in AppServiceClient and DataServiceClient. It also returns testWriters that capture Stdout and
// Stdin.
func setup(
	asc apppb.AppServiceClient,
	dataClient datapb.DataServiceClient,
	buildClient buildpb.BuildServiceClient,
	endUserClient apppb.EndUserServiceClient,
	defaultFlags map[string]any,
	authMethod string,
	cliArgs ...string,
) (*cli.Context, *viamClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	flags := populateFlags(defaultFlags, cliArgs...)

	if dataClient != nil {
		// these flags are only relevant when testing a dataClient
		flags.String(dataFlagDataType, dataTypeTabular, "")
		flags.String(dataFlagDestination, utils.ResolveFile(""), "")
	}

	cCtx := cli.NewContext(NewApp(out, errOut), flags, nil)
	conf := &Config{}
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
		client:        asc,
		conf:          conf,
		c:             cCtx,
		dataClient:    dataClient,
		buildClient:   buildClient,
		endUserClient: endUserClient,
		selectedOrg:   &apppb.Organization{},
		selectedLoc:   &apppb.Location{},
	}
	return cCtx, ac, out, errOut
}

//nolint:unparam
func setupWithRunningPart(
	t *testing.T,
	asc apppb.AppServiceClient,
	dataClient datapb.DataServiceClient,
	buildClient buildpb.BuildServiceClient,
	defaultFlags map[string]any,
	authMethod string,
	partFQDN string,
	cliArgs ...string,
) (*cli.Context, *viamClient, *testWriter, *testWriter) {
	t.Helper()

	cCtx, ac, out, errOut := setup(asc, dataClient, buildClient, nil, defaultFlags, authMethod, cliArgs...)

	// this config could later become a parameter
	r, err := robotimpl.New(cCtx.Context, &robotconfig.Config{
		Services: []resource.Config{
			{
				Name:  "shell1",
				API:   shell.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}, logging.NewInMemoryLogger(t))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	options.FQDN = partFQDN
	err = r.StartWeb(cCtx.Context, options)
	test.That(t, err, test.ShouldBeNil)

	// this will be the URL we use to make new clients. In a backwards way, this
	// lets the robot be the one with external auth handling (if auth were being used)
	ac.conf.BaseURL = fmt.Sprintf("http://%s", addr)
	ac.baseURL, _, err = parseBaseURL(ac.conf.BaseURL, false)
	test.That(t, err, test.ShouldBeNil)

	t.Cleanup(func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	})
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
	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")

	test.That(t, ac.listOrganizationsAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)
	test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("Organizations for %q:\n", testEmail))
	test.That(t, out.messages[1], test.ShouldContainSubstring, "jedi")
	test.That(t, out.messages[1], test.ShouldContainSubstring, "anakin")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "mandalorians")
}

func TestSetSupportEmailAction(t *testing.T) {
	setSupportEmailFunc := func(ctx context.Context, in *apppb.OrganizationSetSupportEmailRequest,
		opts ...grpc.CallOption,
	) (*apppb.OrganizationSetSupportEmailResponse, error) {
		return &apppb.OrganizationSetSupportEmailResponse{}, nil
	}
	asc := &inject.AppServiceClient{
		OrganizationSetSupportEmailFunc: setSupportEmailFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")

	test.That(t, ac.organizationsSupportEmailSetAction(cCtx, "test-org", "test-email"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
}

func TestGetSupportEmailAction(t *testing.T) {
	getSupportEmailFunc := func(ctx context.Context, in *apppb.OrganizationGetSupportEmailRequest,
		opts ...grpc.CallOption,
	) (*apppb.OrganizationGetSupportEmailResponse, error) {
		return &apppb.OrganizationGetSupportEmailResponse{Email: "test-email"}, nil
	}
	asc := &inject.AppServiceClient{
		OrganizationGetSupportEmailFunc: getSupportEmailFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")

	test.That(t, ac.organizationsSupportEmailGetAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "test-email")
}

func TestBillingServiceDisableAction(t *testing.T) {
	disableBillingFunc := func(ctx context.Context, in *apppb.DisableBillingServiceRequest, opts ...grpc.CallOption) (
		*apppb.DisableBillingServiceResponse, error,
	) {
		return &apppb.DisableBillingServiceResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		DisableBillingServiceFunc: disableBillingFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")
	test.That(t, ac.organizationDisableBillingServiceAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)

	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully disabled billing service for organization: ")
}

func TestGetBillingConfigAction(t *testing.T) {
	getConfigEmailFunc := func(ctx context.Context, in *apppb.GetBillingServiceConfigRequest, opts ...grpc.CallOption) (
		*apppb.GetBillingServiceConfigResponse, error,
	) {
		address2 := "Apt 123"
		return &apppb.GetBillingServiceConfigResponse{
			SupportEmail: "test-email@mail.com",
			BillingAddress: &apppb.BillingAddress{
				AddressLine_1: "1234 Main St",
				AddressLine_2: &address2,
				City:          "San Francisco",
				State:         "CA",
				Zipcode:       "94105",
			},
			LogoUrl:             "https://logo.com",
			BillingDashboardUrl: "https://app.viam.dev/my-dashboard",
		}, nil
	}

	asc := &inject.AppServiceClient{
		GetBillingServiceConfigFunc: getConfigEmailFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")
	test.That(t, ac.getBillingConfig(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 12)

	test.That(t, out.messages[0], test.ShouldContainSubstring, "Billing config for organization")
	test.That(t, out.messages[1], test.ShouldContainSubstring, "Support Email: test-email@mail.com")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "Billing Dashboard URL: https://app.viam.dev/my-dashboard")
	test.That(t, out.messages[3], test.ShouldContainSubstring, "Logo URL: https://logo.com")
	test.That(t, out.messages[5], test.ShouldContainSubstring, "--- Billing Address --- ")
	test.That(t, out.messages[6], test.ShouldContainSubstring, "1234 Main St")
	test.That(t, out.messages[7], test.ShouldContainSubstring, "Apt 123")
	test.That(t, out.messages[8], test.ShouldContainSubstring, "San Francisco")
	test.That(t, out.messages[9], test.ShouldContainSubstring, "CA")
	test.That(t, out.messages[10], test.ShouldContainSubstring, "94105")
	test.That(t, out.messages[11], test.ShouldContainSubstring, "USA")
}

func TestOrganizationSetLogoAction(t *testing.T) {
	organizationSetLogoFunc := func(ctx context.Context, in *apppb.OrganizationSetLogoRequest, opts ...grpc.CallOption) (
		*apppb.OrganizationSetLogoResponse, error,
	) {
		return &apppb.OrganizationSetLogoResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		OrganizationSetLogoFunc: organizationSetLogoFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")
	// Create a temporary file for testing
	fileName := "test-logo-*.png"
	tmpFile, err := os.CreateTemp("", fileName)
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tmpFile.Name()) // Clean up temp file after test
	test.That(t, ac.organizationLogoSetAction(cCtx, "test-org", tmpFile.Name()), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully set the logo for organization")

	cCtx, ac, out, errOut = setup(asc, nil, nil, nil, nil, "token")

	logoFileName2 := "test-logo-2-*.PNG"
	tmpFile2, err := os.CreateTemp("", logoFileName2)
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tmpFile2.Name()) // Clean up temp file after test

	test.That(t, ac.organizationLogoSetAction(cCtx, "test-org", tmpFile.Name()), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully set the logo for organization")

	cCtx, ac, out, _ = setup(asc, nil, nil, nil, nil, "token")
	invalidLogoFilePath := "data/test-logo.jpg"
	err = ac.organizationLogoSetAction(cCtx, "test-org", invalidLogoFilePath)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not a valid .png file path")
	test.That(t, len(out.messages), test.ShouldEqual, 0)
}

func TestGetLogoAction(t *testing.T) {
	getLogoFunc := func(ctx context.Context, in *apppb.OrganizationGetLogoRequest, opts ...grpc.CallOption) (
		*apppb.OrganizationGetLogoResponse, error,
	) {
		return &apppb.OrganizationGetLogoResponse{Url: "https://logo.com"}, nil
	}

	asc := &inject.AppServiceClient{
		OrganizationGetLogoFunc: getLogoFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")

	test.That(t, ac.organizationsLogoGetAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "https://logo.com")
}

func TestUpdateBillingServiceAction(t *testing.T) {
	updateConfigFunc := func(ctx context.Context, in *apppb.UpdateBillingServiceRequest, opts ...grpc.CallOption) (
		*apppb.UpdateBillingServiceResponse, error,
	) {
		return &apppb.UpdateBillingServiceResponse{}, nil
	}
	asc := &inject.AppServiceClient{
		UpdateBillingServiceFunc: updateConfigFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "token")
	address := "123 Main St, Suite 100, San Francisco, CA, 94105"
	test.That(t, ac.updateBillingServiceAction(cCtx, "test-org", address), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 8)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully updated billing service for organization")
	test.That(t, out.messages[1], test.ShouldContainSubstring, " --- Billing Address --- ")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "123 Main St")
	test.That(t, out.messages[3], test.ShouldContainSubstring, "Suite 100")
	test.That(t, out.messages[4], test.ShouldContainSubstring, "San Francisco")
	test.That(t, out.messages[5], test.ShouldContainSubstring, "CA")
	test.That(t, out.messages[6], test.ShouldContainSubstring, "94105")
	test.That(t, out.messages[7], test.ShouldContainSubstring, "USA")
}

func TestTabularDataByFilterAction(t *testing.T) {
	pbStruct, err := protoutils.StructToStructPb(map[string]interface{}{"bool": true, "string": "true", "float": float64(1)})
	test.That(t, err, test.ShouldBeNil)

	// calls to `TabularDataByFilter` will repeat so long as data continue to be returned,
	// so we need a way of telling our injected method when data has already been sent so we
	// can send an empty response
	var dataRequested bool
	//nolint:deprecated,staticcheck
	tabularDataByFilterFunc := func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption,
	//nolint:deprecated
	) (*datapb.TabularDataByFilterResponse, error) {
		if dataRequested {
			//nolint:deprecated,staticcheck
			return &datapb.TabularDataByFilterResponse{}, nil
		}
		dataRequested = true
		//nolint:deprecated,staticcheck
		return &datapb.TabularDataByFilterResponse{
			//nolint:deprecated,staticcheck
			Data:     []*datapb.TabularData{{Data: pbStruct}},
			Metadata: []*datapb.CaptureMetadata{{LocationId: "loc-id"}},
		}, nil
	}

	dsc := &inject.DataServiceClient{
		TabularDataByFilterFunc: tabularDataByFilterFunc,
	}

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, nil, "token")

	test.That(t, ac.dataExportAction(cCtx, parseStructFromCtx[dataExportArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 4)
	test.That(t, out.messages[0], test.ShouldEqual, "Downloading..")
	test.That(t, out.messages[1], test.ShouldEqual, ".")
	test.That(t, out.messages[2], test.ShouldEqual, ".")
	test.That(t, out.messages[3], test.ShouldEqual, "\n")

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

func TestLogEntryFieldsToString(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		f1, err := logging.FieldToProto(zapcore.Field{
			Key:    "key1",
			Type:   zapcore.StringType,
			String: "value1",
		})
		test.That(t, err, test.ShouldBeNil)
		f2, err := logging.FieldToProto(zapcore.Field{
			Key:     "key2",
			Type:    zapcore.Int32Type,
			Integer: 123,
		})
		test.That(t, err, test.ShouldBeNil)
		f3, err := logging.FieldToProto(zapcore.Field{
			Key:       "facts",
			Type:      zapcore.ReflectType,
			Interface: map[string]string{"app.viam": "cool", "cli": "cooler"},
		})
		test.That(t, err, test.ShouldBeNil)
		fields := []*structpb.Struct{
			f1, f2, f3,
		}

		expected := `{"key1": "value1", "key2": 123, "facts": map[app.viam:cool cli:cooler]}`
		result, err := logEntryFieldsToString(fields)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldEqual, expected)
	})

	t.Run("empty fields", func(t *testing.T) {
		result, err := logEntryFieldsToString([]*structpb.Struct{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldBeEmpty)
	})
}

func TestGetRobotPartLogs(t *testing.T) {
	// Create fake logs of "0"->"9999".
	logs := make([]*commonpb.LogEntry, 0, maxNumLogs)
	for i := 0; i < maxNumLogs; i++ {
		logs = append(logs, &commonpb.LogEntry{Message: fmt.Sprintf("%d", i)})
	}

	getRobotPartLogsFunc := func(ctx context.Context, in *apppb.GetRobotPartLogsRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotPartLogsResponse, error) {
		// Accept fake page tokens of "2"-"100" and release logs in batches of 100.
		// The first page token will be "", which should be interpreted as "1".
		pt := 1
		if receivedPt := in.PageToken; receivedPt != nil && *receivedPt != "" {
			var err error
			pt, err = strconv.Atoi(*receivedPt)
			test.That(t, err, test.ShouldBeNil)
		}
		resp := &apppb.GetRobotPartLogsResponse{
			Logs:          logs[(pt-1)*100 : pt*100],
			NextPageToken: fmt.Sprintf("%d", pt+1),
		}
		return resp, nil
	}

	listOrganizationsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		orgs := []*apppb.Organization{{Name: "jedi", Id: "123"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListLocationsResponse, error) {
		locs := []*apppb.Location{{Name: "naboo"}}
		return &apppb.ListLocationsResponse{Locations: locs}, nil
	}
	listRobotsFunc := func(ctx context.Context, in *apppb.ListRobotsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListRobotsResponse, error) {
		robots := []*apppb.Robot{{Name: "r2d2"}}
		return &apppb.ListRobotsResponse{Robots: robots}, nil
	}
	getRobotPartsFunc := func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotPartsResponse, error) {
		parts := []*apppb.RobotPart{{Name: "main"}}
		return &apppb.GetRobotPartsResponse{Parts: parts}, nil
	}

	asc := &inject.AppServiceClient{
		GetRobotPartLogsFunc: getRobotPartLogsFunc,
		// Supply some injected functions to avoid a panic when loading
		// organizations, locations, robots and parts.
		ListOrganizationsFunc: listOrganizationsFunc,
		ListLocationsFunc:     listLocationsFunc,
		ListRobotsFunc:        listRobotsFunc,
		GetRobotPartsFunc:     getRobotPartsFunc,
	}

	t.Run("no count", func(t *testing.T) {
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, nil, "")

		test.That(t, ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx)), test.ShouldBeNil)

		// No warnings.
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)

		// There should be a message for "organization -> location -> robot"
		// followed by maxNumLogs messages.
		test.That(t, len(out.messages), test.ShouldEqual, defaultNumLogs+1)
		test.That(t, out.messages[0], test.ShouldEqual, "jedi -> naboo -> r2d2\n")
		// Logs should be printed in order oldest->newest ("99"->"0").
		expectedLogNum := defaultNumLogs - 1
		for i := 1; i <= defaultNumLogs; i++ {
			test.That(t, out.messages[i], test.ShouldContainSubstring,
				fmt.Sprintf("%d", expectedLogNum))
			expectedLogNum--
		}
	})
	t.Run("178 count", func(t *testing.T) {
		flags := map[string]any{"count": 178}
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, flags, "")

		test.That(t, ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx)), test.ShouldBeNil)

		// No warnings.
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)

		// There should be a message for "organization -> location -> robot"
		// followed by 178 messages.
		test.That(t, len(out.messages), test.ShouldEqual, 179)
		test.That(t, out.messages[0], test.ShouldEqual, "jedi -> naboo -> r2d2\n")
		// Logs should be printed in order oldest->newest ("177"->"0").
		expectedLogNum := 177
		for i := 1; i <= 178; i++ {
			test.That(t, out.messages[i], test.ShouldContainSubstring,
				fmt.Sprintf("%d", expectedLogNum))
			expectedLogNum--
		}
	})
	t.Run("max count", func(t *testing.T) {
		flags := map[string]any{logsFlagCount: maxNumLogs}
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, flags, "")

		test.That(t, ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx)), test.ShouldBeNil)

		// No warnings.
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)

		// There should be a message for "organization -> location -> robot"
		// followed by maxNumLogs messages.
		test.That(t, len(out.messages), test.ShouldEqual, maxNumLogs+1)
		test.That(t, out.messages[0], test.ShouldEqual, "jedi -> naboo -> r2d2\n")

		// Logs should be printed in order oldest->newest ("9999"->"0").
		expectedLogNum := maxNumLogs - 1
		for i := 1; i <= maxNumLogs; i++ {
			test.That(t, out.messages[i], test.ShouldContainSubstring,
				fmt.Sprintf("%d", expectedLogNum))
			expectedLogNum--
		}
	})
	t.Run("negative count", func(t *testing.T) {
		flags := map[string]any{"count": -1}
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, flags, "")

		test.That(t, ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx)), test.ShouldBeNil)

		// Warning should read: `Warning:\nProvided negative "count" value. Defaulting to 100`.
		test.That(t, len(errOut.messages), test.ShouldEqual, 2)
		test.That(t, errOut.messages[0], test.ShouldEqual, "Warning: ")
		test.That(t, errOut.messages[1], test.ShouldContainSubstring, `Provided negative "count" value. Defaulting to 100`)

		// There should be a message for "organization -> location -> robot"
		// followed by maxNumLogs messages.
		test.That(t, len(out.messages), test.ShouldEqual, defaultNumLogs+1)
		test.That(t, out.messages[0], test.ShouldEqual, "jedi -> naboo -> r2d2\n")
		// Logs should be printed in order oldest->oldest ("99"->"0").
		expectedLogNum := defaultNumLogs - 1
		for i := 1; i <= defaultNumLogs; i++ {
			test.That(t, out.messages[i], test.ShouldContainSubstring,
				fmt.Sprintf("%d", expectedLogNum))
			expectedLogNum--
		}
	})
	t.Run("count too high", func(t *testing.T) {
		flags := map[string]any{"count": 1000000}
		cCtx, ac, _, _ := setup(asc, nil, nil, nil, flags, "")

		err := ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(`provided too high of a "count" value. Maximum is 10000`))
	})
}

func TestShellFileCopy(t *testing.T) {
	listOrganizationsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		orgs := []*apppb.Organization{{Name: "jedi", Id: uuid.NewString(), PublicNamespace: "anakin"}, {Name: "mandalorians"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListLocationsResponse, error) {
		locs := []*apppb.Location{{Name: "naboo"}}
		return &apppb.ListLocationsResponse{Locations: locs}, nil
	}
	listRobotsFunc := func(ctx context.Context, in *apppb.ListRobotsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListRobotsResponse, error) {
		robots := []*apppb.Robot{{Name: "r2d2"}}
		return &apppb.ListRobotsResponse{Robots: robots}, nil
	}

	partFqdn := uuid.NewString()
	getRobotPartsFunc := func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotPartsResponse, error) {
		parts := []*apppb.RobotPart{{Name: "main", Fqdn: partFqdn}}
		return &apppb.GetRobotPartsResponse{Parts: parts}, nil
	}

	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: listOrganizationsFunc,
		ListLocationsFunc:     listLocationsFunc,
		ListRobotsFunc:        listRobotsFunc,
		GetRobotPartsFunc:     getRobotPartsFunc,
	}

	partFlags := map[string]any{
		"organization": "jedi",
		"location":     "naboo",
		"robot":        "r2d2",
		"part":         "main",
	}

	t.Run("no arguments or files", func(t *testing.T) {
		cCtx, viamClient, _, _ := setup(asc, nil, nil, nil, partFlags, "token")
		test.That(t,
			machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)),
			test.ShouldEqual, errNoFiles)
	})

	t.Run("one file path is insufficient", func(t *testing.T) {
		args := []string{"machine:path"}
		cCtx, viamClient, _, _ := setup(asc, nil, nil, nil, partFlags, "token", args...)
		test.That(t,
			machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)),
			test.ShouldEqual, errLastArgOfFromMissing)

		args = []string{"path"}
		cCtx, viamClient, _, _ = setup(asc, nil, nil, nil, partFlags, "token", args...)
		test.That(t,
			machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)),
			test.ShouldEqual, errLastArgOfToMissing)
	})

	t.Run("from has wrong path prefixes", func(t *testing.T) {
		args := []string{"machine:path", "path2", "machine:path3", "destination"}
		cCtx, viamClient, _, _ := setup(asc, nil, nil, nil, partFlags, "token", args...)
		test.That(t,
			machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)),
			test.ShouldHaveSameTypeAs, copyFromPathInvalidError{})
	})

	tfs := shelltestutils.SetupTestFileSystem(t)

	t.Run("from", func(t *testing.T) {
		t.Run("single file", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{fmt.Sprintf("machine:%s", tfs.SingleFileNested), tempDir}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
		})

		t.Run("single file relative", func(t *testing.T) {
			tempDir := t.TempDir()
			cwd, err := os.Getwd()
			test.That(t, err, test.ShouldBeNil)
			t.Cleanup(func() { os.Chdir(cwd) })
			test.That(t, os.Chdir(tempDir), test.ShouldBeNil)

			args := []string{fmt.Sprintf("machine:%s", tfs.SingleFileNested), "foo"}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(filepath.Join(tempDir, "foo"))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
		})

		t.Run("single directory", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{fmt.Sprintf("machine:%s", tfs.Root), tempDir}

			t.Log("without recursion set")
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			err := machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx))
			test.That(t, errors.Is(err, errDirectoryCopyRequestNoRecursion), test.ShouldBeTrue)
			_, err = os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)

			t.Log("with recursion set")
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ = setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)
			test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
		})

		t.Run("multiple files", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{
				fmt.Sprintf("machine:%s", tfs.SingleFileNested),
				fmt.Sprintf("machine:%s", tfs.InnerDir),
				tempDir,
			}
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)

			test.That(t, shelltestutils.DirectoryContentsEqual(tfs.InnerDir, filepath.Join(tempDir, filepath.Base(tfs.InnerDir))), test.ShouldBeNil)
		})

		t.Run("preserve permissions on a nested file", func(t *testing.T) {
			tfs := shelltestutils.SetupTestFileSystem(t)

			beforeInfo, err := os.Stat(tfs.SingleFileNested)
			test.That(t, err, test.ShouldBeNil)
			t.Log("start with mode", beforeInfo.Mode())
			newMode := os.FileMode(0o444)
			test.That(t, beforeInfo.Mode(), test.ShouldNotEqual, newMode)
			test.That(t, os.Chmod(tfs.SingleFileNested, newMode), test.ShouldBeNil)
			modTime := time.Date(1988, 1, 2, 3, 0, 0, 0, time.UTC)
			test.That(t, os.Chtimes(tfs.SingleFileNested, time.Time{}, modTime), test.ShouldBeNil)
			relNestedPath := strings.TrimPrefix(tfs.SingleFileNested, tfs.Root)

			for _, preserve := range []bool{false, true} {
				t.Run(fmt.Sprintf("preserve=%t", preserve), func(t *testing.T) {
					tempDir := t.TempDir()

					args := []string{fmt.Sprintf("machine:%s", tfs.Root), tempDir}

					partFlagsCopy := make(map[string]any, len(partFlags))
					maps.Copy(partFlagsCopy, partFlags)
					partFlagsCopy["recursive"] = true
					partFlagsCopy["preserve"] = preserve
					cCtx, viamClient, _, _ := setupWithRunningPart(
						t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
					test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)
					test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)

					nestedCopy := filepath.Join(tempDir, filepath.Base(tfs.Root), relNestedPath)
					test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
					afterInfo, err := os.Stat(nestedCopy)
					test.That(t, err, test.ShouldBeNil)
					if preserve {
						test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldEqual, modTime.String())
						test.That(t, afterInfo.Mode(), test.ShouldEqual, newMode)
					} else {
						test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldNotEqual, modTime.String())
						test.That(t, afterInfo.Mode(), test.ShouldNotEqual, newMode)
					}
				})
			}
		})
	})

	t.Run("to", func(t *testing.T) {
		t.Run("single file", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{tfs.SingleFileNested, fmt.Sprintf("machine:%s", tempDir)}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
		})

		t.Run("single file relative", func(t *testing.T) {
			homeDir, err := os.UserHomeDir()
			test.That(t, err, test.ShouldBeNil)
			randomName := uuid.NewString()
			randomPath := filepath.Join(homeDir, randomName)
			defer os.Remove(randomPath)
			args := []string{tfs.SingleFileNested, fmt.Sprintf("machine:%s", randomName)}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(randomPath)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
		})

		t.Run("single directory", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{tfs.Root, fmt.Sprintf("machine:%s", tempDir)}

			t.Log("without recursion set")
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			err := machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx))
			test.That(t, errors.Is(err, errDirectoryCopyRequestNoRecursion), test.ShouldBeTrue)
			_, err = os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)

			t.Log("with recursion set")
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ = setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)
			test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
		})

		t.Run("multiple files", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{
				tfs.SingleFileNested,
				tfs.InnerDir,
				fmt.Sprintf("machine:%s", tempDir),
			}
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)

			rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)

			test.That(t, shelltestutils.DirectoryContentsEqual(tfs.InnerDir, filepath.Join(tempDir, filepath.Base(tfs.InnerDir))), test.ShouldBeNil)
		})

		t.Run("preserve permissions on a nested file", func(t *testing.T) {
			tfs := shelltestutils.SetupTestFileSystem(t)

			beforeInfo, err := os.Stat(tfs.SingleFileNested)
			test.That(t, err, test.ShouldBeNil)
			t.Log("start with mode", beforeInfo.Mode())
			newMode := os.FileMode(0o444)
			test.That(t, beforeInfo.Mode(), test.ShouldNotEqual, newMode)
			test.That(t, os.Chmod(tfs.SingleFileNested, newMode), test.ShouldBeNil)
			modTime := time.Date(1988, 1, 2, 3, 0, 0, 0, time.UTC)
			test.That(t, os.Chtimes(tfs.SingleFileNested, time.Time{}, modTime), test.ShouldBeNil)
			relNestedPath := strings.TrimPrefix(tfs.SingleFileNested, tfs.Root)

			for _, preserve := range []bool{false, true} {
				t.Run(fmt.Sprintf("preserve=%t", preserve), func(t *testing.T) {
					tempDir := t.TempDir()

					args := []string{tfs.Root, fmt.Sprintf("machine:%s", tempDir)}

					partFlagsCopy := make(map[string]any, len(partFlags))
					maps.Copy(partFlagsCopy, partFlags)
					partFlagsCopy["recursive"] = true
					partFlagsCopy["preserve"] = preserve
					cCtx, viamClient, _, _ := setupWithRunningPart(
						t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
					test.That(t, machinesPartCopyFilesAction(cCtx, viamClient, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx)), test.ShouldBeNil)
					test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)

					nestedCopy := filepath.Join(tempDir, filepath.Base(tfs.Root), relNestedPath)
					test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
					afterInfo, err := os.Stat(nestedCopy)
					test.That(t, err, test.ShouldBeNil)
					if preserve {
						test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldEqual, modTime.String())
						test.That(t, afterInfo.Mode(), test.ShouldEqual, newMode)
					} else {
						test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldNotEqual, modTime.String())
						test.That(t, afterInfo.Mode(), test.ShouldNotEqual, newMode)
					}
				})
			}
		})
	})
}
