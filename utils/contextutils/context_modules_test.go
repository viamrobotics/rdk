package contextutils_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mygizmosummer"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils/contextutils"
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
	ctx := context.Background()
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

	r, err := robotimpl.New(ctx, cfg, nil, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, r.Close(ctx), test.ShouldBeNil) }()

	giz, err := gizmoapi.FromProvider(r, "gizmo1")
	test.That(t, err, test.ShouldBeNil)

	callCtx, md := contextutils.ContextWithMetadata(ctx)
	callCtx = contextutils.AppendToOutgoingContext(callCtx,
		"arbitrary-md-from-client", "arbitrary-md-from-client-val1",
		"arbitrary-md-from-client", "arbitrary-md-from-client-val2",
		"opid", "custom",
	)

	_, err = giz.DoOne(callCtx, "1.0")
	test.That(t, err, test.ShouldBeNil)

	// Client to Server sending:
	// client to end md made it to the end
	test.That(t, md["from_client_md_good"], test.ShouldResemble, []string{"true"})

	// middle to end md made it to the end
	test.That(t, md["from_middle_md_good"], test.ShouldResemble, []string{"true"})

	// unknown arbitrary metadata made it to the end (maybe filtering not working)
	test.That(t, md["unknown_metadata_found"], test.ShouldNotResemble, []string{"true"})

	// our (shadowed) opid made it to the end
	test.That(t, md["custom_opid_good"], test.ShouldResemble, []string{"true"})

	// Server to Client sending:
	// end to client md made it to client
	test.That(t, md["arbitrary-md-to-client-from-end"], test.ShouldResemble, []string{"arbitrary-md-to-client-from-end-val1"})

	// end to client md was updated by middle and both made it to client
	test.That(t, md["arbitrary-md-to-client-from-end2"], test.ShouldContain, "arbitrary-md-to-client-from-end2-val1")
	test.That(t, md["arbitrary-md-to-client-from-end2"], test.ShouldContain, "arbitrary-md-to-client-from-end2-val-from-middle")
	// no dups
	test.That(t, len(md["arbitrary-md-to-client-from-end2"]), test.ShouldResemble, 2)

	// middle to client md made it to client
	test.That(t, md["arbitrary-md-to-client-from-middle"], test.ShouldResemble, []string{"arbitrary-md-to-client-from-middle"})
}
