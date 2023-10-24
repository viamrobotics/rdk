package datacapture

import (
	"os"
	"path/filepath"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
)

type structReading struct {
	Field1 bool
}

func (r structReading) toProto() *structpb.Struct {
	msg, err := protoutils.StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

var (
	structSensorData = &v1.SensorData{
		Metadata: &v1.SensorMetadata{},
		Data:     &v1.SensorData_Struct{Struct: structReading{}.toProto()},
	}
	binarySensorData = &v1.SensorData{
		Metadata: &v1.SensorMetadata{},
		Data: &v1.SensorData_Binary{
			Binary: []byte("this sure is binary data, yup it is"),
		},
	}
)

// TODO: rewrite tests.
func TestCaptureQueue(t *testing.T) {
	MaxFileSize = 50
	tests := []struct {
		name               string
		dataType           v1.DataType
		pushCount          int
		expCompleteFiles   int
		expInProgressFiles int
	}{
		{
			name:               "Files that are still being written to should have the InProgressFileExt extension.",
			dataType:           v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			pushCount:          1,
			expCompleteFiles:   0,
			expInProgressFiles: 1,
		},
		{
			name:               "Pushing N binary data should write N files.",
			dataType:           v1.DataType_DATA_TYPE_BINARY_SENSOR,
			pushCount:          2,
			expCompleteFiles:   2,
			expInProgressFiles: 0,
		},
		{
			name:     "Pushing > MaxFileSize + 1 worth of struct data should write two files.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// MaxFileSize / size(structSensorData) = ceil(50 / 19) = 3 readings per file => 2 files, one in progress
			pushCount:          4,
			expCompleteFiles:   1,
			expInProgressFiles: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			md := &v1.DataCaptureMetadata{Type: tc.dataType}
			sut := NewBuffer(tmpDir, md)
			var pushValue *v1.SensorData
			if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
				pushValue = binarySensorData
			} else {
				pushValue = structSensorData
			}

			for i := 0; i < tc.pushCount; i++ {
				err := sut.Write(pushValue)
				test.That(t, err, test.ShouldBeNil)
			}

			dcFiles, inProgressFiles := getCaptureFiles(tmpDir)
			test.That(t, len(dcFiles), test.ShouldEqual, tc.expCompleteFiles)
			test.That(t, len(inProgressFiles), test.ShouldEqual, tc.expInProgressFiles)

			// Test that sync is respected: after closing, all files should no longer be in progress.
			err := sut.Flush()
			test.That(t, err, test.ShouldBeNil)
			completeFiles, remainingProgFiles := getCaptureFiles(tmpDir)
			test.That(t, len(remainingProgFiles), test.ShouldEqual, 0)

			// Validate correct values were written.
			var actCaptures []*v1.SensorData
			for i := 0; i < len(completeFiles); i++ {
				c, err := SensorDataFromFilePath(completeFiles[i])
				test.That(t, err, test.ShouldBeNil)
				actCaptures = append(actCaptures, c...)
			}
			test.That(t, len(actCaptures), test.ShouldEqual, tc.pushCount)
			for _, capture := range actCaptures {
				if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
					test.That(t, capture.GetBinary(), test.ShouldNotBeNil)
					test.That(t, capture.GetBinary(), test.ShouldResemble, binarySensorData.GetBinary())
				}
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					test.That(t, capture.GetStruct(), test.ShouldNotBeNil)
					test.That(t, capture.GetStruct(), test.ShouldResemble, structSensorData.GetStruct())
				}
			}
		})
	}
}

//nolint
func getCaptureFiles(dir string) (dcFiles, progFiles []string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == FileExt {
			dcFiles = append(dcFiles, path)
		}
		if filepath.Ext(path) == InProgressFileExt {
			progFiles = append(progFiles, path)
		}
		return nil
	})
	return dcFiles, progFiles
}
