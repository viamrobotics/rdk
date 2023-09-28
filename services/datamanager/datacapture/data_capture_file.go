// Package datacapture contains tools for interacting with Viam datacapture files.
package datacapture

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	InProgressFileExt = ".prog"
	FileExt           = ".capture"
	readImage         = "ReadImage"
	getImages         = "GetImages"
	nextPointCloud    = "NextPointCloud"
	pointCloudMap     = "PointCloudMap"
)

// File is the data structure containing data captured by collectors. It is backed by a file on disk containing
// length delimited protobuf messages, where the first message is the CaptureMetadata for the file, and ensuing
// messages contain the captured data.
type File struct {
	path     string
	lock     sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	size     int64
	metadata *v1.DataCaptureMetadata

	initialReadOffset int64
	readOffset        int64
	writeOffset       int64
}

// ReadFile creates a File struct from a passed os.File previously constructed using NewFile.
func ReadFile(f *os.File) (*File, error) {
	if !IsDataCaptureFile(f) {
		return nil, errors.Errorf("%s is not a data capture file", f.Name())
	}
	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	md := &v1.DataCaptureMetadata{}
	initOffset, err := pbutil.ReadDelimited(f, md)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("failed to read DataCaptureMetadata from %s", f.Name()))
	}

	ret := File{
		path:              f.Name(),
		file:              f,
		writer:            bufio.NewWriter(f),
		size:              finfo.Size(),
		metadata:          md,
		initialReadOffset: int64(initOffset),
		readOffset:        int64(initOffset),
		writeOffset:       int64(initOffset),
	}

	return &ret, nil
}

// NewFile creates a new File with the specified md in the specified directory.
func NewFile(dir string, md *v1.DataCaptureMetadata) (*File, error) {
	fileName := filepath.Join(dir, getFileTimestampName()) + InProgressFileExt
	//nolint:gosec
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	// Then write first metadata message to the file.
	n, err := pbutil.WriteDelimited(f, md)
	if err != nil {
		return nil, err
	}
	return &File{
		path:              f.Name(),
		writer:            bufio.NewWriter(f),
		file:              f,
		size:              int64(n),
		initialReadOffset: int64(n),
		readOffset:        int64(n),
		writeOffset:       int64(n),
	}, nil
}

// ReadMetadata reads and returns the metadata in f.
func (f *File) ReadMetadata() *v1.DataCaptureMetadata {
	return f.metadata
}

// ReadNext returns the next SensorData reading.
func (f *File) ReadNext() (*v1.SensorData, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if err := f.writer.Flush(); err != nil {
		return nil, err
	}

	if _, err := f.file.Seek(f.readOffset, 0); err != nil {
		return nil, err
	}
	r := v1.SensorData{}
	read, err := pbutil.ReadDelimited(f.file, &r)
	if err != nil {
		return nil, err
	}
	f.readOffset += int64(read)

	return &r, nil
}

// WriteNext writes the next SensorData reading.
func (f *File) WriteNext(data *v1.SensorData) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if _, err := f.file.Seek(f.writeOffset, 0); err != nil {
		return err
	}
	n, err := pbutil.WriteDelimited(f.writer, data)
	if err != nil {
		return err
	}
	f.size += int64(n)
	f.writeOffset += int64(n)
	return nil
}

// Flush flushes any buffered writes to disk.
func (f *File) Flush() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.writer.Flush()
}

// Reset resets the read pointer of f.
func (f *File) Reset() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.readOffset = f.initialReadOffset
}

// Size returns the size of the file.
func (f *File) Size() int64 {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.size
}

// GetPath returns the path of the underlying os.File.
func (f *File) GetPath() string {
	return f.path
}

// Close closes the file.
func (f *File) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.writer.Flush(); err != nil {
		return err
	}

	// Rename file to indicate that it is done being written.
	withoutExt := strings.TrimSuffix(f.file.Name(), filepath.Ext(f.file.Name()))
	newName := withoutExt + FileExt
	if err := os.Rename(f.file.Name(), newName); err != nil {
		return err
	}
	return f.file.Close()
}

// Delete deletes the file.
func (f *File) Delete() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.file.Close(); err != nil {
		return err
	}
	return os.Remove(f.GetPath())
}

// BuildCaptureMetadata builds a DataCaptureMetadata object and returns error if
// additionalParams fails to convert to anypb map.
func BuildCaptureMetadata(
	compAPI resource.API,
	compName string,
	method string,
	additionalParams map[string]string,
	tags []string,
) (*v1.DataCaptureMetadata, error) {
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(additionalParams)
	if err != nil {
		return nil, err
	}

	dataType := getDataType(method)
	return &v1.DataCaptureMetadata{
		ComponentType:    compAPI.String(),
		ComponentName:    compName,
		MethodName:       method,
		Type:             dataType,
		MethodParameters: methodParams,
		FileExtension:    GetFileExt(dataType, method, additionalParams),
		Tags:             tags,
	}, nil
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == FileExt || filepath.Ext(f.Name()) == InProgressFileExt
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// TODO DATA-246: Implement this in some more robust, programmatic way.
func getDataType(methodName string) v1.DataType {
	switch methodName {
	case nextPointCloud, readImage, pointCloudMap, getImages:
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
		return ".dat"
	case v1.DataType_DATA_TYPE_FILE:
		return defaultFileExt
	case v1.DataType_DATA_TYPE_BINARY_SENSOR:
		if methodName == nextPointCloud {
			return ".pcd"
		}
		if methodName == readImage {
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

// SensorDataFromFilePath returns all readings in the file at filePath.
func SensorDataFromFilePath(filePath string) ([]*v1.SensorData, error) {
	//nolint:gosec
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	dcFile, err := ReadFile(f)
	if err != nil {
		return nil, err
	}

	return SensorDataFromFile(dcFile)
}

// SensorDataFromFile returns all readings in f.
func SensorDataFromFile(f *File) ([]*v1.SensorData, error) {
	f.Reset()
	var ret []*v1.SensorData
	for {
		next, err := f.ReadNext()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}
		ret = append(ret, next)
	}
	return ret, nil
}
