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
	}, map[string]any{moduleBuildFlagPath: manifestPath, moduleBuildFlagVersion: "1.2.3"}, "token")
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
	updateCount := 0
	cCtx, vc, _, _ := setup(&inject.AppServiceClient{
		GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest, opts ...grpc.CallOption) (*apppb.GetRobotPartResponse, error) {
			return &apppb.GetRobotPartResponse{Part: &apppb.RobotPart{
				RobotConfig: confStruct,
				Fqdn:        "restart-module-robot.local",
			}, ConfigJson: ``}, nil
		},
		UpdateRobotPartFunc: func(ctx context.Context, req *apppb.UpdateRobotPartRequest, opts ...grpc.CallOption) (*apppb.UpdateRobotPartResponse, error) {
			updateCount++
			return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
		},
		GetRobotAPIKeysFunc: func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest, opts ...grpc.CallOption) (*apppb.GetRobotAPIKeysResponse, error) {
			return &apppb.GetRobotAPIKeysResponse{ApiKeys: []*apppb.APIKeyWithAuthorizations{
				{ApiKey: &apppb.APIKey{}},
			}}, nil
		},
	}, nil, &inject.BuildServiceClient{},
		map[string]any{moduleBuildFlagPath: manifestPath, partFlag: "part-123", moduleBuildFlagNoBuild: true}, "token")
	test.That(t, vc.loginAction(cCtx), test.ShouldBeNil)
	globalTestClient = vc
	t.Cleanup(func() { globalTestClient = nil })
	err = ReloadModuleAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updateCount, test.ShouldEqual, 1)

	// todo: uncomment or delete before merging
	// send correct config, test restart flow
	// wd, _ := os.Getwd()
	// confStruct, err = structpb.NewStruct(map[string]interface{}{
	// 	"modules": []interface{}{
	// 		map[string]interface{}{
	// 			"name":            "hr_test_test",
	// 			"executable_path": wd + "/bin/module",
	// 		},
	// 	},
	// })
	// test.That(t, err, test.ShouldBeNil)
	// err = ReloadModuleAction(cCtx)
	// test.That(t, err, test.ShouldBeNil)
}
