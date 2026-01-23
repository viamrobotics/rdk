package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	"go.viam.com/rdk/web/server"
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
	return cli.NewContext(newTestApp(out, errOut), populateFlags(flags), nil)
}

// setup creates a new cli.Context and viamClient with fake auth and the passed
// in AppServiceClient and DataServiceClient. It also returns testWriters that capture Stdout and
// Stdin.
func setup(asc apppb.AppServiceClient, dataClient datapb.DataServiceClient,
	buildClient buildpb.BuildServiceClient, defaultFlags map[string]any,
	authMethod string, cliArgs ...string,
) (*cli.Context, *viamClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	flags := populateFlags(defaultFlags, cliArgs...)

	if dataClient != nil {
		// these flags are only relevant when testing a dataClient
		flags.String(generalFlagDestination, utils.ResolveFile(""), "")
	}

	cCtx := cli.NewContext(newTestApp(out, errOut), flags, nil)
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
		client:      asc,
		conf:        conf,
		c:           cCtx,
		dataClient:  dataClient,
		buildClient: buildClient,
		selectedOrg: &apppb.Organization{},
		selectedLoc: &apppb.Location{},
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

	cCtx, ac, out, errOut := setup(asc, dataClient, buildClient, defaultFlags, authMethod, cliArgs...)

	// this config could later become a parameter
	r, err := robotimpl.New(cCtx.Context, &robotconfig.Config{
		Services: []resource.Config{
			{
				Name:  "shell1",
				API:   shell.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}, nil, logging.NewInMemoryLogger(t))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	options.FQDN = partFQDN
	err = r.StartWeb(cCtx.Context, options)
	test.That(t, err, test.ShouldBeNil)

	// this will be the URL we use to make new clients. In a backwards way, this
	// lets the robot be the one with external auth handling (if auth were being used)
	ac.conf.BaseURL = fmt.Sprintf("http://%s", addr)
	ac.baseURL, _, err = utils.ParseBaseURL(ac.conf.BaseURL, false)
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
	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
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
				Country:       "United States",
			},
			LogoUrl:             "https://logo.com",
			BillingDashboardUrl: "https://app.viam.dev/my-dashboard",
		}, nil
	}

	asc := &inject.AppServiceClient{
		GetBillingServiceConfigFunc: getConfigEmailFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
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
	test.That(t, out.messages[11], test.ShouldContainSubstring, "United States")
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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
	// Create a temporary file for testing
	fileName := "test-logo-*.png"
	tmpFile, err := os.CreateTemp("", fileName)
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tmpFile.Name()) // Clean up temp file after test
	test.That(t, ac.organizationLogoSetAction(cCtx, "test-org", tmpFile.Name()), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully set the logo for organization")

	cCtx, ac, out, errOut = setup(asc, nil, nil, nil, "token")

	logoFileName2 := "test-logo-2-*.PNG"
	tmpFile2, err := os.CreateTemp("", logoFileName2)
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tmpFile2.Name()) // Clean up temp file after test

	test.That(t, ac.organizationLogoSetAction(cCtx, "test-org", tmpFile.Name()), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully set the logo for organization")
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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

	test.That(t, ac.organizationsLogoGetAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "https://logo.com")
}

