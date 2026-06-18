package metadata_test

import (
	"context"
	"maps"
	"testing"

	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mygizmosummer"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils/contextutils/metadata"
)

// TestMetadataAcrossTwoModules tests that arbitrary user metadata correctly flows in both
// directions, and that the merging logic works. It calls mygizmosummer's DoOne, which next calls
// mysum's Sum.
//
// The expectation is that a) client-to-server arbitrary metadata can move from the client (this test) to mygizmosummer to mysum
// b) server-to-client arbitrary metadata can move from mysum to mygizmosummer to this client.
// Since metadata is a map of string to []string, the middle hop should be able to append to a key from its previous hop,
// and there should be no duplicated values unless requested.
//
// It first works by sending the client-to-server arbitrary metadata. At the end, only if everything looks good, does mysum send back
// the server-to-client metadata required for this test to pass.
func TestMetadataAcrossTwoModules(t *testing.T) {
	ctxWithoutMD := context.Background()
	logger := logging.NewTestLogger(t)

	gizmoModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/gizmomodule")
	sumModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/summationmodule")

	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "GizmoModule", ExePath: gizmoModPath},
			{Name: "SummationModule", ExePath: sumModPath},
		},
		Services: []resource.Config{
			{API: summationapi.API, Model: mysum.Model, Name: "adder"},
		},
		Components: []resource.Config{{
			API:        gizmoapi.API,
			Model:      mygizmosummer.Model,
			Name:       "gizmo1",
			Attributes: map[string]interface{}{"Summer": "adder"},
		}},
	}

	r, err := robotimpl.New(ctxWithoutMD, cfg, nil, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, r.Close(ctxWithoutMD), test.ShouldBeNil) }()

	giz, err := gizmoapi.FromProvider(r, "gizmo1")
	test.That(t, err, test.ShouldBeNil)

	ctxWithMD := metadata.Set(ctxWithoutMD,
		"arbitrary-md-from-client", "arbitrary-md-from-client-val1",
		"arbitrary-md-from-client2", "arbitrary-md-from-client-val2",
		"arbitrary-md-local-func-modify", "tbd",
		"to-be-deleted", "delete",
		"opid", "custom",
	)

	localFunc := func(ctx context.Context) context.Context {
		ctx = context.WithValue(ctx, "arbitrary-md-from-client", "fake")      //nolint
		ctx = context.WithValue(ctx, "arbitrary-md-from-client-fake", "fake") //nolint
		ctx = grpcmetadata.AppendToOutgoingContext(ctx, "arbitrary-md-from-client", "fake")
		ctx = grpcmetadata.AppendToOutgoingContext(ctx, "arbitrary-md-from-client-fake", "fake")
		// test local replace
		ctx = metadata.Set(ctx, "arbitrary-md-local-func-modify", "real")
		md := maps.Collect(metadata.All(ctx))
		test.That(t, len(md), test.ShouldEqual, 5)
		test.That(t, md["arbitrary-md-from-client"], test.ShouldEqual, "arbitrary-md-from-client-val1")
		test.That(t, md["opid"], test.ShouldEqual, "custom")
		test.That(t, md["arbitrary-md-local-func-modify"], test.ShouldEqual, "real")
		return ctx
	}
	ctxWithMD = localFunc(localFunc(ctxWithMD))

	// test FromContext should return a clone.
	// clearing it should have no effect on the map in the context, which will be used for the remainder of the test
	mdCopy, ok := metadata.FromContext(ctxWithMD)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(mdCopy), test.ShouldEqual, len(maps.Collect(metadata.All(ctxWithMD))))
	clear(mdCopy)
	test.That(t, len(mdCopy), test.ShouldNotEqual, len(maps.Collect(metadata.All(ctxWithMD))))

	// test deleting
	v, ok := metadata.Get(ctxWithMD, "to-be-deleted")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, v, test.ShouldEqual, "delete")

	test.That(t, len(maps.Collect(metadata.All(ctxWithMD))), test.ShouldEqual, 5)
	ctxWithMD = metadata.Delete(ctxWithMD, "to-be-deleted")
	test.That(t, len(maps.Collect(metadata.All(ctxWithMD))), test.ShouldEqual, 4)

	v, ok = metadata.Get(ctxWithMD, "to-be-deleted")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, v, test.ShouldEqual, "")

	// Unary: Client to Server sending (see mysum.go for the checks):
	// client to end md made it to the end
	// client to end md made that was modified twice by localFunc it to the end
	// middle to end md made it to the end
	// no unknown arbitrary metadata made it to the end (filtering working)
	// our (shadowed) opid made it to the end
	_, err = giz.DoOne(ctxWithMD, "1.0")
	test.That(t, err, test.ShouldResemble, status.Error(codes.Unknown, "TestMetadataAcrossTwoModules-good"))

	// ServerStream: Client to Server sending (see mygizmosummer.go for the checks):
	// client to server MD made it to the end
	// no unknown arbitrary metadata made it to the end (filtering working)
	// our (shadowed) opid made it to the end
	_, err = giz.DoOneServerStream(ctxWithMD, "1.0")
	test.That(t, err, test.ShouldResemble, status.Error(codes.Unknown, "TestMetadataAcrossTwoModules-ServerStream-good"))

	// ClientStream: Client to Server sending (see mygizmosummer.go for the checks):
	// client to server MD made it to the end
	// no unknown arbitrary metadata made it to the end (filtering working)
	// our (shadowed) opid made it to the end
	_, err = giz.DoOneClientStream(ctxWithMD, []string{"1.0"})
	test.That(t, err, test.ShouldResemble, status.Error(codes.Unknown, "TestMetadataAcrossTwoModules-ClientStream-good"))

	// BiDiStream: Client to Server sending (see mygizmosummer.go for the checks):
	// client to server MD made it to the end
	// no unknown arbitrary metadata made it to the end (filtering working)
	// our (shadowed) opid made it to the end
	_, err = giz.DoOneBiDiStream(ctxWithMD, []string{"1.0"})
	test.That(t, err, test.ShouldResemble, status.Error(codes.Unknown, "TestMetadataAcrossTwoModules-BiDiStream-good"))
}
