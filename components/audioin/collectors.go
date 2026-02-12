package audioin

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go.opencensus.io/trace"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	rutils "go.viam.com/rdk/utils"
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

	codec := rutils.CodecPCM16
	if codecParam := params.MethodParams["codec"]; codecParam != nil {
		val := &structpb.Value{}
		if err := codecParam.UnmarshalTo(val); err == nil {
			codec = val.GetStringValue()
		}
	}

	// Use the capture interval as the stream duration
	durationSeconds := float32(params.Interval.Seconds())
	var previousTimestamp int64

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		_, span := trace.StartSpan(ctx, "audioin::data::collector::CaptureFunc::GetAudio")
		defer span.End()

		audioChan, err := audioIn.GetAudio(ctx, codec, durationSeconds, previousTimestamp, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if data.IsNoCaptureToStoreError(err) {
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
					binary, err := buildPayload(currentBuffer, currentSR, currentCh, codec)
					if err != nil {
						return data.CaptureResult{}, err
					}
					binaries = append(binaries, binary)
				}
				break loop
			case chunk, ok := <-audioChan:
				if !ok {
					if len(currentBuffer) > 0 {
						binary, err := buildPayload(currentBuffer, currentSR, currentCh, codec)
						if err != nil {
							return data.CaptureResult{}, err
						}
						binaries = append(binaries, binary)
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

				if chunk.AudioInfo == nil {
					return data.CaptureResult{}, fmt.Errorf("received audio chunk with nil AudioInfo")
				}

				if chunk.AudioInfo.SampleRateHz == currentSR && chunk.AudioInfo.NumChannels == currentCh {
					currentBuffer = append(currentBuffer, chunk.AudioData...)
				} else {
					// Audio format changed: finalize the current audio file and start a new one
					binary, err := buildPayload(currentBuffer, currentSR, currentCh, codec)
					if err != nil {
						return data.CaptureResult{}, err
					}
					binaries = append(binaries, binary)
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

func buildPayload(audioData []byte, sr, ch int32, codec string) (data.Binary, error) {
	var binary data.Binary
	var payload []byte

	switch codec {
	case rutils.CodecPCM16, rutils.CodecPCM32, rutils.CodecPCM32Float:
		var err error
		payload, err = CreateWAVFile(audioData, sr, ch, codec)
		if err != nil {
			return data.Binary{}, fmt.Errorf("error writing wav file: %w", err)
		}
	default:
		payload = audioData
	}

	binary.Payload = payload
	binary.MimeType = data.MimeTypeUnspecified
	return binary, nil
}

// CreateWAVFile creates a complete WAV file with header from PCM audio data.
func CreateWAVFile(pcmData []byte, sampleRate, numChannels int32, codec string) ([]byte, error) {
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
	default:
		return nil, fmt.Errorf("unsupported codec: %v", codec)
	}

	// WAV file header
	buf.WriteString("RIFF")
	if err := binary.Write(&buf, binary.LittleEndian, uint32(chunkBaseSize+len(pcmData))); err != nil {
		return nil, err
	}
	buf.WriteString("WAVE")

	// "fmt " sub-chunk
	buf.WriteString("fmt ")
	// length of fmt sub chunk
	if err := binary.Write(&buf, binary.LittleEndian, uint32(16)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, audioFormat); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(numChannels)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return nil, err
	}
	byteRate := sampleRate * numChannels * int32(bitsPerSample) / 8
	if err := binary.Write(&buf, binary.LittleEndian, uint32(byteRate)); err != nil {
		return nil, err
	}
	blockAlign := numChannels * int32(bitsPerSample) / 8
	if err := binary.Write(&buf, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, bitsPerSample); err != nil {
		return nil, err
	}

	// "data" sub-chunk
	buf.WriteString("data")
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData))); err != nil {
		return nil, err
	}
	buf.Write(pcmData)

	return buf.Bytes(), nil
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
