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
	var metadata isRegistryItemMetadata
	switch pbMetadata := item.Metadata.(type) {
	case *pb.RegistryItem_ModuleMetadata:
		metadata = &RegistryItemModuleMetadata{ModuleMetadata: moduleMetadataFromProto(pbMetadata.ModuleMetadata)}
	case *pb.RegistryItem_MlModelMetadata:
		metadata = &RegistryItemMLModelMetadata{MlModelMetadata: mlModelMetadataFromProto(pbMetadata.MlModelMetadata)}
	case *pb.RegistryItem_MlTrainingMetadata:
		metadata = &RegistryItemMLTrainingMetadata{MlTrainingMetadata: mlTrainingMetadataFromProto(pbMetadata.MlTrainingMetadata)}
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}

	return &RegistryItem{
		ItemID:                         item.ItemId,
		OrganizationID:                 item.OrganizationId,
		PublicNamespace:                item.PublicNamespace,
		Name:                           item.Name,
		Type:                           packageTypeFromProto(item.Type),
		Visibility:                     visibilityFromProto(item.Visibility),
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

func registryItemStatusToProto(status RegistryItemStatus) (pb.RegistryItemStatus) {
	switch status {
	case RegistryItemStatusUnspecified:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED
	case RegistryItemStatusPublished:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED
	case RegistryItemStatusInDevelopment:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT
	}
	return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED
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

func packageTypeFromProto(packageType packages.PackageType) (PackageType) {
	switch packageType {
	case packages.PackageType_PACKAGE_TYPE_UNSPECIFIED:
		return PackageTypeUnspecified
	case packages.PackageType_PACKAGE_TYPE_ARCHIVE:
		return PackageTypeArchive
	case packages.PackageType_PACKAGE_TYPE_ML_MODEL:
		return PackageTypeMLModel
	case packages.PackageType_PACKAGE_TYPE_MODULE:
		return PackageTypeModule
	case packages.PackageType_PACKAGE_TYPE_SLAM_MAP:
		return PackageTypeSLAMMap
	case packages.PackageType_PACKAGE_TYPE_ML_TRAINING:
		return PackageTypeMLTraining
	}
	return PackageTypeUnspecified
}

func packageTypeToProto(packageType PackageType) (packages.PackageType) {
	switch packageType {
	case PackageTypeUnspecified:
		return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED
	case PackageTypeArchive:
		return packages.PackageType_PACKAGE_TYPE_ARCHIVE
	case PackageTypeMLModel:
		return packages.PackageType_PACKAGE_TYPE_ML_MODEL
	case PackageTypeModule:
		return packages.PackageType_PACKAGE_TYPE_MODULE
	case PackageTypeSLAMMap:
		return packages.PackageType_PACKAGE_TYPE_SLAM_MAP
	case PackageTypeMLTraining:
		return packages.PackageType_PACKAGE_TYPE_ML_TRAINING
	}
	return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED
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

func visibilityFromProto(visibility pb.Visibility) Visibility {
	switch visibility {
	case pb.Visibility_VISIBILITY_UNSPECIFIED:
		return VisibilityUnspecified
	case pb.Visibility_VISIBILITY_PRIVATE:
		return VisibilityPrivate
	case pb.Visibility_VISIBILITY_PUBLIC:
		return VisibilityPublic
	case pb.Visibility_VISIBILITY_PUBLIC_UNLISTED:
		return VisibilityPublicUnlisted
	}
	return VisibilityUnspecified
}

func visibilityToProto(visibility Visibility) (pb.Visibility) {
	switch visibility {
	case VisibilityUnspecified:
		return pb.Visibility_VISIBILITY_UNSPECIFIED
	case VisibilityPrivate:
		return pb.Visibility_VISIBILITY_PRIVATE
	case VisibilityPublic:
		return pb.Visibility_VISIBILITY_PUBLIC
	case VisibilityPublicUnlisted:
		return pb.Visibility_VISIBILITY_PUBLIC_UNLISTED
	}
	return pb.Visibility_VISIBILITY_UNSPECIFIED
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

func mlModelMetadataFromProto(md *pb.MLModelMetadata) (*MLModelMetadata) {
	return &MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
	}
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

func modelTypeFromProto(modelType mlTraining.ModelType) (ModelType) {
	switch modelType {
	case mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED:
		return ModelTypeUnspecified
	case mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION:
		return ModelTypeSingleLabelClassification
	case mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION:
		return ModelTypeMultiLabelClassification
	case mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION:
		return ModelTypeObjectDetection
	}
	return ModelTypeUnspecified
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

func modelFrameworkFromProto(framework mlTraining.ModelFramework) (ModelFramework) {
	switch framework {
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED:
		return ModelFrameworkUnspecified
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE:
		return ModelFrameworkTFLite
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW:
		return ModelFrameworkTensorFlow
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH:
		return ModelFrameworkPyTorch
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX:
		return ModelFrameworkONNX
	}
	return ModelFrameworkUnspecified
}

// MLTrainingMetadata is the metadata of an ML Training.
type MLTrainingMetadata struct {
	Versions       []*MLTrainingVersion
	ModelType      ModelType
	ModelFramework ModelFramework
	Draft          bool
}

func mlTrainingMetadataFromProto(md *pb.MLTrainingMetadata) (*MLTrainingMetadata) {
	var versions []*MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, mlTrainingVersionFromProto(version))
	}
	return &MLTrainingMetadata{
		Versions:       versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
		Draft:          md.Draft,
	}
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
		Visibility:             visibilityFromProto(module.Visibility),
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
