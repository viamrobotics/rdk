package datacapture

import (
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestBuildCaptureMetadata(t *testing.T) {
	tests := []struct {
		name             string
		componentType    string
		componentName    string
		method           string
		additionalParams map[string]string
		dataType         v1.DataType
		fileExtension    string
		tags             []string
	}{
		{
			name:             "Metadata for arm positions stored in a length delimited proto file",
			componentType:    "arm",
			componentName:    "arm1",
			method:           "EndPosition",
			additionalParams: make(map[string]string),
			dataType:         v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			fileExtension:    ".dat",
			tags:             []string{"tagA", "tagB"},
		},
		{
			name:             "Metadata for a camera Next() image stored as a binary .jpeg file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]string{"mime_type": utils.MimeTypeJPEG},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".jpeg",
			tags:             []string{},
		},
		{
			name:             "Metadata for a LiDAR Next() point cloud stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]string{"mime_type": utils.MimeTypePCD},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
			tags:             []string{},
		},
		{
			name:             "Metadata for a LiDAR NextPointCloud() stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           nextPointCloud,
			additionalParams: make(map[string]string),
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
			tags:             []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualMetadata, err := BuildCaptureMetadata(
				resource.APINamespaceRDK.WithComponentType(tc.componentType),
				tc.componentName, tc.method, tc.additionalParams, tc.tags)
			test.That(t, err, test.ShouldEqual, nil)

			methodParams, err := protoutils.ConvertStringMapToAnyPBMap(tc.additionalParams)
			test.That(t, err, test.ShouldEqual, nil)

			expectedMetadata := v1.DataCaptureMetadata{
				ComponentType:    resource.APINamespaceRDK.WithComponentType(tc.componentType).String(),
				ComponentName:    tc.componentName,
				MethodName:       tc.method,
				Type:             tc.dataType,
				MethodParameters: methodParams,
				FileExtension:    tc.fileExtension,
				Tags:             tc.tags,
			}
			test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
		})
	}
}

// TestReadCorruptedFile ensures that if a file ends with invalid data (which can occur if a robot is killed uncleanly
// during a write, e.g. if the power is cut), the file is still successfully read up until that point.
func TestReadCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	md := &v1.DataCaptureMetadata{
		Type: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
	}
	f, err := NewFile(dir, md)
	test.That(t, err, test.ShouldBeNil)
	numReadings := 100
	for i := 0; i < numReadings; i++ {
		err := f.WriteNext(&v1.SensorData{
			Metadata: &v1.SensorMetadata{},
			Data:     &v1.SensorData_Struct{Struct: &structpb.Struct{}},
		})
		test.That(t, err, test.ShouldBeNil)
	}
	_, err = f.writer.Write([]byte("invalid data"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.writer.Flush(), test.ShouldBeNil)

	// Should still be able to successfully read all the successfully written data.
	sd, err := SensorDataFromFilePath(f.GetPath())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(sd), test.ShouldEqual, numReadings)
}
