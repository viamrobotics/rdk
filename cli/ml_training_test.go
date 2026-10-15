package cli

import (
	"context"
	"testing"

	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagespb "go.viam.com/api/app/packages/v1"
	v1 "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestManagedTrainingRejectsRegistryFramework(t *testing.T) {
	_, err := (&viamClient{}).mlSubmitTrainingJob(
		"dataset", "org", "model", "version", string(ModelTypeObjectDetection), string(ModelFrameworkRegistry), []string{"cat"},
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "managed training does not support")
}

func TestValidateRegistryModuleResources(t *testing.T) {
	t.Run("registry with model triples", func(t *testing.T) {
		err := validateRegistryModuleResources(mlTrainingFlagFramework, string(ModelFrameworkRegistry), []string{
			"viam-labs:arm-policy:policy",
			"acme:mod:model",
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("registry requires a resource", func(t *testing.T) {
		err := validateRegistryModuleResources(mlTrainingFlagFramework, string(ModelFrameworkRegistry), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "--module-resource is required when --framework is registry")
	})

	t.Run("other frameworks reject resources", func(t *testing.T) {
		err := validateRegistryModuleResources(packageFlagFramework, string(ModelFrameworkTFLite), []string{"a:b:c"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "--module-resource requires --model-framework registry")
	})

	t.Run("triple must have three segments", func(t *testing.T) {
		for _, resource := range []string{"not-a-triple", "a:b:", ":b:c", "a::c", "a:b:c:d"} {
			err := validateRegistryModuleResources(mlTrainingFlagFramework, string(ModelFrameworkRegistry), []string{resource})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "model triple")
		}
	})

	t.Run("non-registry without resources", func(t *testing.T) {
		err := validateRegistryModuleResources(mlTrainingFlagFramework, string(ModelFrameworkONNX), nil)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestCreateMetadataRegistryResources(t *testing.T) {
	metadata, err := createMetadata(false, string(ModelTypeObjectDetection), string(ModelFrameworkRegistry), "", "private", []string{
		"viam-labs:arm-policy:policy",
		"acme:mod:model",
	})
	test.That(t, err, test.ShouldBeNil)

	metadataStruct, err := convertMetadataToStruct(*metadata)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, metadataStruct.Fields[modelFrameworkKey].GetStringValue(), test.ShouldEqual, string(ModelFrameworkRegistry))
	values := metadataStruct.Fields[deploymentModularResourcesKey].GetListValue().GetValues()
	test.That(t, values, test.ShouldHaveLength, 2)
	test.That(t, values[0].GetStringValue(), test.ShouldEqual, "viam-labs:arm-policy:policy")
	test.That(t, values[1].GetStringValue(), test.ShouldEqual, "acme:mod:model")

	metadata, err = createMetadata(false, "", string(ModelFrameworkTFLite), "", "", nil)
	test.That(t, err, test.ShouldBeNil)
	metadataStruct, err = convertMetadataToStruct(*metadata)
	test.That(t, err, test.ShouldBeNil)
	_, ok := metadataStruct.Fields[deploymentModularResourcesKey]
	test.That(t, ok, test.ShouldBeFalse)

	_, err = createMetadata(false, "", string(ModelFrameworkRegistry), "", "", nil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestPackageUploadRegistryMetadata(t *testing.T) {
	err := validatePackageUploadRequest(nil, packageUploadArgs{
		Type:           string(PackageTypeMLModel),
		ModelFramework: string(ModelFrameworkRegistry),
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "--module-resource is required when --model-framework is registry")

	args := packageUploadArgs{
		Type:           string(PackageTypeMLModel),
		ModelFramework: string(ModelFrameworkRegistry),
		ModelType:      string(ModelTypeObjectDetection),
		ModuleResource: []string{"viam-labs:arm-policy:policy"},
	}
	test.That(t, validatePackageUploadRequest(nil, args), test.ShouldBeNil)
	metadata := packageUploadMetadata(args)
	test.That(t, metadata.Fields["model_framework"].GetStringValue(), test.ShouldEqual, string(ModelFrameworkRegistry))
	values := metadata.Fields[deploymentModularResourcesKey].GetListValue().GetValues()
	test.That(t, values, test.ShouldHaveLength, 1)
	test.That(t, values[0].GetStringValue(), test.ShouldEqual, "viam-labs:arm-policy:policy")

	tflite := packageUploadMetadata(packageUploadArgs{ModelFramework: string(ModelFrameworkTFLite)})
	_, ok := tflite.Fields[deploymentModularResourcesKey]
	test.That(t, ok, test.ShouldBeFalse)
}

func TestUpdateTrainingScriptModuleResources(t *testing.T) {
	var got *v1.UpdateRegistryItemRequest
	asc := &inject.AppServiceClient{
		GetRegistryItemFunc: func(
			ctx context.Context, in *v1.GetRegistryItemRequest, opts ...grpc.CallOption,
		) (*v1.GetRegistryItemResponse, error) {
			return &v1.GetRegistryItemResponse{Item: &v1.RegistryItem{
				Type:        packagespb.PackageType_PACKAGE_TYPE_ML_TRAINING,
				Description: "keep me",
				Metadata: &v1.RegistryItem_MlTrainingMetadata{
					MlTrainingMetadata: &v1.MLTrainingMetadata{
						ModelType:                  mltrainingpb.ModelType_MODEL_TYPE_OBJECT_DETECTION,
						ModelFramework:             mltrainingpb.ModelFramework_MODEL_FRAMEWORK_PYTORCH,
						Draft:                      true,
						DeploymentModularResources: []string{"old:mod:model"},
					},
				},
			}}, nil
		},
		UpdateRegistryItemFunc: func(
			ctx context.Context, in *v1.UpdateRegistryItemRequest, opts ...grpc.CallOption,
		) (*v1.UpdateRegistryItemResponse, error) {
			got = in
			return &v1.UpdateRegistryItemResponse{}, nil
		},
	}
	_, client, out, errOut := setup(asc, nil, nil, nil, "token")
	test.That(t, out.messages, test.ShouldBeEmpty)
	test.That(t, errOut.messages, test.ShouldBeEmpty)

	err := client.updateTrainingScript(context.Background(), "org", "script", "private", "", "", "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.GetMetadata(), test.ShouldBeNil)
	test.That(t, got.GetDescription(), test.ShouldEqual, "keep me")

	err = client.updateTrainingScript(
		context.Background(), "org", "script", "private", "", "",
		string(ModelFrameworkRegistry), []string{"viam-labs:arm-policy:policy"},
	)
	test.That(t, err, test.ShouldBeNil)
	meta := got.GetUpdateMlTrainingMetadata()
	test.That(t, meta.GetModelFramework(), test.ShouldEqual, mltrainingpb.ModelFramework_MODEL_FRAMEWORK_REGISTRY)
	test.That(t, meta.GetDeploymentModularResources(), test.ShouldResemble, []string{"viam-labs:arm-policy:policy"})
	test.That(t, meta.GetModelType(), test.ShouldEqual, mltrainingpb.ModelType_MODEL_TYPE_OBJECT_DETECTION)
	test.That(t, meta.GetDraft(), test.ShouldBeTrue)

	err = client.updateTrainingScript(
		context.Background(), "org", "script", "private", "", "",
		string(ModelFrameworkTFLite), []string{"a:b:c"},
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "--module-resource requires --framework registry")
}
