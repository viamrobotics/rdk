package cli

import (
	"context"
	"testing"

	v1 "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/testutils/inject"
)

func TestConfigureModule(t *testing.T) {
	manifestPath := createTestManifest(t, "")
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, &map[string]any{moduleBuildFlagPath: manifestPath, moduleBuildFlagVersion: "1.2.3"}, "token")
	err := ac.moduleBuildStartAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 1)
}

// func TestMutateModuleConfig(t *testing.T) {
// 	panic("hi")
// }

func TestFullReloadFlow(t *testing.T) {
	manifestPath := createTestManifest(t, "")
	confStruct, err := structpb.NewStruct(map[string]interface{}{
		"modules": []interface{}{},
	})
	test.That(t, err, test.ShouldBeNil)
	cCtx, vc, _, _ := setup(&inject.AppServiceClient{
		GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest, opts ...grpc.CallOption) (*apppb.GetRobotPartResponse, error) {
			return &apppb.GetRobotPartResponse{Part: &apppb.RobotPart{
				RobotConfig: confStruct,
			}, ConfigJson: ``}, nil
		},
		UpdateRobotPartFunc: func(ctx context.Context, req *apppb.UpdateRobotPartRequest, opts ...grpc.CallOption) (*apppb.UpdateRobotPartResponse, error) {
			return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
		},
	}, nil, &inject.BuildServiceClient{},
		&map[string]any{moduleBuildFlagPath: manifestPath, moduleBuildFlagVersion: "1.2.3"}, "token")
	test.That(t, vc.loginAction(cCtx), test.ShouldBeNil)
	globalTestClient = vc
	t.Cleanup(func() { globalTestClient = nil })
	err = ReloadModuleAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
}
