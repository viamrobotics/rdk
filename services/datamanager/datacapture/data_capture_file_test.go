package datacapture

import (
	"testing"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func TestTabularBuildCaptureMetadata(t *testing.T) {
	compType := resource.SubtypeName("arm")
	compName := "arm1"
	compModel := "eva"
	method := "GetEndPosition"
	additionalParams := make(map[string]string)
	actualMetadata := BuildCaptureMetadata(
		compType, compName, compModel, method, additionalParams)
	expectedMetadata := v1.DataCaptureMetadata{
		ComponentType:    string(compType),
		ComponentName:    compName,
		ComponentModel:   compModel,
		MethodName:       method,
		Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		MethodParameters: additionalParams,
		FileExtension:    ".csv",
	}
	test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
}

func TestBinaryJpegBuildCaptureMetadata(t *testing.T) {
	compType := resource.SubtypeName("camera")
	compName := "cam1"
	compModel := "webcam"
	method := "Next"
	additionalParams := map[string]string{"mime_type": utils.MimeTypeJPEG}
	actualMetadata := BuildCaptureMetadata(
		compType, compName, compModel, method, additionalParams)
	expectedMetadata := v1.DataCaptureMetadata{
		ComponentType:    string(compType),
		ComponentName:    compName,
		ComponentModel:   compModel,
		MethodName:       method,
		Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
		MethodParameters: additionalParams,
		FileExtension:    ".jpeg",
	}
	test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
}

func TestBinaryPcdBuildCaptureMetadata(t *testing.T) {
	compType := resource.SubtypeName("camera")
	compName := "cam1"
	compModel := "velodyne"
	method := "NextPointCloud"
	additionalParams := make(map[string]string)
	actualMetadata := BuildCaptureMetadata(
		compType, compName, compModel, method, additionalParams)
	expectedMetadata := v1.DataCaptureMetadata{
		ComponentType:    string(compType),
		ComponentName:    compName,
		ComponentModel:   compModel,
		MethodName:       method,
		Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
		MethodParameters: additionalParams,
		FileExtension:    ".pcd",
	}
	test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
}
