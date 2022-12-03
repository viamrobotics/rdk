package module_test

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	v1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/module"
)

func TestModuleBasic(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	addr, err := os.MkdirTemp("", "viam-test")
	test.That(t, err, test.ShouldBeNil)
	addr += "/mod.sock"

	m, err := module.NewModule(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	err = m.AddModelFromRegistry(ctx, gizmoapi.Subtype, mygizmo.Model)
	test.That(t, err, test.ShouldBeNil)

	err = m.Start(ctx)
	test.That(t, err, test.ShouldBeNil)

	conn, err := grpc.Dial(
		"unix://"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	test.That(t, err, test.ShouldBeNil)

	client := pb.NewModuleServiceClient(conn)

	resp, err := client.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ready, test.ShouldBeTrue)

	hmap := resp.GetHandlermap().GetHandlers()

	test.That(t, hmap[0].Subtype.Subtype.Namespace, test.ShouldEqual, "acme")
	test.That(t, hmap[0].Subtype.Subtype.Type, test.ShouldEqual, "component")
	test.That(t, hmap[0].Subtype.Subtype.Subtype, test.ShouldEqual, "gizmo")
	test.That(t, hmap[0].GetModels()[0], test.ShouldEqual, "acme:demo:mygizmo")

	addReq := &pb.AddResourceRequest{
		Config: &v1.ComponentConfig{
			Name:  "gizmo1",
			Api:   "acme:component:gizmo",
			Model: "acme:demo:mygizmo",
		},
	}

	_, err = m.AddResource(ctx, addReq)
	test.That(t, err, test.ShouldBeNil)

	gClient := gizmoapi.NewClientFromConn(conn, "gizmo1", logger)

	ret, err := gClient.DoOne(ctx, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldBeFalse)

	ret, err = gClient.DoOne(ctx, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldBeTrue)

	err = utils.TryClose(ctx, gClient)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, conn.Close(), test.ShouldBeNil)
	m.Close()
}
