package app

import (
	"time"

	mltrainingpb "go.viam.com/api/app/mltraining/v1"
)

// Constants used throughout app.
const (
	UploadChunkSize = 64 * 1024 // UploadChunkSize is 64 KB
	locationID      = "location_id"
	tag             = "tag"
	robotID         = "robot_id"
	partID          = "part_id"
	robotName       = "robot_name"
	partName        = "part_name"
	host            = "host_name"
	email           = "email"
)

// Variables used throughout app.
var (
	organizationID = "organization_id"
	start          = time.Now().UTC().Round(time.Millisecond)
	end            = time.Now().UTC().Round(time.Millisecond)
	tags           = []string{tag}
	limit          = 2
	pbLimit        = uint64(limit)
)

// Types used throughout app.

// ModelType specifies the type of model used for classification or detection.
type ModelType int

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

func modelTypeFromProto(modelType mltrainingpb.ModelType) ModelType {
	switch modelType {
	case mltrainingpb.ModelType_MODEL_TYPE_UNSPECIFIED:
		return ModelTypeUnspecified
	case mltrainingpb.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION:
		return ModelTypeSingleLabelClassification
	case mltrainingpb.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION:
		return ModelTypeMultiLabelClassification
	case mltrainingpb.ModelType_MODEL_TYPE_OBJECT_DETECTION:
		return ModelTypeObjectDetection
	}
	return ModelTypeUnspecified
}

func modelTypeToProto(modelType ModelType) mltrainingpb.ModelType {
	switch modelType {
	case ModelTypeUnspecified:
		return mltrainingpb.ModelType_MODEL_TYPE_UNSPECIFIED
	case ModelTypeSingleLabelClassification:
		return mltrainingpb.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION
	case ModelTypeMultiLabelClassification:
		return mltrainingpb.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION
	case ModelTypeObjectDetection:
		return mltrainingpb.ModelType_MODEL_TYPE_OBJECT_DETECTION
	}
	return mltrainingpb.ModelType_MODEL_TYPE_UNSPECIFIED
}

// ModelFramework is the framework type of a model.
type ModelFramework int

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

func modelFrameworkFromProto(framework mltrainingpb.ModelFramework) ModelFramework {
	switch framework {
	case mltrainingpb.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED:
		return ModelFrameworkUnspecified
	case mltrainingpb.ModelFramework_MODEL_FRAMEWORK_TFLITE:
		return ModelFrameworkTFLite
	case mltrainingpb.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW:
		return ModelFrameworkTensorFlow
	case mltrainingpb.ModelFramework_MODEL_FRAMEWORK_PYTORCH:
		return ModelFrameworkPyTorch
	case mltrainingpb.ModelFramework_MODEL_FRAMEWORK_ONNX:
		return ModelFrameworkONNX
	}
	return ModelFrameworkUnspecified
}