func TestEnableAuthServiceAction(t *testing.T) {
	enableAuthServiceFunc := func(ctx context.Context, in *apppb.EnableAuthServiceRequest, opts ...grpc.CallOption) (
		*apppb.EnableAuthServiceResponse, error,
	) {
		return &apppb.EnableAuthServiceResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		EnableAuthServiceFunc: enableAuthServiceFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

	test.That(t, ac.enableAuthServiceAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "enabled auth")
}

func TestDisableAuthServiceAction(t *testing.T) {
	disableAuthServiceFunc := func(ctx context.Context, in *apppb.DisableAuthServiceRequest, opts ...grpc.CallOption) (
		*apppb.DisableAuthServiceResponse, error,
	) {
		return &apppb.DisableAuthServiceResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		DisableAuthServiceFunc: disableAuthServiceFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

	test.That(t, ac.disableAuthServiceAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "disabled auth")

	err := ac.disableAuthServiceAction(cCtx, "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot disable")
}

func TestListOAuthAppsAction(t *testing.T) {
	listOAuthAppFunc := func(ctx context.Context, in *apppb.ListOAuthAppsRequest, opts ...grpc.CallOption) (
		*apppb.ListOAuthAppsResponse, error,
	) {
		return &apppb.ListOAuthAppsResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		ListOAuthAppsFunc: listOAuthAppFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
	test.That(t, ac.listOAuthAppsAction(cCtx, "test-org"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "No OAuth apps found for organization")
}

func TestDeleteOAuthAppAction(t *testing.T) {
	deleteOAuthAppFunc := func(ctx context.Context, in *apppb.DeleteOAuthAppRequest, opts ...grpc.CallOption) (
		*apppb.DeleteOAuthAppResponse, error,
	) {
		return &apppb.DeleteOAuthAppResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		DeleteOAuthAppFunc: deleteOAuthAppFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
	test.That(t, ac.deleteOAuthAppAction(cCtx, "test-org", "client-id"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully deleted OAuth application")
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

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
	address := "123 Main St, Suite 100, San Francisco, CA, 94105, United States"
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
	test.That(t, out.messages[7], test.ShouldContainSubstring, "United States")
}

func TestOrganizationEnableBillingServiceAction(t *testing.T) {
	enableBillingFunc := func(ctx context.Context, in *apppb.EnableBillingServiceRequest, opts ...grpc.CallOption) (
		*apppb.EnableBillingServiceResponse, error,
	) {
		return &apppb.EnableBillingServiceResponse{}, nil
	}

	asc := &inject.AppServiceClient{
		EnableBillingServiceFunc: enableBillingFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")
	test.That(t, ac.organizationEnableBillingServiceAction(cCtx, "test-org",
		"123 Main St, Suite 100, San Francisco, CA, 94105"), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully enabled billing service for organization")
}

type mockDataServiceClient struct {
	grpc.ClientStream
	responses []*datapb.ExportTabularDataResponse
	index     int
	err       error
}

func (m *mockDataServiceClient) Recv() (*datapb.ExportTabularDataResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.index >= len(m.responses) {
		return nil, io.EOF
	}

	resp := m.responses[m.index]
	m.index++

	return resp, nil
}

func newMockExportStream(responses []*datapb.ExportTabularDataResponse, err error) *mockDataServiceClient {
	return &mockDataServiceClient{
		responses: responses,
		err:       err,
	}
}

func TestDataExportTabularAction(t *testing.T) {
	t.Run("successful case", func(t *testing.T) {
		pbStructPayload1, err := protoutils.StructToStructPb(map[string]interface{}{"bool": true, "string": "true", "float": float64(1)})
		test.That(t, err, test.ShouldBeNil)

		pbStructPayload2, err := protoutils.StructToStructPb(map[string]interface{}{"booly": false, "string": "true", "float": float64(1)})
		test.That(t, err, test.ShouldBeNil)

		exportTabularDataFunc := func(ctx context.Context, in *datapb.ExportTabularDataRequest, opts ...grpc.CallOption,
		) (datapb.DataService_ExportTabularDataClient, error) {
			return newMockExportStream([]*datapb.ExportTabularDataResponse{
				{LocationId: "loc-id", Payload: pbStructPayload1},
				{LocationId: "loc-id", Payload: pbStructPayload2},
			}, nil), nil
		}

		dsc := &inject.DataServiceClient{
			ExportTabularDataFunc: exportTabularDataFunc,
		}

		cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

		test.That(t, ac.dataExportTabularAction(cCtx, parseStructFromCtx[dataExportTabularArgs](cCtx)), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 3)
		test.That(t, strings.Join(out.messages, ""), test.ShouldEqual, "Downloading...\n")

		filePath := utils.ResolveFile(dataFileName)

		data, err := os.ReadFile(filePath)
		test.That(t, err, test.ShouldBeNil)

		// Output is unstable, so parse back into maps before comparing to expected.
		var actual []map[string]interface{}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		for decoder.More() {
			var item map[string]interface{}
			err = decoder.Decode(&item)
			test.That(t, err, test.ShouldBeNil)
			actual = append(actual, item)
		}

		expectedData := []map[string]interface{}{
			{
				"locationId": "loc-id",
				"payload": map[string]interface{}{
					"bool":   true,
					"float":  float64(1),
					"string": "true",
				},
			},
			{
				"locationId": "loc-id",
				"payload": map[string]interface{}{
					"booly":  false,
					"float":  float64(1),
					"string": "true",
				},
			},
		}

		test.That(t, actual, test.ShouldResemble, expectedData)
	})

	t.Run("error case", func(t *testing.T) {
		exportTabularDataFunc := func(ctx context.Context, in *datapb.ExportTabularDataRequest, opts ...grpc.CallOption,
		) (datapb.DataService_ExportTabularDataClient, error) {
			return newMockExportStream([]*datapb.ExportTabularDataResponse{}, errors.New("whoops")), nil
		}

		dsc := &inject.DataServiceClient{
			ExportTabularDataFunc: exportTabularDataFunc,
		}

		cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

		err := ac.dataExportTabularAction(cCtx, parseStructFromCtx[dataExportTabularArgs](cCtx))
		test.That(t, err, test.ShouldBeError, errors.New("error receiving tabular data: whoops"))
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)

		// Test that export was retried (total of 5 tries).
		test.That(t, len(out.messages), test.ShouldEqual, 7)
		test.That(t, strings.Join(out.messages, ""), test.ShouldEqual, "Downloading.......\n")

		// Test that the data.ndjson file was removed.
		filePath := utils.ResolveFile(dataFileName)
		_, err = os.ReadFile(filePath)
		test.That(t, err, test.ShouldBeError, fmt.Errorf("open %s: no such file or directory", filePath))
	})
}

func TestBaseURLParsing(t *testing.T) {
	// Test basic parsing
	url, rpcOpts, err := utils.ParseBaseURL("https://app.viam.com:443", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "443")
	test.That(t, url.Scheme, test.ShouldEqual, "https")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")
	test.That(t, rpcOpts, test.ShouldBeNil)

	// Test parsing without a port
	url, _, err = utils.ParseBaseURL("https://app.viam.com", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "443")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")

	// Test parsing locally
	url, rpcOpts, err = utils.ParseBaseURL("http://127.0.0.1:8081", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Scheme, test.ShouldEqual, "http")
	test.That(t, url.Port(), test.ShouldEqual, "8081")
	test.That(t, url.Hostname(), test.ShouldEqual, "127.0.0.1")
	test.That(t, rpcOpts, test.ShouldHaveLength, 2)

	// Test localhost:8080
	url, _, err = utils.ParseBaseURL("http://localhost:8080", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Port(), test.ShouldEqual, "8080")
	test.That(t, url.Hostname(), test.ShouldEqual, "localhost")
	test.That(t, rpcOpts, test.ShouldHaveLength, 2)

	// Test no scheme remote
	url, _, err = utils.ParseBaseURL("app.viam.com", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, url.Scheme, test.ShouldEqual, "https")
	test.That(t, url.Hostname(), test.ShouldEqual, "app.viam.com")

	// Test invalid url
	_, _, err = utils.ParseBaseURL(":5", false)
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

	loc := apppb.Location{Name: "naboo"}

	listOrganizationsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		orgs := []*apppb.Organization{{Name: "jedi", Id: "123"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	getOrganizationsWithAccessToLocationFunc := func(ctx context.Context, in *apppb.GetOrganizationsWithAccessToLocationRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetOrganizationsWithAccessToLocationResponse, error) {
		orgIdentities := []*apppb.OrganizationIdentity{{Name: "jedi", Id: "123"}}
		return &apppb.GetOrganizationsWithAccessToLocationResponse{OrganizationIdentities: orgIdentities}, nil
	}
	getLocationFunc := func(ctx context.Context, in *apppb.GetLocationRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetLocationResponse, error) {
		return &apppb.GetLocationResponse{Location: &loc}, nil
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListLocationsResponse, error) {
		return &apppb.ListLocationsResponse{Locations: []*apppb.Location{&loc}}, nil
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
		ListOrganizationsFunc:                    listOrganizationsFunc,
		ListLocationsFunc:                        listLocationsFunc,
		ListRobotsFunc:                           listRobotsFunc,
		GetRobotPartsFunc:                        getRobotPartsFunc,
		GetLocationFunc:                          getLocationFunc,
		GetOrganizationsWithAccessToLocationFunc: getOrganizationsWithAccessToLocationFunc,
	}

	t.Run("no count", func(t *testing.T) {
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "")

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
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "")

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
		flags := map[string]any{generalFlagCount: maxNumLogs}
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "")

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
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "")

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
		cCtx, ac, _, _ := setup(asc, nil, nil, flags, "")

		err := ac.robotsPartLogsAction(cCtx, parseStructFromCtx[robotsPartLogsArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(`provided too high of a "count" value. Maximum is 10000`))
	})
}

func TestShellFileCopy(t *testing.T) {
	logger := logging.NewTestLogger(t)

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
		cCtx, viamClient, _, _ := setup(asc, nil, nil, partFlags, "token")
		test.That(t,
			viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
			test.ShouldEqual, errNoFiles)
	})

	t.Run("one file path is insufficient", func(t *testing.T) {
		args := []string{"machine:path"}
		cCtx, viamClient, _, _ := setup(asc, nil, nil, partFlags, "token", args...)
		test.That(t,
			viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
			test.ShouldEqual, errLastArgOfFromMissing)

		args = []string{"path"}
		cCtx, viamClient, _, _ = setup(asc, nil, nil, partFlags, "token", args...)
		test.That(t,
			viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
			test.ShouldEqual, errLastArgOfToMissing)
	})

	t.Run("from has wrong path prefixes", func(t *testing.T) {
		args := []string{"machine:path", "path2", "machine:path3", "destination"}
		cCtx, viamClient, _, _ := setup(asc, nil, nil, partFlags, "token", args...)
		test.That(t,
			viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
			test.ShouldHaveSameTypeAs, copyFromPathInvalidError{})
	})

	tfs := shelltestutils.SetupTestFileSystem(t)

	t.Run("from", func(t *testing.T) {
		t.Run("single file", func(t *testing.T) {
			tempDir := t.TempDir()

			args := []string{fmt.Sprintf("machine:%s", tfs.SingleFileNested), tempDir}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
			err := viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger)
			test.That(t, errors.Is(err, errDirectoryCopyRequestNoRecursion), test.ShouldBeTrue)
			_, err = os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)

			t.Log("with recursion set")
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ = setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)
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
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
					test.That(t,
						viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
						test.ShouldBeNil)
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
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
			err := viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger)
			test.That(t, errors.Is(err, errDirectoryCopyRequestNoRecursion), test.ShouldBeTrue)
			_, err = os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
			test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)

			t.Log("with recursion set")
			partFlagsCopy := make(map[string]any, len(partFlags))
			maps.Copy(partFlagsCopy, partFlags)
			partFlagsCopy["recursive"] = true
			cCtx, viamClient, _, _ = setupWithRunningPart(
				t, asc, nil, nil, partFlagsCopy, "token", partFqdn, args...)
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)
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
			test.That(t,
				viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
				test.ShouldBeNil)

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
					test.That(t,
						viamClient.machinesPartCopyFilesAction(cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger),
						test.ShouldBeNil)
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

func TestShellGetFTDC(t *testing.T) {
	logger := logging.NewTestLogger(t)

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
	partID := uuid.NewString()
	getRobotPartsFunc := func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotPartsResponse, error) {
		parts := []*apppb.RobotPart{{Name: "main", Fqdn: partFqdn, Id: partID}}
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

	t.Run("too many arguments", func(t *testing.T) {
		args := []string{"foo", "bar"}
		cCtx, viamClient, _, _ := setup(asc, nil, nil, partFlags, "token", args...)
		test.That(t,
			viamClient.machinesPartGetFTDCAction(cCtx, parseStructFromCtx[machinesPartGetFTDCArgs](cCtx), true, logger),
			test.ShouldBeError, wrongNumArgsError{2, 0, 1})
	})

	tfs := shelltestutils.SetupTestFileSystem(t)

	t.Run("ftdc data does not exist", func(t *testing.T) {
		tempDir := t.TempDir()

		originalFTDCPath := ftdcPath
		ftdcPath = filepath.Join(tfs.Root, "FAKEDIR")
		t.Cleanup(func() {
			ftdcPath = originalFTDCPath
		})

		args := []string{tempDir}
		cCtx, viamClient, _, _ := setupWithRunningPart(
			t, asc, nil, nil, partFlags, "token", partFqdn, args...)
		test.That(t,
			viamClient.machinesPartGetFTDCAction(cCtx, parseStructFromCtx[machinesPartGetFTDCArgs](cCtx), true, logger),
			test.ShouldNotBeNil)

		entries, err := os.ReadDir(tempDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, entries, test.ShouldHaveLength, 0)
	})

	t.Run("ftdc data exists", func(t *testing.T) {
		tmpPartFtdcPath := filepath.Join(tfs.Root, partID)
		err := os.Mkdir(tmpPartFtdcPath, 0o750)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() {
			err = os.RemoveAll(tmpPartFtdcPath)
			test.That(t, err, test.ShouldBeNil)
		})
		err = os.WriteFile(filepath.Join(tfs.Root, partID, "foo"), nil, 0o640)
		test.That(t, err, test.ShouldBeNil)
		originalFTDCPath := ftdcPath
		ftdcPath = tfs.Root
		t.Cleanup(func() {
			ftdcPath = originalFTDCPath
		})

		testDownload := func(t *testing.T, targetPath string) {
			args := []string{}
			if targetPath != "" {
				args = append(args, targetPath)
			} else {
				targetPath = "."
			}
			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, partFlags, "token", partFqdn, args...)
			test.That(t,
				viamClient.machinesPartGetFTDCAction(cCtx, parseStructFromCtx[machinesPartGetFTDCArgs](cCtx), true, logger),
				test.ShouldBeNil)

			entries, err := os.ReadDir(filepath.Join(targetPath, partID))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, entries, test.ShouldHaveLength, 1)
			ftdcFile := entries[0]
			test.That(t, ftdcFile.Name(), test.ShouldEqual, "foo")
			test.That(t, ftdcFile.IsDir(), test.ShouldBeFalse)
		}

		t.Run("download to cwd", func(t *testing.T) {
			tempDir := t.TempDir()
			originalWd, err := os.Getwd()
			test.That(t, err, test.ShouldBeNil)
			err = os.Chdir(tempDir)
			test.That(t, err, test.ShouldBeNil)
			t.Cleanup(func() {
				os.Chdir(originalWd)
			})

			testDownload(t, "")
		})
		t.Run("download to specified path", func(t *testing.T) {
			testDownload(t, t.TempDir())
		})
	})
}

func TestCreateOAuthAppAction(t *testing.T) {
	createOAuthAppFunc := func(ctx context.Context, in *apppb.CreateOAuthAppRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateOAuthAppResponse, error) {
		return &apppb.CreateOAuthAppResponse{ClientId: "client-id", ClientSecret: "client-secret"}, nil
	}
	asc := &inject.AppServiceClient{
		CreateOAuthAppFunc: createOAuthAppFunc,
	}
	t.Run("valid inputs", func(t *testing.T) {
		flags := make(map[string]any)
		flags[generalFlagOrgID] = "org-id"
		flags[oauthAppFlagClientName] = "client-name"
		flags[oauthAppFlagClientAuthentication] = "required"
		flags[oauthAppFlagURLValidation] = "allow_wildcards"
		flags[oauthAppFlagPKCE] = "not_required"
		flags[oauthAppFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
		flags[oauthAppFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
		flags[oauthAppFlagLogoutURI] = "https://woof.com/logout"
		flags[oauthAppFlagEnabledGrants] = []string{"implicit", "password"}
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "token")
		test.That(t, ac.createOAuthAppAction(cCtx, parseStructFromCtx[createOAuthAppArgs](cCtx)), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, out.messages[0], test.ShouldContainSubstring,
			"Successfully created OAuth app client-name with client ID client-id and client secret client-secret")
	})

	t.Run("should error if pkce is not a valid enum value", func(t *testing.T) {
		flags := map[string]any{
			oauthAppFlagClientAuthentication: unspecified,
			oauthAppFlagPKCE:                 "not_one_of_the_allowed_values",
			generalFlagOrgID:                 "some-org-id",
		}
		cCtx, ac, out, _ := setup(asc, nil, nil, flags, "token")
		err := ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "pkce must be a valid PKCE")
		test.That(t, len(out.messages), test.ShouldEqual, 0)
	})

	t.Run("should error if url-validation is not a valid enum value", func(t *testing.T) {
		flags := map[string]any{
			oauthAppFlagClientAuthentication: unspecified, oauthAppFlagPKCE: unspecified,
			oauthAppFlagURLValidation: "not_one_of_the_allowed_values",
			generalFlagOrgID:          "some-org-id",
		}
		cCtx, ac, out, _ := setup(asc, nil, nil, flags, "token")
		err := ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "url-validation must be a valid UrlValidation")
		test.That(t, len(out.messages), test.ShouldEqual, 0)
	})
}

