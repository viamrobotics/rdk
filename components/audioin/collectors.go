package audioin

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"time"

	"go.opencensus.io/trace"
	rutils "go.viam.com/rdk/utils"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getAudio method = iota
	doCommand
)

func (m method) String() string {
	switch m {
	case getAudio:
		return "GetAudio"
	case doCommand:
		return "DoCommand"
	}
	return "Unknown"
}

func newGetAudioCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	audioIn, err := assertAudioIn(resource)
	if err != nil {
		return nil, err
	}

	// Parse codec parameter (default: pcm16)
	codec := rutils.CodecPCM16
	if codecParam := params.MethodParams["codec"]; codecParam != nil {
		// Try to unmarshal as StringValue wrapper
		strVal := &wrapperspb.StringValue{}
		if err := codecParam.UnmarshalTo(strVal); err == nil {
			codec = strVal.Value
		} else {
			// Try as structpb.Value
			val := &structpb.Value{}
			if err := codecParam.UnmarshalTo(val); err == nil {
				codec = val.GetStringValue()
			}
		}
	}

	// Use the capture interval as the stream duration
	durationSeconds := float32(params.Interval.Seconds())

	var previousTimestamp int64

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::NextPointCloud")
		defer span.End()

		audioChan, err := audioIn.GetAudio(ctx, codec, durationSeconds, previousTimestamp, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, "GetAudio", err)
		}

		var currentBuffer []byte
		var currentSR, currentCh int32
		var binaries []data.Binary
	loop:
		for {
			select {
			case <-ctx.Done():
				// finalize current buffer if any, then exit
				if len(currentBuffer) > 0 {
					binaries = append(binaries, buildPayload(currentBuffer, currentSR, currentCh, codec))
				}
				break loop
			case chunk, ok := <-audioChan:
				if !ok {
					if len(currentBuffer) > 0 {
						binaries = append(binaries, buildPayload(currentBuffer, currentSR, currentCh, codec))
					}
					break loop
				}

				previousTimestamp = chunk.EndTimestampNanoseconds

				if len(currentBuffer) == 0 {
					currentSR = chunk.AudioInfo.SampleRateHz
					currentCh = chunk.AudioInfo.NumChannels
					currentBuffer = append(currentBuffer, chunk.AudioData...)
					continue
				}

				if chunk.AudioInfo.SampleRateHz == currentSR && chunk.AudioInfo.NumChannels == currentCh {
					currentBuffer = append(currentBuffer, chunk.AudioData...)
				} else {
					// Format changed: finalize current buffer
					binaries = append(binaries, buildPayload(currentBuffer, currentSR, currentCh, codec))
					// Start new buffer with this chunk
					currentBuffer = append([]byte{}, chunk.AudioData...)
					currentSR = chunk.AudioInfo.SampleRateHz
					currentCh = chunk.AudioInfo.NumChannels
				}
			}

		}

		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}

		return data.NewBinaryCaptureResult(ts, binaries), nil
	})

	return data.NewCollector(cFunc, params)
}

func assertAudioIn(resource interface{}) (AudioIn, error) {
	audioIn, ok := resource.(AudioIn)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return audioIn, nil
}

func buildPayload(audioData []byte, sr, ch int32, codec string) data.Binary {
	var binary data.Binary
	var payload []byte

	switch codec {
	case rutils.CodecPCM16, rutils.CodecPCM32, rutils.CodecPCM32Float:
		payload = CreateWAVFile(audioData, sr, ch, codec)
	default:
		payload = audioData
	}

	binary.Payload = payload
	binary.MimeType = data.MimeTypeUnspecified
	return binary
}

// CreateWAVFile creates a complete WAV file with header from PCM audio data.
func CreateWAVFile(pcmData []byte, sampleRate int32, numChannels int32, codec string) []byte {
	var buf bytes.Buffer
	var audioFormat uint16
	var bitsPerSample uint16
	const chunkBaseSize = 36

	switch codec {
	case rutils.CodecPCM16:
		audioFormat = 1
		bitsPerSample = 16
	case rutils.CodecPCM32:
		audioFormat = 1
		bitsPerSample = 32
	case rutils.CodecPCM32Float:
		audioFormat = 3
		bitsPerSample = 32
	}

	// WAV file header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(chunkBaseSize+len(pcmData)))
	buf.WriteString("WAVE")

	// "fmt " sub-chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(bitsPerSample))
	binary.Write(&buf, binary.LittleEndian, uint16(audioFormat))
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := sampleRate * numChannels * int32(bitsPerSample/8)
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	blockAlign := numChannels * int32(bitsPerSample/8)
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))

	// "data" sub-chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)

	return buf.Bytes()
}

// NewDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	audioin, err := assertAudioIn(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(audioin, params)
	return data.NewCollector(cFunc, params)
}
