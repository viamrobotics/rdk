package data

import (
	"bufio"
	"encoding/binary"
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
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

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

var errInvalidVarint = errors.New("invalid varint32 encountered")

func ReadTag(r io.Reader, num *protowire.Number, typ *protowire.Type) (n int, err error) {
	// Per AbstractParser#parsePartialDelimitedFrom with
	// CodedInputStream#readRawVarint32.
	var headerBuf [binary.MaxVarintLen32]byte
	var bytesRead, length int
	var tagNum protowire.Number
	var tagType protowire.Type
	for length <= 0 { // i.e. no varint has been decoded yet.
		if bytesRead >= len(headerBuf) {
			return bytesRead, errInvalidVarint
		}
		// We have to read byte by byte here to avoid reading more bytes
		// than required. Each read byte is appended to what we have
		// read before.
		newBytesRead, err := r.Read(headerBuf[bytesRead : bytesRead+1])
		if newBytesRead == 0 {
			if err != nil {
				return bytesRead, err
			}
			// A Reader should not return (0, nil), but if it does,
			// it should be treated as no-op (according to the
			// Reader contract). So let's go on...
			continue
		}
		bytesRead += newBytesRead
		// Now present everything read so far to the varint decoder and
		// see if a varint with a tag type can be decoded already.
		tagNum, tagType, length = protowire.ConsumeTag(headerBuf[:bytesRead])
	}
	*num = tagNum
	*typ = tagType
	return bytesRead, nil
}

// *SensorMetadata
func ReadMessageLength(r io.Reader, m *uint64) (n int, err error) {
	// Per AbstractParser#parsePartialDelimitedFrom with
	// CodedInputStream#readRawVarint32.
	var headerBuf [binary.MaxVarintLen32]byte
	var bytesRead, varIntBytes int
	var messageLength uint64
	for varIntBytes <= 0 { // i.e. no varint has been decoded yet.
		if bytesRead >= len(headerBuf) {
			return bytesRead, errInvalidVarint
		}
		// We have to read byte by byte here to avoid reading more bytes
		// than required. Each read byte is appended to what we have
		// read before.
		newBytesRead, err := r.Read(headerBuf[bytesRead : bytesRead+1])
		if newBytesRead == 0 {
			if err != nil {
				return bytesRead, err
			}
			// A Reader should not return (0, nil), but if it does,
			// it should be treated as no-op (according to the
			// Reader contract). So let's go on...
			continue
		}
		bytesRead += newBytesRead
		// Now present everything read so far to the varint decoder and
		// see if a varint can be decoded already.
		messageLength, varIntBytes = protowire.ConsumeVarint(headerBuf[:bytesRead])
	}
	*m = messageLength
	return bytesRead, nil
}

func (f *CaptureFile) BinaryReader(md *v1.SensorMetadata) (io.Reader, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.Metadata.Type != v1.DataType_DATA_TYPE_BINARY_SENSOR {
		return nil, errors.New("expected CaptureFile to be of type BINARY")
	}

	// seek to the first 32 bit varint delimeter
	if _, err := f.file.Seek(f.initialReadOffset, io.SeekStart); err != nil {
		return nil, err
	}
	// remove delimiter (we know we will only have one for the binary image)
	var topLevelMsgLen uint64
	bytesRead, err := ReadMessageLength(f.file, &topLevelMsgLen)
	if err != nil {
		return nil, err
	}
	actualLen := f.size - (f.initialReadOffset + int64(bytesRead))
	if int64(topLevelMsgLen) != actualLen {
		return nil, fmt.Errorf("binary capture file payload described as having byte size %d, actual size: %d", topLevelMsgLen, actualLen)
	}
	// now we parse the *v1.SensorMetadata and the binary payload of a binary *v1.SensorData
	var (
		tagNum  protowire.Number
		tagType protowire.Type
		n       int
	)
	n, err = ReadTag(f.file, &tagNum, &tagType)
	bytesRead += n
	if err != nil {
		return nil, err
	}

	if !tagNum.IsValid() {
		return nil, fmt.Errorf("tagNum %d is invalid", tagNum)
	}

	// TODO: Techically it isn't guranteed this value will be 1
	// but this code currently assumes it will for simplicity
	// see: https://protobuf.dev/programming-guides/encoding/#optional
	if tagNum != 1 {
		return nil, fmt.Errorf("expected tagNum to be 1 but instead it is %d", tagNum)
	}

	// expected LEN type https://protobuf.dev/programming-guides/encoding/#structure
	// in this case an embedded message
	if tagType != protowire.BytesType {
		return nil, fmt.Errorf("expected tagNum 1 to have LEN wire type, instead it has wire type: %d", tagType)
	}

	var sensorMDLen uint64
	n, err = ReadMessageLength(f.file, &sensorMDLen)
	bytesRead += n
	if err != nil {
		return nil, err
	}
	sensorMDBytes := make([]byte, sensorMDLen)
	n, err = io.ReadFull(f.file, sensorMDBytes)
	bytesRead += n
	if err != nil {
		return nil, err
	}
	err = proto.Unmarshal(sensorMDBytes, md)
	if err != nil {
		return nil, err
	}

	var (
		payloadTagNum  protowire.Number
		payloadTagType protowire.Type
	)
	n, err = ReadTag(f.file, &payloadTagNum, &payloadTagType)
	bytesRead += n
	if err != nil {
		return nil, err
	}

	if !payloadTagNum.IsValid() {
		return nil, fmt.Errorf("payloadTagNum %d is invalid", payloadTagNum)
	}

	// should be 3 as that is the field number of v1.SensorData's binary oneof
	if payloadTagNum != 3 {
		return nil, fmt.Errorf("expected payloadTagNum to be 3 but was actually %d", payloadTagNum)
	}

	if payloadTagType != protowire.BytesType {
		return nil, fmt.Errorf("expected payloadTagType LEN wire type, instead it has wire type: %d", payloadTagType)
	}

	var payloadLen uint64
	n, err = ReadMessageLength(f.file, &payloadLen)
	bytesRead += n
	if err != nil {
		return nil, err
	}
	actualPayloadLen := f.size - (f.initialReadOffset + int64(bytesRead))
	if int64(payloadLen) != actualPayloadLen {
		return nil, fmt.Errorf("capture file contains incomplete binary payload or data after the binary payload, payloadLength described in capture file: %d, actual payload length: %d, filesize: %d, bytesRead: %d", payloadLen, actualPayloadLen, f.size, bytesRead)
	}
	return bufio.NewReader(f.file), nil
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
