// Package datacapture contains tools for interacting with Viam datacapture files.
package datacapture

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// TODO Data-343: Reorganize this into a more standard interface/package, and add tests.

// FileExt defines the file extension for Viam data capture files.
const (
	FileExt        = ".capture"
	next           = "Next"
	nextPointCloud = "NextPointCloud"
)

// CreateDataCaptureFile creates a timestamped file within the given capture directory.
func CreateDataCaptureFile(captureDir string, md *v1.DataCaptureMetadata) (*os.File, error) {
	// First create directories and the file in it.
	fileDir := filepath.Join(captureDir, md.GetComponentType(), md.GetComponentName(), md.GetMethodName())
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName()) + FileExt
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

// BuildCaptureMetadata builds a DataCaptureMetadata object.
func BuildCaptureMetadata(compType resource.SubtypeName, compName, compModel, method string,
	additionalParams map[string]string,
) *v1.DataCaptureMetadata {
	dataType := getDataType(string(compType), method)
	return &v1.DataCaptureMetadata{
		ComponentType:    string(compType),
		ComponentName:    compName,
		ComponentModel:   compModel,
		MethodName:       method,
		Type:             dataType,
		MethodParameters: additionalParams,
		FileExtension:    GetFileExt(dataType, method, additionalParams),
	}
}

// ReadDataCaptureMetadata reads the SyncMetadata message from the beginning of the capture file.
func ReadDataCaptureMetadata(f *os.File) (*v1.DataCaptureMetadata, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	r := &v1.DataCaptureMetadata{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("failed to read SyncMetadata from %s", f.Name()))
	}

	if r.GetType() == v1.DataType_DATA_TYPE_UNSPECIFIED {
		return nil, errors.Errorf("file %s does not contain valid metadata", f.Name())
	}

	return r, nil
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == FileExt
}

// ReadNextSensorData reads sensorData sequentially from a data capture file. It assumes the file offset is already
// pointing at the beginning of series of SensorData in the file. This is accomplished by first calling
// ReadDataCaptureMetadata.
func ReadNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}

	return r, nil
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// TODO DATA-246: Implement this in some more robust, programmatic way.
// TODO: support GetFrame. This is why image stuff isn't working.
func getDataType(_, methodName string) v1.DataType {
	switch methodName {
	case nextPointCloud, next:
		return v1.DataType_DATA_TYPE_BINARY_SENSOR
	default:
		return v1.DataType_DATA_TYPE_TABULAR_SENSOR
	}
}

// GetFileExt gets the file extension for a capture file.
func GetFileExt(dataType v1.DataType, methodName string, parameters map[string]string) string {
	defaultFileExt := ""
	switch dataType {
	case v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		return ".csv"
	case v1.DataType_DATA_TYPE_FILE:
		return defaultFileExt
	case v1.DataType_DATA_TYPE_BINARY_SENSOR:
		if methodName == nextPointCloud {
			return ".pcd"
		}
		if methodName == next {
			// TODO: Add explicit file extensions for all mime types.
			switch parameters["mime_type"] {
			case utils.MimeTypeJPEG:
				return ".jpeg"
			case utils.MimeTypePNG:
				return ".png"
			case utils.MimeTypePCD:
				return ".pcd"
			default:
				return defaultFileExt
			}
		}
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return defaultFileExt
	default:
		return defaultFileExt
	}
	return defaultFileExt
}
