package data

import (
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

	"go.viam.com/rdk/utils"
)

// TODO Data-343: Reorganize this into a more standard interface/package, and add tests.

const (
	// InProgressCaptureFileExt defines the file extension for Viam data capture files
	// which are currently being written to.
	InProgressCaptureFileExt = ".prog"
	// CompletedCaptureFileExt defines the file extension for Viam data capture files
	// which are no longer being written to.
	CompletedCaptureFileExt = ".capture"
	readImage               = "ReadImage"
	// GetImages is used for getting simultaneous images from different imagers.
	GetImages      = "GetImages"
	nextPointCloud = "NextPointCloud"
	pointCloudMap  = "PointCloudMap"
	// Non-exhaustive list of characters to strip from file paths, since not allowed
	// on certain file systems.
	filePathReservedChars = ":"
)

// CaptureFile is the data structure containing data captured by collectors. It is backed by a file on disk containing
// length delimited protobuf messages, where the first message is the CaptureMetadata for the file, and ensuing
// messages contain the captured data.
type CaptureFile struct {
	Metadata          *v1.DataCaptureMetadata
	path              string
	size              int64
	initialReadOffset int64

	lock       sync.Mutex
	file       *os.File
	readOffset int64
}

// NewCaptureFile creates a File struct from a passed os.File previously constructed using NewFile.
func NewCaptureFile(f *os.File) (*CaptureFile, error) {
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
		return nil, errors.Wrapf(err, fmt.Sprintf("failed to read DataCaptureMetadata from %s", f.Name())) //nolint:govet
	}

	ret := CaptureFile{
		path:              f.Name(),
		file:              f,
		size:              finfo.Size(),
		Metadata:          md,
		initialReadOffset: int64(initOffset),
		readOffset:        int64(initOffset),
	}

	return &ret, nil
}

// ReadMetadata reads and returns the metadata in f.
func (f *CaptureFile) ReadMetadata() *v1.DataCaptureMetadata {
	return f.Metadata
}

// ReadNext returns the next SensorData reading.
func (f *CaptureFile) ReadNext() (*v1.SensorData, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if _, err := f.file.Seek(f.readOffset, io.SeekStart); err != nil {
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

// Reset resets the read pointer of f.
func (f *CaptureFile) Reset() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.readOffset = f.initialReadOffset
}

// Size returns the size of the file.
func (f *CaptureFile) Size() int64 {
	return f.size
}

// GetPath returns the path of the underlying os.File.
func (f *CaptureFile) GetPath() string {
	return f.path
}

// Close closes the file.
func (f *CaptureFile) Close() error {
	return f.file.Close()
}

// Delete deletes the file.
func (f *CaptureFile) Delete() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.file.Close(); err != nil {
		return err
	}
	return os.Remove(f.GetPath())
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == CompletedCaptureFileExt
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

// FilePathWithReplacedReservedChars returns the filepath with substitutions
// for reserved characters.
func FilePathWithReplacedReservedChars(filepath string) string {
	return strings.ReplaceAll(filepath, filePathReservedChars, "_")
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// TODO DATA-246: Implement this in some more robust, programmatic way.
func getDataType(methodName string) v1.DataType {
	switch methodName {
	case nextPointCloud, readImage, pointCloudMap, GetImages:
		return v1.DataType_DATA_TYPE_BINARY_SENSOR
	default:
		return v1.DataType_DATA_TYPE_TABULAR_SENSOR
	}
}

// SensorDataFromCaptureFilePath returns all readings in the file at filePath.
// NOTE: (Nick S) At time of writing this is only used in tests.
func SensorDataFromCaptureFilePath(filePath string) ([]*v1.SensorData, error) {
	//nolint:gosec
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	dcFile, err := NewCaptureFile(f)
	if err != nil {
		return nil, err
	}

	return SensorDataFromCaptureFile(dcFile)
}

// SensorDataFromCaptureFile returns all readings in f.
func SensorDataFromCaptureFile(f *CaptureFile) ([]*v1.SensorData, error) {
	f.Reset()
	var ret []*v1.SensorData
	for {
		next, err := f.ReadNext()
		if err != nil {
			// TODO: This swallows errors if the capture file has invalid proto in it
			// https://viam.atlassian.net/browse/DATA-3068
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}
		ret = append(ret, next)
	}
	return ret, nil
}
