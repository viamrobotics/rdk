package app

import (
	"fmt"

	mlTraining "go.viam.com/api/app/mltraining/v1"
	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RegistryItem has the information of an item in the registry.
type RegistryItem struct {
	ItemID                         string
	OrganizationID                 string
	PublicNamespace                string
	Name                           string
	Type                           PackageType
	Visibility                     Visibility
	URL                            string
	Description                    string
	TotalRobotUsage                int64
	TotalExternalRobotUsage        int64
	TotalOrganizationUsage         int64
	TotalExternalOrganizationUsage int64
	Metadata                       isRegistryItemMetadata
	CreatedAt                      *timestamppb.Timestamp
	UpdatedAt                      *timestamppb.Timestamp
}

func registryItemFromProto(item *pb.RegistryItem) (*RegistryItem, error) {
	packageType, err := packageTypeFromProto(item.Type)
	if err != nil {
		return nil, err
	}
	visibility, err := visibilityFromProto(item.Visibility)
	if err != nil {
		return nil, err
	}

	var metadata isRegistryItemMetadata
	switch pbMetadata := item.Metadata.(type) {
	case *pb.RegistryItem_ModuleMetadata:
		md := moduleMetadataFromProto(pbMetadata.ModuleMetadata)
		metadata = &RegistryItemModuleMetadata{ModuleMetadata: md}
	case *pb.RegistryItem_MlModelMetadata:
		md, err := mlModelMetadataFromProto(pbMetadata.MlModelMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItemMLModelMetadata{MlModelMetadata: md}
	case *pb.RegistryItem_MlTrainingMetadata:
		md, err := mlTrainingMetadataFromProto(pbMetadata.MlTrainingMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItemMLTrainingMetadata{MlTrainingMetadata: md}
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}

	return &RegistryItem{
		ItemID:                         item.ItemId,
		OrganizationID:                 item.OrganizationId,
		PublicNamespace:                item.PublicNamespace,
		Name:                           item.Name,
		Type:                           packageType,
		Visibility:                     visibility,
		URL:                            item.Url,
		Description:                    item.Description,
		TotalRobotUsage:                item.TotalRobotUsage,
		TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
		TotalOrganizationUsage:         item.TotalOrganizationUsage,
		TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
		Metadata:                       metadata,
		CreatedAt:                      item.CreatedAt,
		UpdatedAt:                      item.UpdatedAt,
	}, nil
}

// RegistryItemStatus specifies if a registry item is published or in development.
type RegistryItemStatus int32

const (
	// RegistryItemStatusUnspecified is an unspecified registry item status.
	RegistryItemStatusUnspecified RegistryItemStatus = iota
	// RegistryItemStatusPublished represents a published registry item.
	RegistryItemStatusPublished
	// RegistryItemStatusInDevelopment represents a registry item still in development.
	RegistryItemStatusInDevelopment
)

func registryItemStatusToProto(status RegistryItemStatus) (pb.RegistryItemStatus, error) {
	switch status {
	case RegistryItemStatusUnspecified:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED, nil
	case RegistryItemStatusPublished:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED, nil
	case RegistryItemStatusInDevelopment:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT, nil
	default:
		return 0, fmt.Errorf("unknown registry item status: %v", status)
	}
}

// PackageType is the type of package being used.
type PackageType int32

const (
	// PackageTypeUnspecified represents an unspecified package type.
	PackageTypeUnspecified PackageType = iota
	// PackageTypeArchive represents an archive package type.
	PackageTypeArchive
	// PackageTypeMLModel represents a ML model package type.
	PackageTypeMLModel
	// PackageTypeModule represents a module package type.
	PackageTypeModule
	// PackageTypeSLAMMap represents a SLAM map package type.
	PackageTypeSLAMMap
	// PackageTypeMLTraining represents a ML training package type.
	PackageTypeMLTraining
)

func packageTypeFromProto(packageType packages.PackageType) (PackageType, error) {
	switch packageType {
	case packages.PackageType_PACKAGE_TYPE_UNSPECIFIED:
		return PackageTypeUnspecified, nil
	case packages.PackageType_PACKAGE_TYPE_ARCHIVE:
		return PackageTypeArchive, nil
	case packages.PackageType_PACKAGE_TYPE_ML_MODEL:
		return PackageTypeMLModel, nil
	case packages.PackageType_PACKAGE_TYPE_MODULE:
		return PackageTypeModule, nil
	case packages.PackageType_PACKAGE_TYPE_SLAM_MAP:
		return PackageTypeSLAMMap, nil
	case packages.PackageType_PACKAGE_TYPE_ML_TRAINING:
		return PackageTypeMLTraining, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", packageType)
	}
}

func packageTypeToProto(packageType PackageType) (packages.PackageType, error) {
	switch packageType {
	case PackageTypeUnspecified:
		return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED, nil
	case PackageTypeArchive:
		return packages.PackageType_PACKAGE_TYPE_ARCHIVE, nil
	case PackageTypeMLModel:
		return packages.PackageType_PACKAGE_TYPE_ML_MODEL, nil
	case PackageTypeModule:
		return packages.PackageType_PACKAGE_TYPE_MODULE, nil
	case PackageTypeSLAMMap:
		return packages.PackageType_PACKAGE_TYPE_SLAM_MAP, nil
	case PackageTypeMLTraining:
		return packages.PackageType_PACKAGE_TYPE_ML_TRAINING, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", packageType)
	}
}

// Visibility specifies the type of visibility of a registry item.
type Visibility int32

const (
	// VisibilityUnspecified represents an unspecified visibility.
	VisibilityUnspecified Visibility = iota
	// VisibilityPrivate are for registry items visible only within the owning org.
	VisibilityPrivate
	// VisibilityPublic are for registry items that are visible to everyone.
	VisibilityPublic
	// VisibilityPublicUnlisted are for registry items usable in everyone's robot but are hidden from the registry page as if they are private.
	VisibilityPublicUnlisted
)

func visibilityFromProto(visibility pb.Visibility) (Visibility, error) {
	switch visibility {
	case pb.Visibility_VISIBILITY_UNSPECIFIED:
		return VisibilityUnspecified, nil
	case pb.Visibility_VISIBILITY_PRIVATE:
		return VisibilityPrivate, nil
	case pb.Visibility_VISIBILITY_PUBLIC:
		return VisibilityPublic, nil
	case pb.Visibility_VISIBILITY_PUBLIC_UNLISTED:
		return VisibilityPublicUnlisted, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", visibility)
	}
}

func visibilityToProto(visibility Visibility) (pb.Visibility, error) {
	switch visibility {
	case VisibilityUnspecified:
		return pb.Visibility_VISIBILITY_UNSPECIFIED, nil
	case VisibilityPrivate:
		return pb.Visibility_VISIBILITY_PRIVATE, nil
	case VisibilityPublic:
		return pb.Visibility_VISIBILITY_PUBLIC, nil
	case VisibilityPublicUnlisted:
		return pb.Visibility_VISIBILITY_PUBLIC_UNLISTED, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", visibility)
	}
}

type isRegistryItemMetadata interface {
	isRegistryItemMetadata()
}

// RegistryItemModuleMetadata is a registry item's module metadata.
type RegistryItemModuleMetadata struct {
	ModuleMetadata *ModuleMetadata
}

// RegistryItemMLModelMetadata is a registry item's ML model metadata.
type RegistryItemMLModelMetadata struct {
	MlModelMetadata *MLModelMetadata
}

// RegistryItemMLTrainingMetadata is a registry item's ML Training metadata.
type RegistryItemMLTrainingMetadata struct {
	MlTrainingMetadata *MLTrainingMetadata
}

func (*RegistryItemModuleMetadata) isRegistryItemMetadata() {}

func (*RegistryItemMLModelMetadata) isRegistryItemMetadata() {}

func (*RegistryItemMLTrainingMetadata) isRegistryItemMetadata() {}

// ModuleMetadata holds the metadata of a module.
type ModuleMetadata struct {
	Models     []*Model
	Versions   []*ModuleVersion
	Entrypoint string
	FirstRun   *string
}

func moduleMetadataFromProto(md *pb.ModuleMetadata) *ModuleMetadata {
	var models []*Model
	for _, version := range md.Models {
		models = append(models, modelFromProto(version))
	}
	var versions []*ModuleVersion
	for _, version := range md.Versions {
		versions = append(versions, moduleVersionFromProto(version))
	}
	return &ModuleMetadata{
		Models:     models,
		Versions:   versions,
		Entrypoint: md.Entrypoint,
		FirstRun:   md.FirstRun,
	}
}

// Model has the API and model of a model.
type Model struct {
	API   string
	Model string
}

func modelFromProto(model *pb.Model) *Model {
	return &Model{
		API:   model.Api,
		Model: model.Model,
	}
}

func modelToProto(model *Model) *pb.Model {
	return &pb.Model{
		Api:   model.API,
		Model: model.Model,
	}
}

// ModuleVersion holds the information of a module version.
type ModuleVersion struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
}

func moduleVersionFromProto(version *pb.ModuleVersion) *ModuleVersion {
	var files []*Uploads
	for _, file := range version.Files {
		files = append(files, uploadsFromProto(file))
	}
	var models []*Model
	for _, model := range version.Models {
		models = append(models, modelFromProto(model))
	}
	return &ModuleVersion{
		Version:    version.Version,
		Files:      files,
		Models:     models,
		Entrypoint: version.Entrypoint,
		FirstRun:   version.FirstRun,
	}
}

// Uploads holds the time the file was uploaded and the OS and architecture a module is built to run on.
type Uploads struct {
	Platform   string
	UploadedAt *timestamppb.Timestamp
}

func uploadsFromProto(uploads *pb.Uploads) *Uploads {
	return &Uploads{
		Platform:   uploads.Platform,
		UploadedAt: uploads.UploadedAt,
	}
}

// MLModelMetadata holds the metadata for a ML model.
type MLModelMetadata struct {
	Versions       []string
	ModelType      ModelType
	ModelFramework ModelFramework
}

func mlModelMetadataFromProto(md *pb.MLModelMetadata) (*MLModelMetadata, error) {
	modelType, err := modelTypeFromProto(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := modelFrameworkFromProto(md.ModelFramework)
	if err != nil {
		return nil, err
	}
	return &MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelType,
		ModelFramework: modelFramework,
	}, nil
}

// ModelType specifies the type of model used for classification or detection.
type ModelType int32

const (
	// ModelTypeUnspecified represents an unspecified model.
	ModelTypeUnspecified ModelType = iota
	// ModelTypeSingleLabelClassification represents a single-label classification model.
	ModelTypeSingleLabelClassification
	// ModelTypeMultiLabelClassification represents a multi-label classification model.
	ModelTypeMultiLabelClassification
	// ModelTypeObjectDetection represents an object detection model.
	ModelTypeObjectDetection
)

func modelTypeFromProto(modelType mlTraining.ModelType) (ModelType, error) {
	switch modelType {
	case mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED:
		return ModelTypeUnspecified, nil
	case mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION:
		return ModelTypeSingleLabelClassification, nil
	case mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION:
		return ModelTypeMultiLabelClassification, nil
	case mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION:
		return ModelTypeObjectDetection, nil
	default:
		return 0, fmt.Errorf("unknown model type: %v", modelType)
	}
}

// ModelFramework is the framework type of a model.
type ModelFramework int32

const (
	// ModelFrameworkUnspecified is an unspecified model framework.
	ModelFrameworkUnspecified ModelFramework = iota
	// ModelFrameworkTFLite specifies a TFLite model framework.
	ModelFrameworkTFLite
	// ModelFrameworkTensorFlow specifies a TensorFlow model framework.
	ModelFrameworkTensorFlow
	// ModelFrameworkPyTorch specifies a PyTorch model framework.
	ModelFrameworkPyTorch
	// ModelFrameworkONNX specifies a ONNX model framework.
	ModelFrameworkONNX
)

func modelFrameworkFromProto(framework mlTraining.ModelFramework) (ModelFramework, error) {
	switch framework {
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED:
		return ModelFrameworkUnspecified, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE:
		return ModelFrameworkTFLite, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW:
		return ModelFrameworkTensorFlow, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH:
		return ModelFrameworkPyTorch, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX:
		return ModelFrameworkONNX, nil
	default:
		return 0, fmt.Errorf("unknown model framework: %v", framework)
	}
}

// MLTrainingMetadata is the metadata of an ML Training.
type MLTrainingMetadata struct {
	Versions       []*MLTrainingVersion
	ModelType      ModelType
	ModelFramework ModelFramework
	Draft          bool
}

func mlTrainingMetadataFromProto(md *pb.MLTrainingMetadata) (*MLTrainingMetadata, error) {
	var versions []*MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, mlTrainingVersionFromProto(version))
	}
	modelType, err := modelTypeFromProto(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := modelFrameworkFromProto(md.ModelFramework)
	if err != nil {
		return nil, err
	}
	return &MLTrainingMetadata{
		Versions:       versions,
		ModelType:      modelType,
		ModelFramework: modelFramework,
		Draft:          md.Draft,
	}, nil
}

// MLTrainingVersion is the version of ML Training.
type MLTrainingVersion struct {
	Version   string
	CreatedOn *timestamppb.Timestamp
}

func mlTrainingVersionFromProto(version *pb.MLTrainingVersion) *MLTrainingVersion {
	return &MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: version.CreatedOn,
	}
}

