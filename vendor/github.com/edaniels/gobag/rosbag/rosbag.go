package rosbag

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"sync"

	systemlog "log"

	"github.com/edaniels/gobag/msgpiler"
	"go.uber.org/zap"
)

var (
	// BufCache is a buffer cache
	BufCache chan *bytes.Buffer
	// BagCache is a bag cache
	BagCache chan *RosBag
)

var (
	compiledMessageMapLock sync.RWMutex
	compiledMessages       map[string]*msgpiler.MessageFormat
	compiledMessagesTopics map[string]string
	log                    *zap.Logger
	err                    error
)

func init() {
	compiledMessages = make(map[string]*msgpiler.MessageFormat)
	compiledMessagesTopics = make(map[string]string)
	BagCache = make(chan *RosBag, 10)
	BufCache = make(chan *bytes.Buffer, 20)
	log, err = zap.NewProduction()
	if err != nil {
		systemlog.Fatalf("can't initialize zap logger: %v", err)
	}
}

// RosBag is main interface to interact with single ROS bag
type RosBag struct {
	rawBytes          []byte
	source            string
	ob                *bytes.Buffer
	Version           string
	Length            int
	Headers           []RosRecordHeader
	Chunks            []RosChunk
	Indexes           []RosIndex
	Connections       map[int32]RosConnection
	uncompressedChunk []byte
	TopicsAsJSON      map[string]*bytes.Buffer
}

// RosRecordHeader holds data about ROS record header
type RosRecordHeader struct {
	op               byte
	chunkCount       int32
	connCount        int32
	indexPos         int64
	compressionValue string
	size             int32
	conn             int32
	topic            string
	count            int32
	ver              int32
	chunkPos         int64
	startTime        int64
	endTime          int64
	time             int64
}

// RosChunk holds data about ROS chunk in the bag
type RosChunk struct {
	startOffset      int64
	size             int64
	compressionType  string
	uncompressedSize int64
}

// RosConnection holds data about ROS connections in the bag
type RosConnection struct {
	ConnectionID      int32
	HeaderTopic       string
	Topic             string
	ConnectionType    string
	MD5sum            string
	MessageDefinition []byte
	CallerID          string
	Latching          string
}

// RosIndex holds data about index records
type RosIndex struct {
	Index []RosIndexData
}

// RosIndexData holds data about index records
type RosIndexData struct {
	ConnectionID int32
	MessageCount int32
	OffsetArray  []RosIndexMessageOffsets
}

// RosIndexMessageOffsets holds data about index records
type RosIndexMessageOffsets struct {
	sec     int32
	nanoSec int32
	offset  int32
}

// processRecordData parses record data out of given stream
func (r *RosBag) processRecordData(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	// Now we are in the beginning of data record
	switch rh.op {
	case 2: // Message data
		err := r.processMessageData(bagReader, rh)
		if err != nil {
			log.Error("Error while processing message data", zap.Error(err))
			return err
		}
	case 4: // Index data
		err := r.processIndexData(bagReader, rh)
		if err != nil {
			log.Error("Error while processing index data", zap.Error(err))
			return err
		}
	case 5: // Chunk data
		err := r.processChunkData(bagReader, rh)
		if err != nil {
			log.Error("Error while processing chunk data", zap.Error(err))
			return err
		}
	case 6: // Chunk info
		err := r.processChunkInfo(bagReader, rh)
		if err != nil {
			log.Error("Error while processing chunk info", zap.Error(err))
			return err
		}
	case 7: // Connection data
		err := r.processConnectionData(bagReader, rh)
		if err != nil {
			log.Error("Error while processing connection data", zap.Error(err))
			return err
		}
	default:
		err := r.processDefault(bagReader)
		if err != nil {
			log.Error("Error while discarding record data", zap.Error(err))
			return err
		}
	}
	return nil
}

