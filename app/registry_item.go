package app

import (
	"fmt"

	mlTraining "go.viam.com/api/app/mltraining/v1"
	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

func ProtoToRegistryItem(item *pb.RegistryItem) (*RegistryItem, error) {
	packageType, err := ProtoToPackageType(item.Type)
	if err != nil {
		return nil, err
	}
	visibility, err := ProtoToVisibility(item.Visibility)
	if err != nil {
		return nil, err
	}

	var metadata isRegistryItemMetadata
	switch pbMetadata := item.Metadata.(type) {
	case *pb.RegistryItem_ModuleMetadata:
		md, err := ProtoToModuleMetadata(pbMetadata.ModuleMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItemModuleMetadata{ModuleMetadata: md}
	case *pb.RegistryItem_MlModelMetadata:
		md, err := ProtoToMLModelMetadata(pbMetadata.MlModelMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItemMLModelMetadata{MlModelMetadata: md}
	case *pb.RegistryItem_MlTrainingMetadata:
		md, err := ProtoToMLTrainingMetadata(pbMetadata.MlTrainingMetadata)
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

func RegistryItemToProto(item *RegistryItem) (*pb.RegistryItem, error) {
	packageType, err := PackageTypeToProto(item.Type)
	if err != nil {
		return nil, err
	}
	visibility, err := VisibilityToProto(item.Visibility)
	if err != nil {
		return nil, err
	}

	switch md := item.Metadata.(type) {
	case *RegistryItemModuleMetadata:
		protoMetadata, err := ModuleMetadataToProto(md.ModuleMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageType,
			Visibility:                     visibility,
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_ModuleMetadata{ModuleMetadata: protoMetadata},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	case *RegistryItemMLModelMetadata:
		protoMetadata, err := MLModelMetadataToProto(md.MlModelMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageType,
			Visibility:                     visibility,
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_MlModelMetadata{MlModelMetadata: protoMetadata},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	case *RegistryItemMLTrainingMetadata:
		protoMetadata, err := MLTrainingMetadataToProto(md.MlTrainingMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageType,
			Visibility:                     visibility,
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_MlTrainingMetadata{MlTrainingMetadata: protoMetadata},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}
}

type RegistryItemStatus int32

const (
	RegistryItemStatusUnspecified   RegistryItemStatus = 0
	RegistryItemStatusPublished     RegistryItemStatus = 1
	RegistryItemStatusInDevelopment RegistryItemStatus = 2
)

func ProtoToRegistryItemStatus(status pb.RegistryItemStatus) (RegistryItemStatus, error) {
	switch status {
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED:
		return RegistryItemStatusUnspecified, nil
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED:
		return RegistryItemStatusPublished, nil
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT:
		return RegistryItemStatusInDevelopment, nil
	default:
		return 0, fmt.Errorf("unknown registry item status: %v", status)
	}
}

func RegistryItemStatusToProto(status RegistryItemStatus) (pb.RegistryItemStatus, error) {
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

type PackageType int32

const (
	PackageTypeUnspecified PackageType = 0
	PackageTypeArchive     PackageType = 1
	PackageTypeMLModel     PackageType = 2
	PackageTypeModule      PackageType = 3
	PackageTypeSLAMMap     PackageType = 4
	PackageTypeMLTraining  PackageType = 5
)

func ProtoToPackageType(packageType packages.PackageType) (PackageType, error) {
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

func PackageTypeToProto(packageType PackageType) (packages.PackageType, error) {
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

type Visibility int32

const (
	VisibilityUnspecified    Visibility = 0
	VisibilityPrivate        Visibility = 1
	VisibilityPublic         Visibility = 2
	VisibilityPublicUnlisted Visibility = 3
)

func ProtoToVisibility(visibility pb.Visibility) (Visibility, error) {
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

func VisibilityToProto(visibility Visibility) (pb.Visibility, error) {
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

type RegistryItemModuleMetadata struct {
	ModuleMetadata *ModuleMetadata
}

type RegistryItemMLModelMetadata struct {
	MlModelMetadata *MLModelMetadata
}

type RegistryItemMLTrainingMetadata struct {
	MlTrainingMetadata *MLTrainingMetadata
}

func (*RegistryItemModuleMetadata) isRegistryItemMetadata() {}

func (*RegistryItemMLModelMetadata) isRegistryItemMetadata() {}

func (*RegistryItemMLTrainingMetadata) isRegistryItemMetadata() {}

type ModuleMetadata struct {
	Models     []*Model
	Versions   []*ModuleVersion
	Entrypoint string
	FirstRun   *string
}

func ProtoToModuleMetadata(md *pb.ModuleMetadata) (*ModuleMetadata, error) {
	var models []*Model
	for _, version := range md.Models {
		models = append(models, ProtoToModel(version))
	}
	var versions []*ModuleVersion
	for _, version := range md.Versions {
		versions = append(versions, ProtoToModuleVersion(version))
	}
	return &ModuleMetadata{
		Models:     models,
		Versions:   versions,
		Entrypoint: md.Entrypoint,
		FirstRun:   md.FirstRun,
	}, nil
}

func ModuleMetadataToProto(md *ModuleMetadata) (*pb.ModuleMetadata, error) {
	var models []*pb.Model
	for _, version := range md.Models {
		models = append(models, ModelToProto(version))
	}
	var versions []*pb.ModuleVersion
	for _, version := range md.Versions {
		versions = append(versions, ModuleVersionToProto(version))
	}
	return &pb.ModuleMetadata{
		Models:     models,
		Versions:   versions,
		Entrypoint: md.Entrypoint,
		FirstRun:   md.FirstRun,
	}, nil
}

type Model struct {
	API   string
	Model string
}

func ProtoToModel(model *pb.Model) *Model {
	return &Model{
		API:   model.Api,
		Model: model.Model,
	}
}

func ModelToProto(model *Model) *pb.Model {
	return &pb.Model{
		Api:   model.API,
		Model: model.Model,
	}
}

type ModuleVersion struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
}

func ProtoToModuleVersion(version *pb.ModuleVersion) *ModuleVersion {
	var files []*Uploads
	for _, file := range version.Files {
		files = append(files, ProtoToUploads(file))
	}
	var models []*Model
	for _, model := range version.Models {
		models = append(models, ProtoToModel(model))
	}
	return &ModuleVersion{
		Version:    version.Version,
		Files:      files,
		Models:     models,
		Entrypoint: version.Entrypoint,
		FirstRun:   version.FirstRun,
	}
}

func ModuleVersionToProto(version *ModuleVersion) *pb.ModuleVersion {
	var files []*pb.Uploads
	for _, file := range version.Files {
		files = append(files, UploadsToProto(file))
	}
	var models []*pb.Model
	for _, model := range version.Models {
		models = append(models, ModelToProto(model))
	}
	return &pb.ModuleVersion{
		Version:    version.Version,
		Files:      files,
		Models:     models,
		Entrypoint: version.Entrypoint,
		FirstRun:   version.FirstRun,
	}
}

type Uploads struct {
	Platform   string
	UploadedAt *timestamppb.Timestamp
}

func ProtoToUploads(uploads *pb.Uploads) *Uploads {
	return &Uploads{
		Platform:   uploads.Platform,
		UploadedAt: uploads.UploadedAt,
	}
}

func UploadsToProto(uploads *Uploads) *pb.Uploads {
	return &pb.Uploads{
		Platform:   uploads.Platform,
		UploadedAt: uploads.UploadedAt,
	}
}

type MLModelMetadata struct {
	Versions       []string
	ModelType      ModelType
	ModelFramework ModelFramework
}

func ProtoToMLModelMetadata(md *pb.MLModelMetadata) (*MLModelMetadata, error) {
	modelType, err := ProtoToModelType(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := ProtoToModelFramework(md.ModelFramework)
	if err != nil {
		return nil, err
	}
	return &MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelType,
		ModelFramework: modelFramework,
	}, nil
}

func MLModelMetadataToProto(md *MLModelMetadata) (*pb.MLModelMetadata, error) {
	modelType, err := ModelTypeToProto(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := ModelFrameworkToProto(md.ModelFramework)
	if err != nil {
		return nil, err
	}
	return &pb.MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelType,
		ModelFramework: modelFramework,
	}, nil
}

type ModelType int32

const (
	ModelTypeUnspecified               ModelType = 0
	ModelTypeSingleLabelClassification ModelType = 1
	ModelTypeMultiLabelClassification  ModelType = 2
	ModelTypeObjectDetection           ModelType = 3
)

func ProtoToModelType(modelType mlTraining.ModelType) (ModelType, error) {
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

func ModelTypeToProto(modelType ModelType) (mlTraining.ModelType, error) {
	switch modelType {
	case ModelTypeUnspecified:
		return mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED, nil
	case ModelTypeSingleLabelClassification:
		return mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION, nil
	case ModelTypeMultiLabelClassification:
		return mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION, nil
	case ModelTypeObjectDetection:
		return mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION, nil
	default:
		return 0, fmt.Errorf("unknown model type: %v", modelType)
	}
}

type ModelFramework int32

const (
	ModelFrameworkUnspecified ModelFramework = 0
	ModelFrameworkTFLite      ModelFramework = 1
	ModelFrameworkTensorFlow  ModelFramework = 2
	ModelFrameworkPyTorch     ModelFramework = 3
	ModelFrameworkONNX        ModelFramework = 4
)

func ProtoToModelFramework(framework mlTraining.ModelFramework) (ModelFramework, error) {
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

func ModelFrameworkToProto(framework ModelFramework) (mlTraining.ModelFramework, error) {
	switch framework {
	case ModelFrameworkUnspecified:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED, nil
	case ModelFrameworkTFLite:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE, nil
	case ModelFrameworkTensorFlow:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW, nil
	case ModelFrameworkPyTorch:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH, nil
	case ModelFrameworkONNX:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX, nil
	default:
		return 0, fmt.Errorf("unknown model framework: %v", framework)
	}
}

type MLTrainingMetadata struct {
	Versions       []*MLTrainingVersion
	ModelType      ModelType
	ModelFramework ModelFramework
	Draft          bool
}

func ProtoToMLTrainingMetadata(md *pb.MLTrainingMetadata) (*MLTrainingMetadata, error) {
	var versions []*MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, ProtoToMLTrainingVersion(version))
	}
	modelType, err := ProtoToModelType(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := ProtoToModelFramework(md.ModelFramework)
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

func MLTrainingMetadataToProto(md *MLTrainingMetadata) (*pb.MLTrainingMetadata, error) {
	var versions []*pb.MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, MLTrainingVersionToProto(version))
	}
	modelType, err := ModelTypeToProto(md.ModelType)
	if err != nil {
		return nil, err
	}
	modelFramework, err := ModelFrameworkToProto(md.ModelFramework)
	if err != nil {
		return nil, err
	}
	return &pb.MLTrainingMetadata{
		Versions:       versions,
		ModelType:      modelType,
		ModelFramework: modelFramework,
		Draft:          md.Draft,
	}, nil
}

type MLTrainingVersion struct {
	Version   string
	CreatedOn *timestamppb.Timestamp
}

func ProtoToMLTrainingVersion(version *pb.MLTrainingVersion) *MLTrainingVersion {
	return &MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: version.CreatedOn,
	}
}

func MLTrainingVersionToProto(version *MLTrainingVersion) *pb.MLTrainingVersion {
	return &pb.MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: version.CreatedOn,
	}
}

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

func ProtoToModule(module *pb.Module) (*Module, error) {
	visibility, err := ProtoToVisibility(module.Visibility)
	if err != nil {
		return nil, err
	}
	var versions []*VersionHistory
	for _, version := range module.Versions {
		versions = append(versions, ProtoToVersionHistory(version))
	}
	var models []*Model
	for _, model := range module.Models {
		models = append(models, ProtoToModel(model))
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

func ModuleToProto(module *Module) (*pb.Module, error) {
	visibility, err := VisibilityToProto(module.Visibility)
	if err != nil {
		return nil, err
	}
	var versions []*pb.VersionHistory
	for _, version := range module.Versions {
		versions = append(versions, VersionHistoryToProto(version))
	}
	var models []*pb.Model
	for _, model := range module.Models {
		models = append(models, ModelToProto(model))
	}
	return &pb.Module{
		ModuleId:               module.ModuleID,
		Name:                   module.Name,
		Visibility:             visibility,
		Versions:               versions,
		Url:                    module.URL,
		Description:            module.Description,
		Models:                 models,
		TotalRobotUsage:        module.TotalRobotUsage,
		TotalOrganizationUsage: module.TotalOrganizationUsage,
		OrganizationId:         module.OrganizationID,
		Entrypoint:             module.Entrypoint,
		PublicNamespace:        module.PublicNamespace,
		FirstRun:               module.FirstRun,
	}, nil
}

type VersionHistory struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
}

func ProtoToVersionHistory(history *pb.VersionHistory) *VersionHistory {
	var files []*Uploads
	for _, file := range history.Files {
		files = append(files, ProtoToUploads(file))
	}
	var models []*Model
	for _, model := range history.Models {
		models = append(models, ProtoToModel(model))
	}
	return &VersionHistory{
		Version:    history.Version,
		Files:      files,
		Models:     models,
		Entrypoint: history.Entrypoint,
		FirstRun:   history.FirstRun,
	}
}

func VersionHistoryToProto(history *VersionHistory) *pb.VersionHistory {
	var files []*pb.Uploads
	for _, file := range history.Files {
		files = append(files, UploadsToProto(file))
	}
	var models []*pb.Model
	for _, model := range history.Models {
		models = append(models, ModelToProto(model))
	}
	return &pb.VersionHistory{
		Version:    history.Version,
		Files:      files,
		Models:     models,
		Entrypoint: history.Entrypoint,
		FirstRun:   history.FirstRun,
	}
}