func TestReadOAuthApp(t *testing.T) {
	readOAuthAppFunc := func(ctx context.Context, in *apppb.ReadOAuthAppRequest, opts ...grpc.CallOption) (
		*apppb.ReadOAuthAppResponse, error,
	) {
		return &apppb.ReadOAuthAppResponse{
			ClientName:   "clientname",
			ClientSecret: "fakesecret",
			OauthConfig: &apppb.OAuthConfig{
				ClientAuthentication: apppb.ClientAuthentication_CLIENT_AUTHENTICATION_REQUIRED,
				Pkce:                 apppb.PKCE_PKCE_REQUIRED,
				UrlValidation:        apppb.URLValidation_URL_VALIDATION_ALLOW_WILDCARDS,
				LogoutUri:            "https://my-logout-uri.com",
				OriginUris:           []string{"https://my-origin-uri.com", "https://second-origin-uri.com"},
				RedirectUris:         []string{"https://my-redirect-uri.com"},
				EnabledGrants:        []apppb.EnabledGrant{apppb.EnabledGrant_ENABLED_GRANT_IMPLICIT, apppb.EnabledGrant_ENABLED_GRANT_PASSWORD},
			},
		}, nil
	}

	asc := &inject.AppServiceClient{
		ReadOAuthAppFunc: readOAuthAppFunc,
	}

	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

	test.That(t, ac.readOAuthAppAction(cCtx, "test-org-id", "test-client-id"), test.ShouldBeNil)
	test.That(t, len(out.messages), test.ShouldEqual, 10)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "OAuth config for client ID test-client-id")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "Client Name: clientname")
	test.That(t, out.messages[3], test.ShouldContainSubstring, "Client Authentication: required")
	test.That(t, out.messages[4], test.ShouldContainSubstring, "PKCE (Proof Key for Code Exchange): required")
	test.That(t, out.messages[5], test.ShouldContainSubstring, "URL Validation Policy: allow_wildcards")
	test.That(t, out.messages[6], test.ShouldContainSubstring, "Logout URL: https://my-logout-uri.com")
	test.That(t, out.messages[7], test.ShouldContainSubstring, "Redirect URLs: https://my-redirect-uri.com")
	test.That(t, out.messages[8], test.ShouldContainSubstring, "Origin URLs: https://my-origin-uri.com, https://second-origin-uri.com")
	test.That(t, out.messages[9], test.ShouldContainSubstring, "Enabled Grants: implicit, password")
}

