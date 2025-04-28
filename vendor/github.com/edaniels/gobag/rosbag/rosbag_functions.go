package rosbag

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"go.uber.org/zap"
)

// SetSource set's name for the source
func (r *RosBag) SetSource(source string) {
	r.source = source
}

// GetSource get's name for the source
func (r *RosBag) GetSource() string {
	return r.source
}

// DumpChunkInfo dumps all messages with related data for debuging
func (r *RosBag) DumpChunkInfo(filename string) error {
	// For every chunk
	for i, c := range r.Chunks {
		// Uncompress chunk
		log.Debug("Chunk nr", zap.Int("nr", i))
		fmt.Printf("Size: %d, Start offset: %d, Uncompressed size %d, Compression type %s\n", c.size, c.startOffset, c.uncompressedSize, c.compressionType)
	}
	return nil
}

// DumpChunks dumps all messages with related data for debuging
func (r *RosBag) DumpChunks(filename string) error {
	// For every chunk
	for i, c := range r.Chunks {
		// Uncompress chunk
		log.Debug("Chunk nr", zap.Int("nr", i))
		var err error
		err = r.uncompressChunk(c)
		if err != nil {
			log.Error("Error on uncompressing chunk", zap.Error(err))
			return err
		}
		basename := strings.Split(filepath.Base(filename), ".")[0]
		err = ioutil.WriteFile(basename+"-chunk-"+strconv.Itoa(i)+".bin", r.uncompressedChunk, 0644)
		if err != nil {
			log.Error("Error on writing uncompressed chunk", zap.Error(err))
			return err
		}
	}
	return nil
}

// DumpMessageDefinitions dumps all messages with related data for debuging
func (r *RosBag) DumpMessageDefinitions(filename string) error {
	basename := strings.Split(filepath.Base(filename), ".")[0]
	var noSlashTopic string
	// for every chunk/index pair
	for _, connection := range r.Connections {
		// connection.MD5sum
		log.Debug("Type", zap.String("connection.ConnectionType", connection.ConnectionType))

		if connection.HeaderTopic[0] == '/' {
			noSlashTopic = strings.ToLower(strings.Replace(connection.HeaderTopic[1:], "/", "_", -1))
		} else {
			noSlashTopic = strings.ToLower(strings.Replace(connection.HeaderTopic, "/", "_", -1))
		}

		err := ioutil.WriteFile(basename+"-"+noSlashTopic+".msg", connection.MessageDefinition, 0644)
		if err != nil {
			log.Error("Error on writing message definition", zap.Error(err))
			return err
		}
	}
	return nil
}

// ParseTopicsToJSON parses messages into JSON inside bag structure
func (r *RosBag) ParseTopicsToJSON(extraFields string, timeFilter func(int64) bool, topicFilter func(string) bool, addTopicToMeta bool) (err error) {
	var noSlashTopic string
	// Recover from panics gracefully, log invalid bag and go to next one
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(runtime.Error); ok {
				log.Error("Recovering from runtime error in ParseTopicsToJSON", zap.Error(err))
				err = r.(error)
			} else {
				log.Error("Recovering from unknown error in ParseTopicsToJSON")
			}
		}
	}()
	r.ob.Reset()
	// for every chunk/index pair
	for i, c := range r.Chunks {
		// Uncompress chunk
		log.Debug("Chunk nr", zap.Int("i", i))
		var err error
		err = r.uncompressChunk(c)
		if err != nil {
			log.Error("Error on uncompressing chunk", zap.Error(err))
			return err
		}
		// For each Index in RosIndexData
		for j, index := range r.Indexes[i].Index {
			log.Debug("Index", zap.Int("j", j))
			// Find respective connection, get channel and format hash
			connection := r.Connections[index.ConnectionID]
			log.Debug("Check topic filter", zap.String("connection.HeaderTopic", connection.HeaderTopic), zap.Bool("topicFilter(connection.HeaderTopic)", topicFilter(connection.HeaderTopic)))
			if !topicFilter(connection.HeaderTopic) {
				continue
			}
			extraFieldsAmended := extraFields
			if addTopicToMeta {
				if len(extraFieldsAmended) > 0 {
					extraFieldsAmended = extraFieldsAmended + ","
				}
				extraFieldsAmended = extraFieldsAmended + "\"topic\":" + "\"" + connection.HeaderTopic + "\""
			}
			log.Debug("Topic", zap.String("connection.HeaderTopic", connection.HeaderTopic))
			log.Debug("Type", zap.String("connection.ConnectionType", connection.ConnectionType))
			log.Debug("Definition", zap.String("connection.MessageDefinition", string(connection.MessageDefinition[:])))
			compiledMessageMapLock.RLock()
			compiledMessageFormat := compiledMessages[connection.MD5sum]
			compiledMessageMapLock.RUnlock()
			log.Debug("Message format", zap.String("format", fmt.Sprintf("%v", compiledMessageFormat)))
			// For each offset in RosIndexMessageOffsets
			// Seek to right place in bytestream
			for _, messageOffset := range index.OffsetArray {
				// Process Message record header
				if !timeFilter(int64(messageOffset.sec)) {
					continue
				}
				var size int32
				headerLength := *(*int32)(unsafe.Pointer(&r.uncompressedChunk[messageOffset.offset]))
				// Size + offset will point now to the start of actual data strema
				size = size + 4 + headerLength + 4
				// Get input buffer for message building
				if len(connection.HeaderTopic) == 0 {
					err := fmt.Errorf("Connection topic is empty")
					log.Error("Connection topic is empty", zap.Error(err))
					return err
				}

				if connection.HeaderTopic[0] == '/' {
					noSlashTopic = strings.ToLower(strings.Replace(connection.HeaderTopic[1:], "/", "_", -1))
				} else {
					noSlashTopic = strings.ToLower(strings.Replace(connection.HeaderTopic, "/", "_", -1))
				}

				r.ob.Reset()

				r.ob.WriteByte('{')
				r.ob.WriteString("\"meta\": {")
				if len(extraFieldsAmended) > 0 {
					r.ob.WriteString(extraFieldsAmended)
					r.ob.WriteString(",")
				}
				r.ob.WriteString("\"secs\":")
				r.ob.WriteString(strconv.FormatInt(int64(messageOffset.sec), 10))
				r.ob.WriteString(",")

				r.ob.WriteString("\"nsecs\":")
				r.ob.WriteString(strconv.FormatInt(int64(messageOffset.nanoSec), 10))
				r.ob.WriteByte('}')
				r.ob.WriteString(", \"data\":")

				size = r.readMessage(messageOffset.offset+size, compiledMessageFormat)
				if size == 0 {
					err = fmt.Errorf(noSlashTopic)
					log.Error("Error while reading message in topic: ", zap.Error(err))
					// return err
				}
				r.ob.WriteByte('}')
				r.ob.WriteByte('\n')

				buf, ok := r.TopicsAsJSON[noSlashTopic]
				if !ok {
					buf = NewBuffer()
				}
				_, err := buf.Write(r.ob.Bytes())
				if err != nil {
					log.Error("Error on writing parsed line to collecting buffer", zap.Error(err))
					return err
				}
				r.TopicsAsJSON[noSlashTopic] = buf
			}
		}
	}
	return nil
}