// Module holds the information of a module.
type Module struct {
	ModuleID               string
	Name                   string
	Visibility             Visibility
	Versions               []*VersionHistory
	URL                    string
	Description            string
	Models                 []*Model
	TotalRobotUsage        int64
	TotalOrganizationUsage int64
	OrganizationID         string
	Entrypoint             string
	PublicNamespace        string
	FirstRun               *string
}

func moduleFromProto(module *pb.Module) (*Module, error) {
	visibility, err := visibilityFromProto(module.Visibility)
	if err != nil {
		return nil, err
	}
	var versions []*VersionHistory
	for _, version := range module.Versions {
		versions = append(versions, versionHistoryFromProto(version))
	}
	var models []*Model
	for _, model := range module.Models {
		models = append(models, modelFromProto(model))
	}
	return &Module{
		ModuleID:               module.ModuleId,
		Name:                   module.Name,
		Visibility:             visibility,
		Versions:               versions,
		URL:                    module.Url,
		Description:            module.Description,
		Models:                 models,
		TotalRobotUsage:        module.TotalRobotUsage,
		TotalOrganizationUsage: module.TotalOrganizationUsage,
		OrganizationID:         module.OrganizationId,
		Entrypoint:             module.Entrypoint,
		PublicNamespace:        module.PublicNamespace,
		FirstRun:               module.FirstRun,
	}, nil
}

// VersionHistory holds the history of a version.
type VersionHistory struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
}

func versionHistoryFromProto(history *pb.VersionHistory) *VersionHistory {
	var files []*Uploads
	for _, file := range history.Files {
		files = append(files, uploadsFromProto(file))
	}
	var models []*Model
	for _, model := range history.Models {
		models = append(models, modelFromProto(model))
	}
	return &VersionHistory{
		Version:    history.Version,
		Files:      files,
		Models:     models,
		Entrypoint: history.Entrypoint,
		FirstRun:   history.FirstRun,
	}
}
