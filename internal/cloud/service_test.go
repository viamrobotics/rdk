package cloud_test

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestNotCloudManaged(t *testing.T) {
	logger := logging.NewTestLogger(t)
	svc := cloud.NewCloudConnectionService(nil, nil, logger)
	_, _, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldEqual, cloud.ErrNotCloudManaged)
	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
	_, _, err = svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldEqual, cloud.ErrNotCloudManaged)
}

func TestCloudManaged(t *testing.T) {
	logger := logging.NewTestLogger(t)

	server, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, server.RegisterServiceServer(
		context.Background(),
		&echopb.EchoService_ServiceDesc,
		&echoserver.Server{},
		echopb.RegisterEchoServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	test.That(t, server.Start(), test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	addr := server.InternalAddr().String()

	conf := &config.Cloud{
		AppAddress: fmt.Sprintf("http://%s", addr),
	}

	appConn, err := grpc.NewAppConn(context.Background(), conf.AppAddress, conf.ID, nil, logger)
	test.That(t, err, test.ShouldBeNil)

	svc := cloud.NewCloudConnectionService(conf, appConn, logger)
	id, conn1, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id, test.ShouldBeEmpty)
	test.That(t, conn1, test.ShouldEqual, appConn)

	id2, conn2, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id2, test.ShouldBeEmpty)
	test.That(t, conn2, test.ShouldEqual, appConn)

	echoClient1 := echopb.NewEchoServiceClient(conn1)
	echoClient2 := echopb.NewEchoServiceClient(conn2)

	resp, err := echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	test.That(t, appConn.Close(), test.ShouldBeNil)

	// now "both" connections are closed
	resp, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)

	resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)

	id3, conn3, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id3, test.ShouldBeEmpty)
	test.That(t, conn3, test.ShouldNotBeNil)
	test.That(t, conn3, test.ShouldEqual, conn2)

	echoClient3 := echopb.NewEchoServiceClient(conn3)
	resp, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
}

func TestCloudManagedWithAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)

	server, err := rpc.NewServer(
		logger,
		rpc.WithAuthHandler(
			utils.CredentialsTypeRobotSecret,
			rpc.MakeSimpleMultiAuthHandler([]string{"secret_foo"}, []string{"secret_bar"}),
		),
		rpc.WithAuthHandler(
			utils.CredentialsTypeAPIKey,
			rpc.MakeSimpleMultiAuthHandler([]string{"api_foo"}, []string{"api_bar"}),
		),
	)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, server.RegisterServiceServer(
		context.Background(),
		&echopb.EchoService_ServiceDesc,
		&echoserver.Server{},
		echopb.RegisterEchoServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	test.That(t, server.Start(), test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	addr := server.InternalAddr().String()

	testCases := []struct {
		name       string
		config     *config.Cloud
		shouldAuth bool
	}{
		{
			name: "no credentials - should fail auth",
			config: &config.Cloud{
				AppAddress: fmt.Sprintf("http://%s", addr),
			},
			shouldAuth: false,
		},
		{
			name: "robot secret credentials - should succeed",
			config: &config.Cloud{
				AppAddress: fmt.Sprintf("http://%s", addr),
				ID:         "secret_foo",
				Secret:     "secret_bar",
			},
			shouldAuth: true,
		},
		{
			name: "API key credentials - should succeed",
			config: &config.Cloud{
				AppAddress: fmt.Sprintf("http://%s", addr),
				APIKey: config.APIKey{
					ID:  "api_foo",
					Key: "api_bar",
				},
			},
			shouldAuth: true,
		},
		{
			name: "both credentials - API key should be prioritized",
			config: &config.Cloud{
				AppAddress: fmt.Sprintf("http://%s", addr),
				ID:         "secret_foo",
				Secret:     "secret_bar",
				APIKey: config.APIKey{
					ID:  "api_foo",
					Key: "api_bar",
				},
			},
			shouldAuth: true,
		},
		{
			name: "valid robot secret with invalid API key - should fail",
			config: &config.Cloud{
				AppAddress: fmt.Sprintf("http://%s", addr),
				ID:         "secret_foo",
				Secret:     "secret_bar",
				APIKey: config.APIKey{
					ID:  "invalid_foo",
					Key: "invalid_bar",
				},
			},
			shouldAuth: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCloudConnectionAuth(t, logger, tc.config, tc.shouldAuth)
		})
	}
}

func testCloudConnectionAuth(t *testing.T, logger logging.Logger, conf *config.Cloud, shouldAuth bool) {
	cloudCreds := conf.GetCloudCredsDialOpt()
	appConn, err := grpc.NewAppConn(context.Background(), conf.AppAddress, conf.ID, cloudCreds, logger)
	test.That(t, err, test.ShouldBeNil)

	svc := cloud.NewCloudConnectionService(conf, appConn, logger)
	_, conn1, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1, test.ShouldEqual, appConn)

	_, conn2, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn2, test.ShouldEqual, appConn)

	echoClient1 := echopb.NewEchoServiceClient(conn1)
	echoClient2 := echopb.NewEchoServiceClient(conn2)

	// Test first echo call
	resp, err := echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})

	if shouldAuth {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Message, test.ShouldEqual, "hello")

		// Test second echo call
		resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
			Message: "hello",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Message, test.ShouldEqual, "hello")

		test.That(t, appConn.Close(), test.ShouldBeNil)

		// Test connection behavior after close
		resp, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
			Message: "hello",
		})
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)

		resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
			Message: "hello",
		})
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)

		_, conn3, err := svc.AcquireConnection(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, conn3, test.ShouldNotBeNil)
		test.That(t, conn3, test.ShouldEqual, conn2)

		echoClient3 := echopb.NewEchoServiceClient(conn3)
		resp, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
			Message: "hello",
		})
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldEqual, grpc.ErrNotConnected)
	} else {
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unauthenticated)

		test.That(t, appConn.Close(), test.ShouldBeNil)
	}

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
}