func TestUpdateOAuthAppAction(t *testing.T) {
	updateOAuthAppFunc := func(ctx context.Context, in *apppb.UpdateOAuthAppRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateOAuthAppResponse, error) {
		return &apppb.UpdateOAuthAppResponse{}, nil
	}
	asc := &inject.AppServiceClient{
		UpdateOAuthAppFunc: updateOAuthAppFunc,
	}

	t.Run("valid inputs", func(t *testing.T) {
		flags := make(map[string]any)
		flags[generalFlagOrgID] = "org-id"
		flags[oauthAppFlagClientID] = "client-id"
		flags[oauthAppFlagClientName] = "client-name"
		flags[oauthAppFlagClientAuthentication] = "required"
		flags[oauthAppFlagURLValidation] = "allow_wildcards"
		flags[oauthAppFlagPKCE] = "not_required"
		flags[oauthAppFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
		flags[oauthAppFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
		flags[oauthAppFlagLogoutURI] = "https://woof.com/logout"
		flags[oauthAppFlagEnabledGrants] = []string{"implicit", "password"}
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "token")
		test.That(t, ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx)), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, out.messages[0], test.ShouldContainSubstring, "Successfully updated OAuth app")
	})

	t.Run("should error if client-authentication is not a valid enum value", func(t *testing.T) {
		flags := make(map[string]any)
		flags[generalFlagOrgID] = "org-id"
		flags[oauthAppFlagClientID] = "client-id"
		flags[oauthAppFlagClientAuthentication] = "not_one_of_the_allowed_values"
		cCtx, ac, out, _ := setup(asc, nil, nil, flags, "token")
		err := ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "client-authentication must be a valid ClientAuthentication")
		test.That(t, len(out.messages), test.ShouldEqual, 0)
	})

	t.Run("should error if pkce is not a valid enum value", func(t *testing.T) {
		flags := map[string]any{
			oauthAppFlagClientAuthentication: unspecified,
			oauthAppFlagPKCE:                 "not_one_of_the_allowed_values",
			generalFlagOrgID:                 "some-org-id",
		}
		cCtx, ac, out, _ := setup(asc, nil, nil, flags, "token")
		err := ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "pkce must be a valid PKCE")
		test.That(t, len(out.messages), test.ShouldEqual, 0)
	})

	t.Run("should error if url-validation is not a valid enum value", func(t *testing.T) {
		flags := map[string]any{
			oauthAppFlagClientAuthentication: unspecified, oauthAppFlagPKCE: unspecified,
			oauthAppFlagURLValidation: "not_one_of_the_allowed_values",
			generalFlagOrgID:          "some_org_id",
		}
		cCtx, ac, out, _ := setup(asc, nil, nil, flags, "token")
		err := ac.updateOAuthAppAction(cCtx, parseStructFromCtx[updateOAuthAppArgs](cCtx))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "url-validation must be a valid UrlValidation")
		test.That(t, len(out.messages), test.ShouldEqual, 0)
	})
}

