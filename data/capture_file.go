package data

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
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/resource"
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
	GetImages            = "GetImages"
	nextPointCloud       = "NextPointCloud"
	pointCloudMap        = "PointCloudMap"
	captureAllFromCamera = "CaptureAllFromCamera"
	// Non-exhaustive list of characters to strip from file paths, since not allowed
	// on certain file systems.
	filePathReservedChars = ":"
)

// CaptureFile is the data structure containing data captured by collectors. It is backed by a file on disk containing
// length delimited protobuf messages, where the first message is the CaptureMetadata for the file, and ensuing
// messages contain the captured data.
type CaptureFile struct {
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

// ReadCaptureFile creates a File struct from a passed os.File previously constructed using NewFile.
func ReadCaptureFile(f *os.File) (*CaptureFile, error) {
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
		writer:            bufio.NewWriter(f),
		size:              finfo.Size(),
		metadata:          md,
		initialReadOffset: int64(initOffset),
		readOffset:        int64(initOffset),
		writeOffset:       int64(initOffset),
	}

	return &ret, nil
}

// NewCaptureFile creates a new *CaptureFile with the specified md in the specified directory.
func NewCaptureFile(dir string, md *v1.DataCaptureMetadata) (*CaptureFile, error) {
	fileName := CaptureFilePathWithReplacedReservedChars(
		filepath.Join(dir, getFileTimestampName()) + InProgressCaptureFileExt)
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
	return &CaptureFile{
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
func (f *CaptureFile) ReadMetadata() *v1.DataCaptureMetadata {
	return f.metadata
}

// ReadNext returns the next SensorData reading.
func (f *CaptureFile) ReadNext() (*v1.SensorData, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if err := f.writer.Flush(); err != nil {
		return nil, err
	}

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

// WriteNext writes the next SensorData reading.
func (f *CaptureFile) WriteNext(data *v1.SensorData) error {
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
func (f *CaptureFile) Flush() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.writer.Flush()
}

// Reset resets the read pointer of f.
func (f *CaptureFile) Reset() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.readOffset = f.initialReadOffset
}

// Size returns the size of the file.
func (f *CaptureFile) Size() int64 {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.size
}

// GetPath returns the path of the underlying os.File.
func (f *CaptureFile) GetPath() string {
	return f.path
}

// Close closes the file.
func (f *CaptureFile) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.writer.Flush(); err != nil {
		return err
	}

	// Rename file to indicate that it is done being written.
	withoutExt := strings.TrimSuffix(f.file.Name(), filepath.Ext(f.file.Name()))
	newName := withoutExt + CompletedCaptureFileExt
	if err := f.file.Close(); err != nil {
		return err
	}
	return os.Rename(f.file.Name(), newName)
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

// BuildCaptureMetadata builds a DataCaptureMetadata object and returns error if
// additionalParams fails to convert to anypb map.
func BuildCaptureMetadata(
	api resource.API,
	name string,
	method string,
	additionalParams map[string]string,
	methodParams map[string]*anypb.Any,
	tags []string,
) (*v1.DataCaptureMetadata, CaptureType) {
	dataType := MethodToCaptureType(method)
	return &v1.DataCaptureMetadata{
		ComponentType:    api.String(),
		ComponentName:    name,
		MethodName:       method,
		Type:             dataType.ToProto(),
		MethodParameters: methodParams,
		FileExtension:    getFileExt(dataType, method, additionalParams),
		Tags:             tags,
	}, dataType
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == CompletedCaptureFileExt || filepath.Ext(f.Name()) == InProgressCaptureFileExt
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// SensorDataFromCaptureFilePath returns all readings in the file at filePath.
// NOTE: (Nick S) At time of writing this is only used in tests.
func SensorDataFromCaptureFilePath(filePath string) ([]*v1.SensorData, error) {
	//nolint:gosec
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	dcFile, err := ReadCaptureFile(f)
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

// CaptureFilePathWithReplacedReservedChars returns the filepath with substitutions
// for reserved characters.
func CaptureFilePathWithReplacedReservedChars(filepath string) string {
	return strings.ReplaceAll(filepath, filePathReservedChars, "_")
}
