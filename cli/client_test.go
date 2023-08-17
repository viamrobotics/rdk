package cli

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
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
func setup(asc apppb.AppServiceClient) (*cli.Context, *appClient, *testWriter, *testWriter) {
	out := &testWriter{}
	errOut := &testWriter{}
	cCtx := &cli.Context{
		App: NewApp(out, errOut),
	}
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
		client: asc,
		conf:   conf,
		c:      cCtx,
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
	cCtx, ac, out, errOut := setup(asc)

	test.That(t, ac.listOrganizationsAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)
	test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("organizations for %q:\n", testEmail))
	test.That(t, out.messages[1], test.ShouldContainSubstring, "jedi")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "mandalorians")
}
