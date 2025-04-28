package rosbag

import (
	"bytes"
	"encoding/binary"
	"io"

	"go.uber.org/zap"

	"github.com/edaniels/gobag/msgpiler"
)

// processDefault discards data from the stream
func (r *RosBag) processDefault(bagReader *bytes.Reader) error {
	var dataLength int32
	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		log.Error("Error while reading record data length", zap.Error(err))
		return err
	}
	_, err = bagReader.Seek(int64(dataLength), 1)
	if err != nil {
		log.Error("Error while discarding record data", zap.Error(err))
		return err
	}
	return nil
}

// processMessageData
func (r *RosBag) processMessageData(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	var dataLength int32
	log.Warn("Handling uncompressed type 2 \"Message Data\" is not implemented. It will be ignored")
	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		return err
	}
	_, err = bagReader.Seek(int64(dataLength), 1)
	if err != nil {
		return err
	}
	return nil
}

// processIndexData
func (r *RosBag) processIndexData(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	var (
		dataLength int32
		rid        RosIndexData
	)

	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		log.Error("Error while reading record data length", zap.Error(err))
		return err
	}
	rid.ConnectionID = rh.conn
	rid.MessageCount = rh.count
	rid.OffsetArray = make([]RosIndexMessageOffsets, rh.count)
	for i := int32(0); i < rh.count; i++ {
		var rimo RosIndexMessageOffsets
		err := binary.Read(bagReader, binary.LittleEndian, &rimo.sec)
		if err != nil {
			log.Error("Error while reading time seconds", zap.Error(err))
			return err
		}
		err = binary.Read(bagReader, binary.LittleEndian, &rimo.nanoSec)
		if err != nil {
			log.Error("Error while reading time nanosecond", zap.Error(err))
			return err
		}
		err = binary.Read(bagReader, binary.LittleEndian, &rimo.offset)
		if err != nil {
			log.Error("Error while reading offset", zap.Error(err))
			return err
		}
		rid.OffsetArray[i] = rimo
	}
	r.Indexes[len(r.Indexes)-1].Index = append(r.Indexes[len(r.Indexes)-1].Index, rid)
	return nil
}

// processChunkData parses connection data into RecordData structure
func (r *RosBag) processChunkData(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	var (
		dataLength int32
		rc         RosChunk
		ri         RosIndex
	)

	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		log.Error("Error while reading record data length", zap.Error(err))
		return err
	}
	rc.startOffset = bagReader.Size() - int64(bagReader.Len())
	rc.size = int64(dataLength)
	rc.compressionType = rh.compressionValue
	rc.uncompressedSize = int64(rh.size)
	r.Chunks = append(r.Chunks, rc)
	_, err = bagReader.Seek(int64(dataLength), 1)
	// There will be index data after so we need to create place for index
	r.Indexes = append(r.Indexes, ri)
	return nil
}

// processChunkInfo
func (r *RosBag) processChunkInfo(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	var dataLength int32
	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		log.Error("Error while reading record data length", zap.Error(err))
		return err
	}
	_, err = bagReader.Seek(int64(dataLength), 1)
	if err != nil {
		log.Error("Error while discarding record data", zap.Error(err))
		return err
	}
	return nil
}

// processConnectionData parses connection data into RecordData structure
func (r *RosBag) processConnectionData(bagReader *bytes.Reader, rh *RosRecordHeader) error {
	var (
		dataLength  int32
		fieldLength int32
		rc          RosConnection
	)
	err := binary.Read(bagReader, binary.LittleEndian, &dataLength)
	if err != nil {
		log.Error("Error while reading record data length", zap.Error(err))
		return err
	}

	for dataLength > 0 {
		err := binary.Read(bagReader, binary.LittleEndian, &fieldLength)
		if err != nil {
			log.Error("Reading field length while processing connection data failed", zap.Error(err))
			return err
		}
		dataLength -= 4
		dataLength -= fieldLength
		fieldName, err := ReadString(bagReader, '=')
		if err != nil {
			log.Error("Reading field name while processing connection data failed", zap.Error(err))
			return err
		}
		switch {
		case fieldName == "topic=":
			topicValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, topicValue)
			rc.Topic = string(topicValue)
		case fieldName == "type=":
			typeValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, typeValue)
			rc.ConnectionType = string(typeValue)
		case fieldName == "md5sum=":
			md5sumValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, md5sumValue)
			rc.MD5sum = string(md5sumValue)
		case fieldName == "message_definition=":
			mdValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, mdValue)
			rc.MessageDefinition = mdValue
		case fieldName == "callerid=":
			calleridValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, calleridValue)
			rc.CallerID = string(calleridValue)
		case fieldName == "latching=":
			laytchingValue := make([]byte, fieldLength-int32(len(fieldName)))
			_, err = io.ReadFull(bagReader, laytchingValue)
			rc.Latching = string(laytchingValue)
		default:
			log.Error("Unhandled field", zap.String("fieldName", fieldName))
			_, err = bagReader.Seek(int64(fieldLength)-int64(len(fieldName)), 1)
		}
		if err != nil {
			log.Error("Reading field value while processing connection data failed", zap.Error(err))
			return err
		}
	}
	rc.ConnectionID = rh.conn
	rc.HeaderTopic = rh.topic
	if len(rc.HeaderTopic) == 0 {
		log.Error("Skipping connection, header topic is empty")
		return nil
	}
	// Compile message description and add to compiled messages
	compiledMessageMapLock.RLock()
	if _, present := compiledMessages[rc.MD5sum]; !present {
		compiledMessageMapLock.RUnlock()
		compiledMessageMapLock.Lock()
		message, err := msgpiler.Compile(append(rc.MessageDefinition, '\n'), rc.ConnectionType)
		if err != nil {
			log.Error("Error while compiling MessageDefinition", zap.Error(err))
			compiledMessageMapLock.Unlock()
			return err
		}
		// Link abstract tree into executable tree
		err = linkMessage(message, message)
		if err != nil {
			log.Error("Error while linking rootmessage MessageFormats", zap.Error(err))
			compiledMessageMapLock.Unlock()
			return err
		}
		// Link all complex types defined in root message
		for _, m := range message.MessageFormats {
			err := linkMessage(message, &m)
			if err != nil {
				log.Error("Error while linking rootmessage MessageFormats", zap.Error(err))
				compiledMessageMapLock.Unlock()
				return err
			}
		}
		compiledMessages[rc.MD5sum] = message
		compiledMessagesTopics[rc.HeaderTopic+"|"+rc.MD5sum] = rc.MD5sum
		compiledMessageMapLock.Unlock()
	} else {
		compiledMessageMapLock.RUnlock()
	}

	compiledMessageMapLock.RLock()
	if _, present := compiledMessagesTopics[rc.HeaderTopic+"|"+rc.MD5sum]; !present {
		compiledMessageMapLock.RUnlock()
		compiledMessageMapLock.Lock()
		compiledMessagesTopics[rc.HeaderTopic+"|"+rc.MD5sum] = rc.MD5sum
		compiledMessageMapLock.Unlock()
	} else {
		compiledMessageMapLock.RUnlock()
	}

	r.Connections[rc.ConnectionID] = rc
	return nil
}
