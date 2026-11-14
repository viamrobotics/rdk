package shell_test

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(sMap map[resource.Name]shell.Service, logger logging.Logger) (pb.ShellServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(shell.API, sMap)
	if err != nil {
		return nil, err
	}
	return shell.NewRPCServiceServer(coll, logger).(pb.ShellServiceServer), nil
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]shell.Service{
		testSvcName1: &inject.ShellService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	server, err := newServer(resourceMap, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}

var errGetStatusFailed = errors.New("can't get status")

func TestServerGetStatus(t *testing.T) {
	injectShell := &inject.ShellService{}
	resourceMap := map[resource.Name]shell.Service{
		testSvcName1: injectShell,
	}
	server, err := newServer(resourceMap, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	_, err = server.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: "missing"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	resp, err := server.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testSvcName1.ShortName()})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Result.AsMap(), test.ShouldBeEmpty)

	expectedStatus := map[string]interface{}{"key": "value", "count": float64(42)}
	injectShell.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return expectedStatus, nil
	}
	resp, err = server.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testSvcName1.ShortName()})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Result.AsMap(), test.ShouldResemble, expectedStatus)

	injectShell.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return nil, errGetStatusFailed
	}
	_, err = server.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testSvcName1.ShortName()})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errGetStatusFailed.Error())

	injectShell.StatusFunc = nil
}

// TestValidUTF8 checks that non-UTF-8 shell data doesn't break the ShellResponse marshal that
// carries it, which would otherwise drop the shell stream.
func TestValidUTF8(t *testing.T) {
	// valid input (including multibyte) passes through untouched
	in := "hello \u25cf \u65e5\u672c\u8a9e"
	test.That(t, shell.ValidUTF8(in), test.ShouldEqual, in)

	// genuinely malformed bytes are replaced, not dropped
	test.That(t, shell.ValidUTF8(string([]byte{'a', 0xff, 'b'})), test.ShouldEqual, "a�b")

	// a multibyte rune truncated mid-read comes out valid UTF-8 and marshals cleanly
	for _, b := range [][]byte{
		{0xe2, 0x97},       // leading 2 bytes of ●
		{0x8f},             // trailing byte of ●
		{0xf0, 0x9f, 0x98}, // leading 3 bytes of a 4-byte rune
	} {
		got := shell.ValidUTF8(string(b))
		test.That(t, utf8.ValidString(got), test.ShouldBeTrue)
		_, err := proto.Marshal(&pb.ShellResponse{DataOut: got})
		test.That(t, err, test.ShouldBeNil)
	}
}
