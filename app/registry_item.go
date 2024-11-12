package app

import (
	"fmt"

	mlTraining "go.viam.com/api/app/mltraining/v1"
	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RegistryItem struct {
	ItemId string
	OrganizationId string
	PublicNamespace string
	Name string
	Type PackageType
	Visibility Visibility
	Url string
	Description string
	TotalRobotUsage int64
	TotalExternalRobotUsage int64
	TotalOrganizationUsage int64
	TotalExternalOrganizationUsage int64
	Metadata isRegistryItem_Metadata
	CreatedAt *timestamppb.Timestamp
	UpdatedAt *timestamppb.Timestamp
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

	var metadata isRegistryItem_Metadata
	switch pbMetadata := item.Metadata.(type) {
	case *pb.RegistryItem_ModuleMetadata:
		md, err := ProtoToModuleMetadata(pbMetadata.ModuleMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItem_ModuleMetadata{ModuleMetadata: md}
	case *pb.RegistryItem_MlModelMetadata:
		md, err := ProtoToMLModelMetadata(pbMetadata.MlModelMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItem_MlModelMetadata{MlModelMetadata: md}
	case *pb.RegistryItem_MlTrainingMetadata:
		md, err := ProtoToMLTrainingMetadata(pbMetadata.MlTrainingMetadata)
		if err != nil {
			return nil, err
		}
		metadata = &RegistryItem_MlTrainingMetadata{MlTrainingMetadata: md}
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}

	return &RegistryItem{
		ItemId: item.ItemId,
		OrganizationId: item.OrganizationId,
		PublicNamespace: item.PublicNamespace,
		Name: item.Name,
		Type: packageType,
		Visibility: visibility,
		Url: item.Url,
		Description: item.Description,
		TotalRobotUsage: item.TotalRobotUsage,
		TotalExternalRobotUsage: item.TotalExternalRobotUsage,
		TotalOrganizationUsage: item.TotalOrganizationUsage,
		TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
		Metadata: metadata,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
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
	case *RegistryItem_ModuleMetadata:
		protoMetadata, err := ModuleMetadataToProto(md.ModuleMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId: item.ItemId,
			OrganizationId: item.OrganizationId,
			PublicNamespace: item.PublicNamespace,
			Name: item.Name,
			Type: packageType,
			Visibility: visibility,
			Url: item.Url,
			Description: item.Description,
			TotalRobotUsage: item.TotalRobotUsage,
			TotalExternalRobotUsage: item.TotalExternalRobotUsage,
			TotalOrganizationUsage: item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata: &pb.RegistryItem_ModuleMetadata{ModuleMetadata: protoMetadata},
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}, nil
	case *RegistryItem_MlModelMetadata:
		protoMetadata, err := MLModelMetadataToProto(md.MlModelMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId: item.ItemId,
			OrganizationId: item.OrganizationId,
			PublicNamespace: item.PublicNamespace,
			Name: item.Name,
			Type: packageType,
			Visibility: visibility,
			Url: item.Url,
			Description: item.Description,
			TotalRobotUsage: item.TotalRobotUsage,
			TotalExternalRobotUsage: item.TotalExternalRobotUsage,
			TotalOrganizationUsage: item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata: &pb.RegistryItem_MlModelMetadata{MlModelMetadata: protoMetadata},
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}, nil
	case *RegistryItem_MlTrainingMetadata:
		protoMetadata, err := MLTrainingMetadataToProto(md.MlTrainingMetadata)
		if err != nil {
			return nil, err
		}
		return &pb.RegistryItem{
			ItemId: item.ItemId,
			OrganizationId: item.OrganizationId,
			PublicNamespace: item.PublicNamespace,
			Name: item.Name,
			Type: packageType,
			Visibility: visibility,
			Url: item.Url,
			Description: item.Description,
			TotalRobotUsage: item.TotalRobotUsage,
			TotalExternalRobotUsage: item.TotalExternalRobotUsage,
			TotalOrganizationUsage: item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata: &pb.RegistryItem_MlTrainingMetadata{MlTrainingMetadata: protoMetadata},
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}, nil
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}
}

type RegistryItemStatus int32
const (
	RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED    RegistryItemStatus = 0
	RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED      RegistryItemStatus = 1
	RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT RegistryItemStatus = 2
)

func ProtoToRegistryItemStatus(status pb.RegistryItemStatus) (RegistryItemStatus, error) {
	switch status{
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED:
		return RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED, nil
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED:
		return RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED, nil
	case pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT:
		return RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT, nil
	default:
		return 0, fmt.Errorf("unknown registry item status: %v", status)
	}
}

func RegistryItemStatusToProto(status RegistryItemStatus) (pb.RegistryItemStatus, error) {
	switch status{
	case RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED, nil
	case RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED, nil
	case RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT, nil
	default:
		return 0, fmt.Errorf("unknown registry item status: %v", status)
	}
}

type PackageType int32
const (
	PackageType_PACKAGE_TYPE_UNSPECIFIED PackageType = 0
	PackageType_PACKAGE_TYPE_ARCHIVE     PackageType = 1
	PackageType_PACKAGE_TYPE_ML_MODEL    PackageType = 2
	PackageType_PACKAGE_TYPE_MODULE      PackageType = 3
	PackageType_PACKAGE_TYPE_SLAM_MAP    PackageType = 4
	PackageType_PACKAGE_TYPE_ML_TRAINING PackageType = 5
)

func ProtoToPackageType(packageType packages.PackageType) (PackageType, error) {
	switch packageType{
	case packages.PackageType_PACKAGE_TYPE_UNSPECIFIED:
		return PackageType_PACKAGE_TYPE_UNSPECIFIED, nil
	case packages.PackageType_PACKAGE_TYPE_ARCHIVE:
		return PackageType_PACKAGE_TYPE_ARCHIVE, nil
	case packages.PackageType_PACKAGE_TYPE_ML_MODEL:
		return PackageType_PACKAGE_TYPE_ML_MODEL, nil
	case packages.PackageType_PACKAGE_TYPE_MODULE:
		return PackageType_PACKAGE_TYPE_MODULE, nil
	case packages.PackageType_PACKAGE_TYPE_SLAM_MAP:
		return PackageType_PACKAGE_TYPE_SLAM_MAP, nil
	case packages.PackageType_PACKAGE_TYPE_ML_TRAINING:
		return PackageType_PACKAGE_TYPE_ML_TRAINING, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", packageType)
	}
}

func PackageTypeToProto(packageType PackageType) (packages.PackageType, error) {
	switch packageType{
	case PackageType_PACKAGE_TYPE_UNSPECIFIED:
		return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED, nil
	case PackageType_PACKAGE_TYPE_ARCHIVE:
		return packages.PackageType_PACKAGE_TYPE_ARCHIVE, nil
	case PackageType_PACKAGE_TYPE_ML_MODEL:
		return packages.PackageType_PACKAGE_TYPE_ML_MODEL, nil
	case PackageType_PACKAGE_TYPE_MODULE:
		return packages.PackageType_PACKAGE_TYPE_MODULE, nil
	case PackageType_PACKAGE_TYPE_SLAM_MAP:
		return packages.PackageType_PACKAGE_TYPE_SLAM_MAP, nil
	case PackageType_PACKAGE_TYPE_ML_TRAINING:
		return packages.PackageType_PACKAGE_TYPE_ML_TRAINING, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", packageType)
	}
}


type Visibility int32
const (
	Visibility_VISIBILITY_UNSPECIFIED Visibility = 0
	Visibility_VISIBILITY_PRIVATE Visibility = 1
	Visibility_VISIBILITY_PUBLIC Visibility = 2
	Visibility_VISIBILITY_PUBLIC_UNLISTED Visibility = 3
)

func ProtoToVisibility(visibility pb.Visibility) (Visibility, error) {
	switch visibility{
	case pb.Visibility_VISIBILITY_UNSPECIFIED:
		return Visibility_VISIBILITY_UNSPECIFIED, nil
	case pb.Visibility_VISIBILITY_PRIVATE:
		return Visibility_VISIBILITY_PRIVATE, nil
	case pb.Visibility_VISIBILITY_PUBLIC:
		return Visibility_VISIBILITY_PUBLIC, nil
	case pb.Visibility_VISIBILITY_PUBLIC_UNLISTED:
		return Visibility_VISIBILITY_PUBLIC_UNLISTED, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", visibility)
	}
}

func VisibilityToProto(visibility Visibility) (pb.Visibility, error) {
	switch visibility{
	case Visibility_VISIBILITY_UNSPECIFIED:
		return pb.Visibility_VISIBILITY_UNSPECIFIED, nil
	case Visibility_VISIBILITY_PRIVATE:
		return pb.Visibility_VISIBILITY_PRIVATE, nil
	case Visibility_VISIBILITY_PUBLIC:
		return pb.Visibility_VISIBILITY_PUBLIC, nil
	case Visibility_VISIBILITY_PUBLIC_UNLISTED:
		return pb.Visibility_VISIBILITY_PUBLIC_UNLISTED, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", visibility)
	}
}

type isRegistryItem_Metadata interface {
	isRegistryItem_Metadata()
}

type RegistryItem_ModuleMetadata struct {
	ModuleMetadata *ModuleMetadata
}

type RegistryItem_MlModelMetadata struct {
	MlModelMetadata *MLModelMetadata
}

type RegistryItem_MlTrainingMetadata struct {
	MlTrainingMetadata *MLTrainingMetadata
}

func (*RegistryItem_ModuleMetadata) isRegistryItem_Metadata() {}

func (*RegistryItem_MlModelMetadata) isRegistryItem_Metadata() {}

func (*RegistryItem_MlTrainingMetadata) isRegistryItem_Metadata() {}

type ModuleMetadata struct {
	Models []*Model
	Versions []*ModuleVersion
	Entrypoint string
	FirstRun *string
}

func ProtoToModuleMetadata(md *pb.ModuleMetadata) (*ModuleMetadata, error) {
	var models []*Model
	for _, version := range(md.Models) {
		models = append(models, ProtoToModel(version))
	}
	var versions []*ModuleVersion
	for _, version := range(md.Versions) {
		versions = append(versions, ProtoToModuleVersion(version))
	}
	return &ModuleMetadata{
		Models: models,
		Versions: versions,
		Entrypoint: md.Entrypoint,
		FirstRun: md.FirstRun,
	}, nil
}

func ModuleMetadataToProto(md *ModuleMetadata) (*pb.ModuleMetadata, error) {
	var models []*pb.Model
	for _, version := range(md.Models) {
		models = append(models, ModelToProto(version))
	}
	var versions []*pb.ModuleVersion
	for _, version := range(md.Versions) {
		versions = append(versions, ModuleVersionToProto(version))
	}
	return &pb.ModuleMetadata{
		Models: models,
		Versions: versions,
		Entrypoint: md.Entrypoint,
		FirstRun: md.FirstRun,
	}, nil
}

type Model struct {
	Api string
	Model string
}

func ProtoToModel(model *pb.Model) *Model {
	return &Model{
		Api: model.Api,
		Model: model.Model,
	}
}

func ModelToProto(model *Model) *pb.Model {
	return &pb.Model{
		Api: model.Api,
		Model: model.Model,
	}
}

type ModuleVersion struct {
	Version string
	Files []*Uploads
	Models []*Model
	Entrypoint string
	FirstRun *string
}

func ProtoToModuleVersion(version *pb.ModuleVersion) (*ModuleVersion) {
	var files []*Uploads
	for _, file := range(version.Files) {
		files = append(files, ProtoToUploads(file))
	}
	var models []*Model
	for _, model := range(version.Models) {
		models = append(models, ProtoToModel(model))
	}
	return &ModuleVersion{
		Version: version.Version,
		Files: files,
		Models: models,
		Entrypoint: version.Entrypoint,
		FirstRun: version.FirstRun,
	}
}

func ModuleVersionToProto(version *ModuleVersion) (*pb.ModuleVersion) {
	var files []*pb.Uploads
	for _, file := range(version.Files) {
		files = append(files, UploadsToProto(file))
	}
	var models []*pb.Model
	for _, model := range(version.Models) {
		models = append(models, ModelToProto(model))
	}
	return &pb.ModuleVersion{
		Version: version.Version,
		Files: files,
		Models: models,
		Entrypoint: version.Entrypoint,
		FirstRun: version.FirstRun,
	}
}

type Uploads struct {
	Platform string
	UploadedAt *timestamppb.Timestamp
}

func ProtoToUploads(uploads *pb.Uploads) *Uploads {
	return &Uploads{
		Platform: uploads.Platform,
		UploadedAt: uploads.UploadedAt,
	}
}

func UploadsToProto(uploads *Uploads) *pb.Uploads {
	return &pb.Uploads{
		Platform: uploads.Platform,
		UploadedAt: uploads.UploadedAt,
	}
}
 
type MLModelMetadata struct {
	Versions []string
	ModelType ModelType
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
		Versions: md.Versions,
		ModelType: modelType,
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
		Versions: md.Versions,
		ModelType: modelType,
		ModelFramework: modelFramework,
	}, nil
}

type ModelType int32
const (
	ModelType_MODEL_TYPE_UNSPECIFIED                 ModelType = 0
	ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION ModelType = 1
	ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION  ModelType = 2
	ModelType_MODEL_TYPE_OBJECT_DETECTION            ModelType = 3
)

func ProtoToModelType(modelType mlTraining.ModelType) (ModelType, error) {
	switch modelType{
	case mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED:
		return ModelType_MODEL_TYPE_UNSPECIFIED, nil
	case mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION:
		return ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION, nil
	case mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION:
		return ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION, nil
	case mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION:
		return ModelType_MODEL_TYPE_OBJECT_DETECTION, nil
	default:
		return 0, fmt.Errorf("unknown model type: %v", modelType)
	}
}

func ModelTypeToProto(modelType ModelType) (mlTraining.ModelType, error) {
	switch modelType{
	case ModelType_MODEL_TYPE_UNSPECIFIED:
		return mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED, nil
	case ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION:
		return mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION, nil
	case ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION:
		return mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION, nil
	case ModelType_MODEL_TYPE_OBJECT_DETECTION:
		return mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION, nil
	default:
		return 0, fmt.Errorf("unknown model type: %v", modelType)
	}
}

type ModelFramework int32
const (
	ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED ModelFramework = 0
	ModelFramework_MODEL_FRAMEWORK_TFLITE      ModelFramework = 1
	ModelFramework_MODEL_FRAMEWORK_TENSORFLOW  ModelFramework = 2
	ModelFramework_MODEL_FRAMEWORK_PYTORCH     ModelFramework = 3
	ModelFramework_MODEL_FRAMEWORK_ONNX        ModelFramework = 4
)

func ProtoToModelFramework(framework mlTraining.ModelFramework) (ModelFramework, error) {
	switch framework{
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED:
		return ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE:
		return ModelFramework_MODEL_FRAMEWORK_TFLITE, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW:
		return ModelFramework_MODEL_FRAMEWORK_TENSORFLOW, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH:
		return ModelFramework_MODEL_FRAMEWORK_PYTORCH, nil
	case mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX:
		return ModelFramework_MODEL_FRAMEWORK_ONNX, nil
	default:
		return 0, fmt.Errorf("unknown model framework: %v", framework)
	}
}

func ModelFrameworkToProto(framework ModelFramework) (mlTraining.ModelFramework, error) {
	switch framework{
	case ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED, nil
	case ModelFramework_MODEL_FRAMEWORK_TFLITE:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE, nil
	case ModelFramework_MODEL_FRAMEWORK_TENSORFLOW:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW, nil
	case ModelFramework_MODEL_FRAMEWORK_PYTORCH:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH, nil
	case ModelFramework_MODEL_FRAMEWORK_ONNX:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX, nil
	default:
		return 0, fmt.Errorf("unknown model framework: %v", framework)
	}
}

type MLTrainingMetadata struct {
	Versions []*MLTrainingVersion
	ModelType ModelType
	ModelFramework ModelFramework
	Draft bool
}

func ProtoToMLTrainingMetadata(md *pb.MLTrainingMetadata) (*MLTrainingMetadata, error) {
	var versions []*MLTrainingVersion
	for _, version := range(md.Versions) {
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
		Versions: versions,
		ModelType: modelType,
		ModelFramework: modelFramework,
		Draft: md.Draft,
	}, nil
}

func MLTrainingMetadataToProto(md *MLTrainingMetadata) (*pb.MLTrainingMetadata, error) {
	var versions []*pb.MLTrainingVersion
	for _, version := range(md.Versions) {
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
		Versions: versions,
		ModelType: modelType,
		ModelFramework: modelFramework,
		Draft: md.Draft,
	}, nil
}

type MLTrainingVersion struct {
	Version string
	CreatedOn *timestamppb.Timestamp
}

func ProtoToMLTrainingVersion(version *pb.MLTrainingVersion) *MLTrainingVersion {
	return &MLTrainingVersion{
		Version: version.Version,
		CreatedOn: version.CreatedOn,
	}
}

func MLTrainingVersionToProto(version *MLTrainingVersion) *pb.MLTrainingVersion {
	return &pb.MLTrainingVersion{
		Version: version.Version,
		CreatedOn: version.CreatedOn,
	}
}

type Module struct {
	ModuleId string
	Name string
	Visibility Visibility
	Versions []*VersionHistory
	Url string
	Description string
	Models []*Model
	TotalRobotUsage int64
	TotalOrganizationUsage int64
	OrganizationId string
	Entrypoint string
	PublicNamespace string
	FirstRun *string
}

func ProtoToModule(module *pb.Module) (*Module, error) {
	visibility, err := ProtoToVisibility(module.Visibility)
	if err != nil {
		return nil, err
	}
	var versions []*VersionHistory
	for _, version := range(module.Versions){
		versions = append(versions, ProtoToVersionHistory(version))
	}
	var models []*Model
	for _, model := range(module.Models){
		models = append(models, ProtoToModel(model))
	}
	return &Module{
		ModuleId: module.ModuleId,
		Name: module.Name,
		Visibility: visibility,
		Versions: versions,
		Url: module.Url,
		Description: module.Description,
		Models: models,
		TotalRobotUsage: module.TotalRobotUsage,
		TotalOrganizationUsage: module.TotalOrganizationUsage,
		OrganizationId: module.OrganizationId,
		Entrypoint: module.Entrypoint,
		PublicNamespace: module.PublicNamespace,
		FirstRun: module.FirstRun,
	}, nil
}

func ModuleToProto(module *Module) (*pb.Module, error) {
	visibility, err := VisibilityToProto(module.Visibility)
	if err != nil {
		return nil, err
	}
	var versions []*pb.VersionHistory
	for _, version := range(module.Versions){
		versions = append(versions, VersionHistoryToProto(version))
	}
	var models []*pb.Model
	for _, model := range(module.Models){
		models = append(models, ModelToProto(model))
	}
	return &pb.Module{
		ModuleId: module.ModuleId,
		Name: module.Name,
		Visibility: visibility,
		Versions: versions,
		Url: module.Url,
		Description: module.Description,
		Models: models,
		TotalRobotUsage: module.TotalRobotUsage,
		TotalOrganizationUsage: module.TotalOrganizationUsage,
		OrganizationId: module.OrganizationId,
		Entrypoint: module.Entrypoint,
		PublicNamespace: module.PublicNamespace,
		FirstRun: module.FirstRun,
	}, nil
}

type VersionHistory struct {
	Version string
	Files []*Uploads
	Models []*Model
	Entrypoint string
	FirstRun *string
}

func ProtoToVersionHistory(history *pb.VersionHistory) *VersionHistory {
	var files []*Uploads
	for _, file := range(history.Files){
		files = append(files, ProtoToUploads(file))
	}
	var models []*Model
	for _, model := range(history.Models){
		models = append(models, ProtoToModel(model))
	}
	return &VersionHistory{
		Version: history.Version,
		Files: files,
		Models: models,
		Entrypoint: history.Entrypoint,
		FirstRun: history.FirstRun,
	}
}

func VersionHistoryToProto(history *VersionHistory) *pb.VersionHistory {
	var files []*pb.Uploads
	for _, file := range(history.Files){
		files = append(files, UploadsToProto(file))
	}
	var models []*pb.Model
	for _, model := range(history.Models){
		models = append(models, ModelToProto(model))
	}
	return &pb.VersionHistory{
		Version: history.Version,
		Files: files,
		Models: models,
		Entrypoint: history.Entrypoint,
		FirstRun: history.FirstRun,
	}
}