func TestTunnelE2ECLI(t *testing.T) {
	t.Parallel()
	// `TestTunnelE2ECLI` attempts to send "Hello, World!" across a tunnel created by the
	// CLI. It is mostly identical to `TestTunnelE2E` in web/server/entrypoint_test.go.
	// The tunnel is:
	//
	// test-process <-> cli-listener(localhost:23659) <-> machine(localhost:23658) <-> dest-listener(localhost:23657)

	tunnelMsg := "Hello, World!"
	destPort := 23657
	destListenerAddr := net.JoinHostPort("localhost", strconv.Itoa(destPort))
	machineAddr := net.JoinHostPort("localhost", "23658")
	sourcePort := 23657
	sourceListenerAddr := net.JoinHostPort("localhost", strconv.Itoa(sourcePort))

	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	runServerCtx, runServerCtxCancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	// Start "destination" listener.
	destListener, err := net.Listen("tcp", destListenerAddr)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, destListener.Close(), test.ShouldBeNil)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Infof("Listening on %s for tunnel message", destListenerAddr)
		conn, err := destListener.Accept()
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, conn.Close(), test.ShouldBeNil)
		}()

		bytes := make([]byte, 1024)
		n, err := conn.Read(bytes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, len(tunnelMsg))
		test.That(t, string(bytes), test.ShouldContainSubstring, tunnelMsg)
		logger.Info("Received expected tunnel message at", destListenerAddr)

		// Write the same message back.
		n, err = conn.Write([]byte(tunnelMsg))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, len(tunnelMsg))
	}()

	// Start a machine at `machineAddr` (`RunServer` in a goroutine.)
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Create a temporary config file.
		tempConfigFile, err := os.CreateTemp(t.TempDir(), "temp_config.json")
		test.That(t, err, test.ShouldBeNil)
		cfg := &robotconfig.Config{
			Network: robotconfig.NetworkConfig{
				NetworkConfigData: robotconfig.NetworkConfigData{
					TrafficTunnelEndpoints: []robotconfig.TrafficTunnelEndpoint{
						{
							Port: destPort, // allow tunneling to destination port
						},
					},
					BindAddress: machineAddr,
				},
			},
		}
		cfgBytes, err := json.Marshal(&cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(tempConfigFile.Name(), cfgBytes, 0o755), test.ShouldBeNil)

		args := []string{"viam-server", "-config", tempConfigFile.Name()}
		test.That(t, server.RunServer(runServerCtx, args, logger), test.ShouldBeNil)
	}()

	rc := robottestutils.NewRobotClient(t, logger, machineAddr, time.Second)

	// Start CLI tunneler.
	//nolint:dogsled
	cCtx, _, _, _ := setup(nil, nil, nil, nil, "token")

	// error early if tunnel not listed
	err = tunnelTraffic(cCtx, rc, sourcePort, 1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not allowed")

	wg.Add(1)
	go func() {
		defer wg.Done()
		tunnelTraffic(cCtx, rc, sourcePort, destPort)
	}()

	// Write `tunnelMsg` to CLI tunneler over TCP from this test process.
	conn, err := net.Dial("tcp", sourceListenerAddr)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()
	n, err := conn.Write([]byte(tunnelMsg))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, len(tunnelMsg))

	// Expect `tunnelMsg` to be written back.
	bytes := make([]byte, 1024)
	n, err = conn.Read(bytes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, len(tunnelMsg))
	test.That(t, string(bytes), test.ShouldContainSubstring, tunnelMsg)

	// Cancel `runServerCtx` once message has made it all the way across and has been
	// echoed back. This should stop the `RunServer` goroutine.
	runServerCtxCancel()

	wg.Wait()
}