// WriteJSON writes all messages from the bag as JSON lines into writer
func (r *RosBag) WriteJSON(w io.Writer) error {
	log.Info("Starting WriteJSON")
	timeFilter := func(int64) bool { return true }
	topicFilter := func(string) bool { return true }
	err := r.ParseTopicsToJSON("", timeFilter, topicFilter, true)
	if err != nil {
		log.Error("Error while parsing bag to JSON", zap.Error(err))
		return err
	}

	for _, messages := range r.TopicsAsJSON {
		_, err := messages.WriteTo(w)
		if err != nil {
			log.Error("Error on writing message to Writer", zap.Error(err))
			return err
		}
	}
	log.Info("Done WriteJSON")
	return nil
}

// WriteTopicsJSON writes all messages from the bag as JSON lines into writer
func (r *RosBag) WriteTopicsJSON(outputPath string, startTime int64, endTime int64, topicsFilter []string) error {
	log.Info("Starting WriteTopicsJSON")
	var timeFilterFunc func(int64) bool
	if startTime == 0 || endTime == 0 {
		timeFilterFunc = func(timestamp int64) bool {
			return true
		}

	} else {
		timeFilterFunc = func(timestamp int64) bool {
			return timestamp >= startTime && timestamp <= endTime
		}
	}

	var topicFilterFunc func(string) bool
	if len(topicsFilter) == 0 {
		topicFilterFunc = func(string) bool {
			return true
		}
	} else {
		topicsFilterMap := make(map[string]bool)
		for _, topic := range topicsFilter {
			topicsFilterMap[topic] = true
		}
		topicFilterFunc = func(topic string) bool {
			_, ok := topicsFilterMap[topic]
			return ok
		}
	}

	err := r.ParseTopicsToJSON("", timeFilterFunc, topicFilterFunc, false)
	if err != nil {
		log.Error("Error while parsing bag to JSON", zap.Error(err))
		return err
	}

	for topic, messages := range r.TopicsAsJSON {
		var outFileName string
		if topic[0] == '/' {
			outFileName = strings.Replace(topic[1:], "/", "-", -1) + ".json"
		} else {
			outFileName = strings.Replace(topic, "/", "-", -1) + ".json"
		}
		outputFile, err := os.Create(filepath.Join(outputPath, outFileName))
		if err != nil {
			log.Error("Unable to open output file", zap.Error(err))
			return err
		}
		_, err = messages.WriteTo(outputFile)
		if err != nil {
			log.Error("Error on writing message to Writer", zap.Error(err))
			return err
		}
		outputFile.Close()
	}
	log.Info("Done WriteTopicsJSON")
	return nil
}
