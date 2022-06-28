package datamanager

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

const dataCaptureFileExt = ".sd"

// Create a timestamped file within the given capture directory.
func createDataCaptureFile(captureDir string, md *v1.SyncMetadata) (*os.File, error) {
	// First create directories and the file in it.
	fileDir := filepath.Join(captureDir, md.GetComponentType(), md.GetComponentName(), md.GetMethodName())
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName()) + dataCaptureFileExt
	//nolint:gosec
	f, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}

	// Then write first metadata message to the file.
	if _, err := pbutil.WriteDelimited(f, md); err != nil {
		return nil, err
	}
	return f, nil
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// TODO: Implement this in some more robust way. Probably by making the DataType a field of the collector.
//nolint:unparam
func getDataType(componentType string, methodName string) v1.DataType {
	if methodName == "NextPointCloud" {
		return v1.DataType_DATA_TYPE_BINARY_SENSOR
	}
	return v1.DataType_DATA_TYPE_TABULAR_SENSOR
}

func getSyncMetadata(attributes dataCaptureConfig) *v1.SyncMetadata {
	return &v1.SyncMetadata{
		ComponentType:    string(attributes.Type),
		ComponentName:    attributes.Name,
		MethodName:       attributes.Method,
		Type:             getDataType(string(attributes.Type), attributes.Method),
		MethodParameters: attributes.AdditionalParams,
	}
}

// readSyncMetadata reads the SyncMetadata message from the beginning of the capture file.
func readSyncMetadata(f *os.File) (*v1.SyncMetadata, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	r := &v1.SyncMetadata{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("failed to read SyncMetadata from %s", f.Name()))
	}

	return r, nil
}

// readNextSensorData reads sensorData sequentially from a data capture file. It assumes the file offset is already
// pointing at the beginning of series of SensorData in the file. This is accomplished by first calling
// readSyncMetadata.
func readNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}

	// Ensure we construct and return a SensorData value for tabular data when the tabular data's fields and
	// corresponding entries are not nil. Otherwise, return io.EOF error and nil.
	if r.GetBinary() == nil {
		if r.GetStruct() == nil {
			return r, emptyReadingErr(filepath.Base(f.Name()))
		}
		return r, nil
	}
	return r, nil
}

func isDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == dataCaptureFileExt
}