func TestAPIToGRPCServiceName(t *testing.T) {
	tests := []struct {
		name     string
		api      resource.API
		expected string
	}{
		{
			name:     "simple component camera",
			api:      resource.APINamespaceRDK.WithComponentType("camera"),
			expected: "viam.component.camera.v1.CameraService",
		},
		{
			name:     "simple component arm",
			api:      resource.APINamespaceRDK.WithComponentType("arm"),
			expected: "viam.component.arm.v1.ArmService",
		},
		{
			name:     "compound component movement_sensor",
			api:      resource.APINamespaceRDK.WithComponentType("movement_sensor"),
			expected: "viam.component.movementsensor.v1.MovementSensorService",
		},
		{
			name:     "compound component input_controller",
			api:      resource.APINamespaceRDK.WithComponentType("input_controller"),
			expected: "viam.component.inputcontroller.v1.InputControllerService",
		},
		{
			name:     "simple service vision",
			api:      resource.APINamespaceRDK.WithServiceType("vision"),
			expected: "viam.service.vision.v1.VisionService",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apiToGRPCServiceName(tt.api)
			test.That(t, result, test.ShouldEqual, tt.expected)
		})
	}
}

func TestIsShortMethodName(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"DoCommand", true},
		{"GetPosition", true},
		{"viam.component.camera.v1.CameraService.DoCommand", false},
		{"CameraService.DoCommand", false},
		{"CameraService/DoCommand", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := isShortMethodName(tt.method)
			test.That(t, result, test.ShouldEqual, tt.expected)
		})
	}
}

