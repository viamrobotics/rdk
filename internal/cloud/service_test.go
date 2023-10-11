package cloud_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.viam.com/test"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestNotCloudManaged(t *testing.T) {
	logger := logging.NewTestLogger(t)
	svc := cloud.NewCloudConnectionService(nil, logger)
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

	svc := cloud.NewCloudConnectionService(conf, logger)
	id, conn1, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id, test.ShouldBeEmpty)
	test.That(t, conn1, test.ShouldNotBeNil)

	id2, conn2, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id2, test.ShouldBeEmpty)
	test.That(t, conn2, test.ShouldNotBeNil)
	test.That(t, conn2, test.ShouldNotEqual, conn1)

	echoClient1 := echopb.NewEchoServiceClient(conn1)
	echoClient2 := echopb.NewEchoServiceClient(conn2)

	resp, err := echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	conn2.Close()

	// technically conn2 is open via ref count
	resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")
	test.That(t, conn1.Close(), test.ShouldBeNil)

	// now both are closed
	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)

	id3, conn3, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id3, test.ShouldBeEmpty)
	test.That(t, conn3, test.ShouldNotBeNil)
	test.That(t, conn3, test.ShouldNotEqual, conn2)

	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)

	echoClient3 := echopb.NewEchoServiceClient(conn3)
	resp, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	test.That(t, conn2.Close(), test.ShouldBeNil)

	_, _, err = svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("service closed"))
}

func TestCloudManagedWithAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)

	server, err := rpc.NewServer(
		logger,
		rpc.WithAuthHandler(
			utils.CredentialsTypeRobotSecret,
			rpc.MakeSimpleMultiAuthHandler([]string{"foo"}, []string{"bar"}),
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

	conf := &config.Cloud{
		AppAddress: fmt.Sprintf("http://%s", addr),
	}

	svc := cloud.NewCloudConnectionService(conf, logger)
	id, conn1, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id, test.ShouldBeEmpty)
	test.That(t, conn1, test.ShouldNotBeNil)

	id2, conn2, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id2, test.ShouldBeEmpty)
	test.That(t, conn2, test.ShouldNotBeNil)
	test.That(t, conn2, test.ShouldNotEqual, conn1)

	echoClient := echopb.NewEchoServiceClient(conn1)
	_, err = echoClient.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Unauthenticated)

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)

	conf = &config.Cloud{
		AppAddress: fmt.Sprintf("http://%s", addr),
		ID:         "foo",
		Secret:     "bar",
	}

	svc = cloud.NewCloudConnectionService(conf, logger)
	id, conn1, err = svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id, test.ShouldEqual, "foo")
	test.That(t, conn1, test.ShouldNotBeNil)

	id2, conn2, err = svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id2, test.ShouldEqual, "foo")
	test.That(t, conn2, test.ShouldNotBeNil)
	test.That(t, conn2, test.ShouldNotEqual, conn1)

	echoClient1 := echopb.NewEchoServiceClient(conn1)
	echoClient2 := echopb.NewEchoServiceClient(conn2)

	resp, err := echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	conn2.Close()

	// technically conn2 is open via ref count
	resp, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")
	test.That(t, conn1.Close(), test.ShouldBeNil)

	// now both are closed
	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)

	id3, conn3, err := svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, id3, test.ShouldEqual, "foo")
	test.That(t, conn3, test.ShouldNotBeNil)
	test.That(t, conn3, test.ShouldNotEqual, conn2)

	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)

	echoClient3 := echopb.NewEchoServiceClient(conn3)
	resp, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
	_, err = echoClient1.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient2.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	_, err = echoClient3.Echo(context.Background(), &echopb.EchoRequest{
		Message: "hello",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	test.That(t, conn2.Close(), test.ShouldBeNil)

	_, _, err = svc.AcquireConnection(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("service closed"))
}
