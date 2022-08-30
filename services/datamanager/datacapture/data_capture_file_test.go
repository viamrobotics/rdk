package datacapture

import (
	"testing"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestBuildCaptureMetadata(t *testing.T) {
	tests := []struct {
		name             string
		componentType    resource.SubtypeName
		componentName    string
		componentModel   string
		method           string
		additionalParams map[string]string
		dataType         v1.DataType
		fileExtension    string
	}{
		{
			name:             "Metadata for arm positions stored in a tabular .csv file",
			componentType:    "arm",
			componentName:    "arm1",
			componentModel:   "eva",
			method:           "GetEndPosition",
			additionalParams: make(map[string]string),
			dataType:         v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			fileExtension:    ".csv",
		},
		{
			name:             "Metadata for a camera Next() image stored as a binary .jpeg file",
			componentType:    "camera",
			componentName:    "cam1",
			componentModel:   "webcam",
			method:           "Next",
			additionalParams: map[string]string{"mime_type": utils.MimeTypeJPEG},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".jpeg",
		},
		{
			name:             "Metadata for a LiDAR Next() point cloud stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			componentModel:   "velodyne",
			method:           "Next",
			additionalParams: map[string]string{"mime_type": utils.MimeTypePCD},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
		},
		{
			name:             "Metadata for a LiDAR NextPointCloud() stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			componentModel:   "velodyne",
			method:           "NextPointCloud",
			additionalParams: make(map[string]string),
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
		},
	}

	for _, tc := range tests {
		t.Log(tc.name)
		actualMetadata := BuildCaptureMetadata(
			tc.componentType, tc.componentName, tc.componentModel, tc.method, tc.additionalParams)
		expectedMetadata := v1.DataCaptureMetadata{
			ComponentType:    string(tc.componentType),
			ComponentName:    tc.componentName,
			ComponentModel:   tc.componentModel,
			MethodName:       tc.method,
			Type:             tc.dataType,
			MethodParameters: tc.additionalParams,
			FileExtension:    tc.fileExtension,
		}
		test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
	}
}