func TestMergeComponentNameIntoData(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		componentName string
		expected      string
		shouldErr     bool
	}{
		{
			name:          "empty data",
			data:          "",
			componentName: "my-camera",
			expected:      `{"name":"my-camera"}`,
			shouldErr:     false,
		},
		{
			name:          "existing data without name",
			data:          `{"foo":"bar"}`,
			componentName: "my-camera",
			expected:      `{"foo":"bar","name":"my-camera"}`,
			shouldErr:     false,
		},
		{
			name:          "existing data with name should not override",
			data:          `{"name":"existing","foo":"bar"}`,
			componentName: "my-camera",
			expected:      `{"foo":"bar","name":"existing"}`,
			shouldErr:     false,
		},
		{
			name:          "invalid json should error",
			data:          `{invalid`,
			componentName: "my-camera",
			expected:      "",
			shouldErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mergeComponentNameIntoData(tt.data, tt.componentName)
			if tt.shouldErr {
				test.That(t, err, test.ShouldNotBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				// Parse both JSONs and compare as maps since key order may vary
				var expectedMap, resultMap map[string]interface{}
				test.That(t, json.Unmarshal([]byte(tt.expected), &expectedMap), test.ShouldBeNil)
				test.That(t, json.Unmarshal([]byte(result), &resultMap), test.ShouldBeNil)
				test.That(t, resultMap, test.ShouldResemble, expectedMap)
			}
		})
	}
}

func TestCLIUpdateAction(t *testing.T) {
	// Save original version to restore later
	originalVersion := robotconfig.Version
	defer func() {
		robotconfig.Version = originalVersion
	}()

	// Set up a mock latest version
	mockLatestVersion := "0.104.0"
	originalGetLatestReleaseVersion := getLatestReleaseVersionFunc
	getLatestReleaseVersionFunc = func() (string, error) {
		return mockLatestVersion, nil
	}
	defer func() {
		getLatestReleaseVersionFunc = originalGetLatestReleaseVersion
	}()

	testLatestVersion, err := latestVersion()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, testLatestVersion.Original(), test.ShouldEqual, mockLatestVersion)

	// Set local version to 0.100.0 (older than mockLatestVersion 0.104.0)
	robotconfig.Version = "0.100.0"
	testLocalVersion, err := localVersion()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, testLocalVersion.Original(), test.ShouldEqual, "0.100.0")

	// Test that binaryURL returns a valid URL with correct OS/arch and .exe for Windows
	actualURL := binaryURL()
	test.That(t, actualURL, test.ShouldContainSubstring, "https://storage.googleapis.com/packages.viam.com/apps/viam-cli/viam-cli-stable-")
	test.That(t, actualURL, test.ShouldContainSubstring, runtime.GOOS)
	test.That(t, actualURL, test.ShouldContainSubstring, runtime.GOARCH)
	if runtime.GOOS == osWindows {
		test.That(t, strings.HasSuffix(actualURL, ".exe"), test.ShouldBeTrue)
	} else {
		test.That(t, actualURL, test.ShouldNotContainSubstring, ".exe")
	}

	// Test that downloadBinaryIntoDir succeeds
	// Create a test HTTP server that serves a mock binary
	newBinaryContent := []byte("new-binary-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(newBinaryContent)
	}))
	defer server.Close()

	// Create temp directory for download
	tempDir := t.TempDir()

	filename := "/viam-cli-stable"
	if runtime.GOOS == osWindows {
		filename += ".exe"
	}
	downloadURL := server.URL + filename
	downloadedPath, err := downloadBinaryIntoDir(downloadURL, tempDir)
	test.That(t, err, test.ShouldBeNil)

	// downloadBinaryIntoDir creates a file with a fixed name pattern, not the URL filename
	expectedFileName := "viam-cli-update"
	if runtime.GOOS == osWindows {
		expectedFileName += ".exe"
	}
	expectedFileName += ".new"
	expectedPath := filepath.Join(tempDir, expectedFileName)
	test.That(t, downloadedPath, test.ShouldEqual, expectedPath)

	// Verify file was created and has correct content
	downloadedContent, err := os.ReadFile(downloadedPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, downloadedContent, test.ShouldResemble, newBinaryContent)

	// Test that replaceBinary works (create two temp binaries and see if replaced)
	oldBinaryPath := filepath.Join(tempDir, "old-binary")
	newBinaryPath := downloadedPath // Use the downloaded file as the "new" binary

	// Create old binary file
	err = os.WriteFile(oldBinaryPath, []byte("old-binary-content"), 0o755)
	test.That(t, err, test.ShouldBeNil)

	// Replace old binary with new binary
	err = replaceBinary(oldBinaryPath, newBinaryPath)
	test.That(t, err, test.ShouldBeNil)

	// Verify that old binary location now contains the new binary content
	// (os.Rename moves the new binary to the old location, so newBinaryPath no longer exists)
	oldBinary, err := os.ReadFile(oldBinaryPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oldBinary, test.ShouldResemble, newBinaryContent)
	// Verify that newBinaryPath no longer exists (it was moved to oldBinaryPath)
	_, err = os.ReadFile(newBinaryPath)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
}

