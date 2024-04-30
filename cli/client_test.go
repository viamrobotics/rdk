package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

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

	"go.viam.com/rdk/logging"
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

// populateFlags populates a FlagSet from a map.
func populateFlags(m map[string]any) *flag.FlagSet {
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
	defaultFlags map[string]any,
	authMethod string,
) (*cli.Context, *viamClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	flags := populateFlags(defaultFlags)

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
		client:      asc,
		conf:        conf,
		c:           cCtx,
		dataClient:  dataClient,
		buildClient: buildClient,
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
	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

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

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

	test.That(t, ac.dataExportAction(cCtx), test.ShouldBeNil)
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
		cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "")

		test.That(t, ac.robotsPartLogsAction(cCtx), test.ShouldBeNil)

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

		test.That(t, ac.robotsPartLogsAction(cCtx), test.ShouldBeNil)

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
		cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "")

		test.That(t, ac.robotsPartLogsAction(cCtx), test.ShouldBeNil)

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

		test.That(t, ac.robotsPartLogsAction(cCtx), test.ShouldBeNil)

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

		err := ac.robotsPartLogsAction(cCtx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(`provided too high of a "count" value. Maximum is 10000`))
	})
}
