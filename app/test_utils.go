package app

import (
	"time"

	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Constants used throughout testing in app.
const (
	locationID     = "location_id"
	tag            = "tag"
	robotID        = "robot_id"
	partID         = "part_id"
	robotName      = "robot_name"
	partName       = "part_name"
	host           = "host_name"
	email          = "email"
	datasetID      = "dataset_id"
	version        = "version"
	modelType      = ModelTypeObjectDetection
	itemID         = "item_id"
	modelFramework = ModelFrameworkPyTorch
	level          = "level"
	secret         = "secret"
	fragmentID     = "fragment_id"
)

// Variables used throughout testing in app.
var (
	organizationID = "organization_id"
	start          = time.Now().UTC().Round(time.Millisecond)
	pbStart        = timestamppb.New(start)
	end            = time.Now().UTC().Round(time.Millisecond)
	pbEnd          = timestamppb.New(end)
	tags           = []string{tag}
	limit          = 2
	pbLimit        = uint64(limit)
	createdOn      = time.Now().UTC().Round(time.Millisecond)
	pbCreatedOn    = timestamppb.New(createdOn)
	pbModelType    = modelTypeToProto(modelType)
	message        = "message"
	siteURL        = "url.test.com"
	lastUpdated    = time.Now().UTC().Round(time.Millisecond)
	byteData       = []byte{4, 8}
	pageToken      = "page_token"
	timestamp      = time.Now().UTC().Round(time.Millisecond)
)

// Functions used throughout testing in app.

func modelFrameworkToProto(framework ModelFramework) mltrainingpb.ModelFramework {
	switch framework {
	case ModelFrameworkUnspecified:
		return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED
	case ModelFrameworkTFLite:
		return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_TFLITE
	case ModelFrameworkTensorFlow:
		return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW
	case ModelFrameworkPyTorch:
		return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_PYTORCH
	case ModelFrameworkONNX:
		return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_ONNX
	}
	return mltrainingpb.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED
}
