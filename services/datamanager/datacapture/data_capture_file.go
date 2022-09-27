// Package datacapture contains tools for interacting with Viam datacapture files.
package datacapture

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/protoutils"
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

type File struct {
	path   string
	lock   *sync.Mutex
	file   *os.File
	writer *bufio.Writer
	size   int64
}

func NewFileFromFile(f *os.File) (*File, error) {
	if !IsDataCaptureFile(f) {
		return nil, errors.New(fmt.Sprintf("%s is not a data capture file", f.Name()))
	}
	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	ret := File{
		path:   f.Name(),
		lock:   &sync.Mutex{},
		file:   f,
		writer: bufio.NewWriter(f),
		size:   finfo.Size(),
	}
	return &ret, nil
}

func NewFile(captureDir string, md *v1.DataCaptureMetadata) (*File, error) {
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
	n, err := pbutil.WriteDelimited(f, md)
	if err != nil {
		return nil, err
	}
	return &File{
		path:   f.Name(),
		writer: bufio.NewWriter(f),
		file:   f,
		size:   int64(n),
	}, nil
}

func (f *File) ReadMetadata() (*v1.DataCaptureMetadata, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if _, err := f.file.Seek(0, 0); err != nil {
		return nil, err
	}

	r := &v1.DataCaptureMetadata{}
	if _, err := pbutil.ReadDelimited(f.file, r); err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("failed to read DataCaptureMetadata from %s", f.file.Name()))
	}

	if r.GetType() == v1.DataType_DATA_TYPE_UNSPECIFIED {
		return nil, errors.Errorf("file %s does not contain valid metadata", f.file.Name())
	}

	return r, nil
}

func (f *File) ReadNext() (*v1.SensorData, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f.file, r); err != nil {
		return nil, err
	}

	return r, nil
}

func (f *File) WriteNext(data *v1.SensorData) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	n, err := pbutil.WriteDelimited(f.writer, data)
	if err != nil {
		return err
	}
	f.size += int64(n)
	return nil
}

func (f *File) Sync() error {
	return f.writer.Flush()
}

func (f *File) Size() int64 {
	return f.size
}

func (f *File) GetPath() string {
	return f.path
}

// BuildCaptureMetadata builds a DataCaptureMetadata object and returns error if
// additionalParams fails to convert to anypb map.
func BuildCaptureMetadata(compType resource.SubtypeName, compName, compModel, method string,
	additionalParams map[string]string, tags []string,
) (*v1.DataCaptureMetadata, error) {
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(additionalParams)
	if err != nil {
		return nil, err
	}

	dataType := getDataType(string(compType), method)
	return &v1.DataCaptureMetadata{
		ComponentType:    string(compType),
		ComponentName:    compName,
		ComponentModel:   compModel,
		MethodName:       method,
		Type:             dataType,
		MethodParameters: methodParams,
		FileExtension:    GetFileExt(dataType, method, additionalParams),
		Tags:             tags,
	}, nil
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == FileExt
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// TODO DATA-246: Implement this in some more robust, programmatic way.
// TODO: support GetImage. This is why image stuff isn't working.
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