// readRecordHeader parses record header out of given stream
func (r *RosBag) readRecordHeader(bagReader *bytes.Reader) (*RosRecordHeader, error) {
	var (
		headerLength int32
		fieldLength  int32
		recordHeader RosRecordHeader
	)

	err := binary.Read(bagReader, binary.LittleEndian, &headerLength)
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		log.Error("Error while reading record header length", zap.Error(err))
		return nil, err
	}
	for headerLength > 0 {

		err = binary.Read(bagReader, binary.LittleEndian, &fieldLength)
		if err != nil {
			log.Error("Error while reading header field length", zap.Error(err))
			return nil, err
		}
		headerLength -= 4
		headerLength -= fieldLength

		fieldName, err := ReadString(bagReader, '=')
		if err != nil {
			log.Error("Error while reading header field name", zap.Error(err))
			return nil, err
		}
		switch {
		case fieldName == "op=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.op)
		case fieldName == "chunk_count=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.chunkCount)
		case fieldName == "conn_count=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.connCount)
		case fieldName == "index_pos=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.indexPos)
		case fieldName == "compression=":
			compressionValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, compressionValue)
			recordHeader.compressionValue = string(compressionValue)
		case fieldName == "size=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.size)
		case fieldName == "conn=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.conn)
		case fieldName == "topic=":
			topic := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, topic)
			recordHeader.topic = string(topic)
		case fieldName == "count=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.count)
		case fieldName == "ver=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.ver)
		case fieldName == "chunk_pos=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.chunkPos)
		case fieldName == "start_time=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.startTime)
		case fieldName == "end_time=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.endTime)
		case fieldName == "time=":
			err = binary.Read(bagReader, binary.LittleEndian, &recordHeader.time)
		default:
			log.Error("Unhandled field", zap.String("fieldName", fieldName))
			_, err = bagReader.Seek(int64(fieldLength)-int64(len(fieldName)), 1)
		}
		if err != nil {
			log.Error("error while reading header field data", zap.Error(err))
			return nil, err
		}
	}
	return &recordHeader, nil
}

// Process raw bytes into structures
func (r *RosBag) processBag() error {
	var signature [13]byte
	bagReader := bytes.NewReader(r.rawBytes)

	err := binary.Read(bagReader, binary.LittleEndian, &signature)
	if err != nil {
		return err
	}
	r.Version = string(signature[:])
	// Process records in bag data
	for {
		recordHeader, err := r.readRecordHeader(bagReader)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Error("Error while processing record header", zap.Error(err))
				return err
			}
		}
		r.Headers = append(r.Headers, *recordHeader)

		err = r.processRecordData(bagReader, recordHeader)
		if err != nil {
			log.Error("Error while processing record data", zap.Error(err))
			return err
		}
	}
	return err
}

// Reuse existing bag to ease burden on GC in case of massive usage
func (r *RosBag) Read(input io.Reader) error {
	r.rawBytes = r.rawBytes[:cap(r.rawBytes)]
	readBytes, err := io.ReadFull(input, r.rawBytes)
	if err != nil {
		if err.Error() == "unexpected EOF" {
			// Everything is good, we read less than buffer size
			r.rawBytes = r.rawBytes[:readBytes]
		} else {
			// Some other error condition, this is bad
			return err
		}
	} else {
		// Watch out we have badass here. Does not fit to our buffer.
		restOfBytes, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}
		r.rawBytes = append(r.rawBytes, restOfBytes...)
	}
	r.Length = len(r.rawBytes)
	// Process bag internal structure
	err = r.processBag()
	if err != nil {
		log.Error("Error while processing input stream", zap.Error(err))
		return err
	}
	return nil
}

// ReadString imitates bufio.ReadString
func ReadString(r io.Reader, delim byte) (string, error) {
	buf := make([]byte, 1)
	var output []byte
	for {
		_, err := r.Read(buf)
		if err != nil {
			return "", err
		}
		output = append(output, buf[0])
		if buf[0] == delim {
			break
		}
	}
	return string(output[:]), nil
}

// NewBuffer provides new buffer, using cache if possible
func NewBuffer() (b *bytes.Buffer) {
	select {
	case b = <-BufCache:
	default:
		b = new(bytes.Buffer)
	}
	return
}

// NewRosBag will return rosbag, ready to be recycled
func NewRosBag() (r *RosBag) {
	select {
	case r = <-BagCache:
	default:
		r = initRosBag()
	}
	r.Connections = make(map[int32]RosConnection)
	r.rawBytes = r.rawBytes[:0]
	r.uncompressedChunk = r.uncompressedChunk[:0]
	r.ob.Reset()
	for key := range r.TopicsAsJSON {
		buf := NewBuffer()
		buf.Reset()
		r.TopicsAsJSON[key] = buf
	}
	r.Headers = r.Headers[:0]
	r.Chunks = r.Chunks[:0]
	r.Indexes = r.Indexes[:0]
	return
}

// initRosBag will create new RosBag from bytes that can be read from Reader
func initRosBag() *RosBag {
	var (
		rb RosBag
	)
	rb.Connections = make(map[int32]RosConnection)
	rb.rawBytes = make([]byte, 15*1024*1024)         // 15 MB buffer
	rb.uncompressedChunk = make([]byte, 5*1024*1024) // 5 MB buffer
	internalArray := make([]byte, 5*1024*1024)
	rb.ob = bytes.NewBuffer(internalArray)
	rb.TopicsAsJSON = make(map[string]*bytes.Buffer)
	return &rb
}
