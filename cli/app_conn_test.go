package cli

import (
	"context"
	"net"
	"net/url"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
)

// newAppConnTestClient returns a viamClient connected to a bare gRPC server standing in for app.
// Nothing is registered on that server, so an app call over a working connection answers
// Unimplemented.
func newAppConnTestClient(t *testing.T) *viamClient {
	t.Helper()

	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	server := grpc.NewServer()
	go func() {
		utils.UncheckedError(server.Serve(listener))
	}()
	t.Cleanup(server.Stop)

	//nolint:dogsled
	_, vc, _, _ := setup(nil, nil, nil, nil, "token")
	vc.baseURL = &url.URL{Scheme: "http", Host: listener.Addr().String()}
	vc.conf.BaseURL = vc.baseURL.String()

	conn, err := vc.dialApp(context.Background())
	test.That(t, err, test.ShouldBeNil)
	vc.setAppClients(conn)
	t.Cleanup(vc.closeAppConn)
	return vc
}

func listOrgs(vc *viamClient) error {
	_, err := vc.client.ListOrganizations(context.Background(), &apppb.ListOrganizationsRequest{})
	return err
}

// TestRedialApp asserts that closing the app connection is recoverable: redialApp reconnects and
// rebuilds the service clients, and an app call made without it errors rather than panicking.
func TestRedialApp(t *testing.T) {
	vc := newAppConnTestClient(t)
	test.That(t, status.Code(listOrgs(vc)), test.ShouldEqual, codes.Unimplemented)

	vc.closeAppConn()
	vc.closeAppConn() // closing twice is a no-op.
	test.That(t, vc.conn, test.ShouldBeNil)

	// the clients still point at the closed connection, so a missed redialApp is an error.
	test.That(t, status.Code(listOrgs(vc)), test.ShouldEqual, codes.Canceled)

	test.That(t, vc.redialApp(context.Background()), test.ShouldBeNil)
	test.That(t, vc.conn, test.ShouldNotBeNil)
	test.That(t, status.Code(listOrgs(vc)), test.ShouldEqual, codes.Unimplemented)

	// with the connection up, redialApp leaves it alone.
	conn := vc.conn
	test.That(t, vc.redialApp(context.Background()), test.ShouldBeNil)
	test.That(t, vc.conn == conn, test.ShouldBeTrue)
}

// TestRedialAppLeavesInjectedClientsAlone asserts redialApp is inert for a viamClient that never
// had a connection of its own, which is how the tests for app-facing commands are built.
func TestRedialAppLeavesInjectedClientsAlone(t *testing.T) {
	//nolint:dogsled
	_, vc, _, _ := setup(nil, nil, nil, nil, "token")
	vc.closeAppConn()
	test.That(t, vc.redialApp(context.Background()), test.ShouldBeNil)
	test.That(t, vc.conn, test.ShouldBeNil)
}

// TestConnectToRobotClosesAppConn asserts that dialing a machine drops the app connection, which
// is what keeps it from idling for the length of a machine session.
func TestConnectToRobotClosesAppConn(t *testing.T) {
	vc := newAppConnTestClient(t)
	vc.dialOverride = func(
		ctx context.Context, fqdn string, rpcOpts []rpc.DialOption, logger logging.Logger,
	) (*client.RobotClient, error) {
		return nil, nil
	}
	test.That(t, status.Code(listOrgs(vc)), test.ShouldEqual, codes.Unimplemented)

	_, err := vc.connectToRobot(context.Background(), "fqdn", nil, false, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vc.conn, test.ShouldBeNil)

	test.That(t, vc.redialApp(context.Background()), test.ShouldBeNil)
	test.That(t, status.Code(listOrgs(vc)), test.ShouldEqual, codes.Unimplemented)
}