func TestRetryableCopy(t *testing.T) {
	t.Run("SuccessOnFirstAttempt", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		mockCopyFunc := func() error {
			return nil // Success immediately
		}

		allSteps := []*Step{
			{ID: "copy", Message: "Copying package...", CompletedMsg: "Package copied", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("copy")
		test.That(t, err, test.ShouldBeNil)

		// Copy to part
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			false,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, errOut.messages, test.ShouldHaveLength, 0)

		// Verify no retry steps were created
		retryStepFound := false
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepFound = true
				break
			}
		}
		test.That(t, retryStepFound, test.ShouldBeFalse)

		// Copy from part
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			true,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, errOut.messages, test.ShouldHaveLength, 0)

		// Verify no retry steps were created for the second
		retryStepFound = false
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepFound = true
				break
			}
		}
		test.That(t, retryStepFound, test.ShouldBeFalse)
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepFound = true
				break
			}
		}
		test.That(t, retryStepFound, test.ShouldBeFalse)
	})

	t.Run("SuccessAfter2Retries", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func() error {
			attemptCount++
			if attemptCount <= 2 {
				return errors.New("copy failed")
			}
			return nil // Success on 3rd attempt
		}

		allSteps := []*Step{
			{ID: "copy", Message: "Copying package...", CompletedMsg: "Package copied", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("copy")
		test.That(t, err, test.ShouldBeNil)

		// Copy to part
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			false,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 3)

		// Verify retry steps were created (attempt 1, 2, and 3)
		retryStepCount := 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepCount++
				// Verify IndentLevel is 2 for deeper nesting
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		test.That(t, retryStepCount, test.ShouldEqual, 3) // Attempt-1, Attempt-2, and Attempt-3

		// Verify no duplicate warning messages in errOut (only permission denied warnings should appear)
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Attempt 1/6 failed:")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Attempt 2/6 failed:")

		// Copy from part - reset attemptCount for the second call
		attemptCount = 0
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			true,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 3)

		// Verify retry steps were created (attempt 1, 2, and 3)
		retryStepCount = 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepCount++
				// Verify IndentLevel is 2 for deeper nesting
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		// 3 from first call + 3 from second call
		test.That(t, retryStepCount, test.ShouldEqual, 6)

		// Verify no duplicate warning messages in errOut (only permission denied warnings should appear)
		errMsg = strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Attempt 1/6 failed:")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Attempt 2/6 failed:")
	})

	t.Run("SuccessAfter5Retries", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func() error {
			attemptCount++
			if attemptCount <= 5 {
				return errors.New("copy failed")
			}
			return nil // Success on 6th attempt
		}

		allSteps := []*Step{
			{ID: "copy", Message: "Copying package...", CompletedMsg: "Package copied", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("copy")
		test.That(t, err, test.ShouldBeNil)

		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			false,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 6)

		// Verify all retry steps were created (attempt 1 through 6)
		retryStepCount := 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepCount++
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		test.That(t, retryStepCount, test.ShouldEqual, 6) // attempt-1 through attempt-6

		// No duplicate warning messages should appear (only permission denied warnings)
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Attempt")

		// Copy from part - reset attemptCount for the second call
		attemptCount = 0
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			true,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 6)

		// Verify all retry steps were created (attempt 1 through 6)
		retryStepCount = 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "Attempt-") {
				retryStepCount++
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		// 6 from first call + 6 from second call
		test.That(t, retryStepCount, test.ShouldEqual, 12)

		// No duplicate warning messages should appear (only permission denied warnings)
		errMsg = strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "copy attempt")
	})

	t.Run("AllAttemptsFail", func(t *testing.T) {
		cCtx, vc, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func() error {
			attemptCount++
			return errors.New("persistent copy failure")
		}

		allSteps := []*Step{
			{ID: "copy", Message: "Copying package...", CompletedMsg: "Package copied", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("copy")
		test.That(t, err, test.ShouldBeNil)

		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			false,
		)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "persistent copy failure")
		test.That(t, attemptCount, test.ShouldEqual, 6)

		// Copy from part - reset attemptCount for the second call
		attemptCount = 0
		_, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			true,
		)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "persistent copy failure")
		test.That(t, attemptCount, test.ShouldEqual, 6)
	})

	t.Run("PermissionDeniedError", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		mockCopyFunc := func() error {
			return status.Error(codes.PermissionDenied, "permission denied")
		}

		allSteps := []*Step{
			{ID: "copy", Message: "Copying package...", CompletedMsg: "Package copied", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("copy")
		test.That(t, err, test.ShouldBeNil)

		attemptCount, err := vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			false,
		)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 1)

		// Verify permission denied specific warning appears
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldContainSubstring, "RDK couldn't write to the default file copy destination")
		test.That(t, errMsg, test.ShouldContainSubstring, "--home")
		test.That(t, errMsg, test.ShouldContainSubstring, "run the RDK as root")

		// Copy from part
		attemptCount, err = vc.retryableCopy(
			cCtx,
			pm,
			mockCopyFunc,
			true,
		)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 1)

		// Verify permission denied specific warning appears
		errMsg = strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldContainSubstring, "RDK couldn't read the source files on the machine.")
	})
}
