package audioin_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/data"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	componentName   = "audioin"
	captureInterval = 1 * time.Second
	testSampleRate  = 2000
	testNumChannels = 1
)

var doCommandMap = map[string]any{"readings": "random-test"}

func newAudioIn(chunks []chunkConfig) audioin.AudioIn {
	audioIn := &inject.AudioIn{}
	audioIn.GetAudioFunc = func(ctx context.Context,
		codec string,
		durationSeconds float32,
		previousTimestampNs int64,
		extra map[string]interface{},
	) (chan *audioin.AudioChunk, error) {
		ch := make(chan *audioin.AudioChunk, 10)

		go func() {
			defer close(ch)

			if chunks != nil {
				// Send provided chunks
				for i, chunkCfg := range chunks {
					select {
					case ch <- makeAudioChunkFromChunkConfig(i, chunkCfg, codec):
					case <-ctx.Done():
						return
					}
				}
			} else {
				// Send default chunk for basic tests
				sampleRate := int32(testSampleRate)
				numChannels := int32(testNumChannels)
				numSamples := int(float32(sampleRate) * durationSeconds)
				var audioData []byte

				switch codec {
				case rutils.CodecPCM16:
					audioData = make([]byte, numSamples*2) // 2 bytes per 16-bit sample
				case rutils.CodecPCM32, rutils.CodecPCM32Float:
					audioData = make([]byte, numSamples*4) // 4 bytes per 32-bit sample
				default:
					audioData = make([]byte, numSamples*2) // default to 16-bit
				}
				for i := range audioData {
					audioData[i] = byte(i % 256)
				}

				startNs := int64(1234567890)
				endNs := startNs + int64(durationSeconds*1e9)

				chunk := &audioin.AudioChunk{
					AudioData: audioData,
					AudioInfo: &rutils.AudioInfo{
						SampleRateHz: sampleRate,
						NumChannels:  numChannels,
					},
					Sequence:                  0,
					StartTimestampNanoseconds: startNs,
					EndTimestampNanoseconds:   endNs,
				}

				select {
				case ch <- chunk:
				case <-ctx.Done():
				}
			}
		}()

		return ch, nil
	}

	audioIn.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return audioIn
}

type chunkConfig struct {
	sampleRate  int32
	numChannels int32
	dataSize    int
}

// makeAudioChunkFromSpec builds an AudioChunk
func makeAudioChunkFromChunkConfig(i int, cfg chunkConfig, codec string) *audioin.AudioChunk {
	audioData := make([]byte, cfg.dataSize)
	for j := range audioData {
		audioData[j] = byte(j % 256)
	}

	startNs := int64(i) * 1e9
	endNs := startNs + int64(1e9)

	return &audioin.AudioChunk{
		AudioData: audioData,
		AudioInfo: &rutils.AudioInfo{
			SampleRateHz: cfg.sampleRate,
			NumChannels:  cfg.numChannels,
			Codec:        codec,
		},
		Sequence:                  int32(i),
		StartTimestampNanoseconds: startNs,
		EndTimestampNanoseconds:   endNs,
	}
}

func TestGetAudioCollector(t *testing.T) {
	tests := []struct {
		name             string
		codec            string
		expectedDataSize int

		isMP3 bool
	}{
		{
			name:             "GetAudio collector with PCM16 should write WAV binary data",
			codec:            rutils.CodecPCM16,
			expectedDataSize: 4044, // 2000 samples * 2 bytes + 44 byte WAV header
			isMP3:            false,
		},
		{
			name:             "GetAudio collector with PCM32 should write WAV binary data",
			codec:            rutils.CodecPCM32,
			expectedDataSize: 8044, // 2000 samples * 4 bytes + 44 byte WAV header
			isMP3:            false,
		},
		{
			name:             "GetAudio collector with PCM32Float should write WAV binary data",
			codec:            rutils.CodecPCM32Float,
			expectedDataSize: 8044, // 2000 samples * 4 bytes + 44 byte WAV header
			isMP3:            false,
		},
		{
			name:             "GetAudio collector with MP3 should write MP3 binary data",
			codec:            rutils.CodecMP3,
			expectedDataSize: 4000, // 2000 samples * 2 bytes
			isMP3:            true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			codecStruct, err := anypb.New(structpb.NewStringValue(tc.codec))
			test.That(t, err, test.ShouldBeNil)

			params := data.CollectorParams{
				DataType:      data.CaptureTypeBinary,
				ComponentName: componentName,
				ComponentType: "audio_in",
				MethodName:    "GetAudio",
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				QueueSize:     10,
				BufferSize:    10,
				MethodParams: map[string]*anypb.Any{
					"codec": codecStruct,
				},
			}

			audioIn := newAudioIn(nil)

			collectorConstructor := data.CollectorLookup(data.MethodMetadata{
				API:        audioin.API,
				MethodName: "GetAudio",
			})
			test.That(t, collectorConstructor, test.ShouldNotBeNil)

			col, err := collectorConstructor(audioIn, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			// create pcmData to match newAudioIn's output
			durationSeconds := float32(captureInterval.Seconds())
			numSamples := int(float32(testSampleRate) * durationSeconds)

			var pcmData []byte
			var bytesPerSample int
			switch tc.codec {
			case rutils.CodecPCM16, rutils.CodecMP3:
				bytesPerSample = 2
			case rutils.CodecPCM32, rutils.CodecPCM32Float:
				bytesPerSample = 4
			default:
				bytesPerSample = 2
			}
			pcmData = make([]byte, numSamples*bytesPerSample)
			for i := range pcmData {
				pcmData[i] = byte(i % 256)
			}

			var expectedBinary []byte
			if tc.codec == rutils.CodecMP3 {
				expectedBinary = pcmData
			} else {
				expectedBinary, err = audioin.CreateWAVFile(pcmData, testSampleRate, testNumChannels, tc.codec)
				test.That(t, err, test.ShouldBeNil)
			}
			expected := []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data:     &datasyncpb.SensorData_Binary{Binary: expectedBinary},
			}}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, expected)
			buf.Close()
		})
	}
}

func TestGetAudioCollectorFormatChanges(t *testing.T) {
	tests := []struct {
		name             string
		codec            string
		chunks           []chunkConfig
		expectedBinaries int
	}{
		{
			name:  "Sample rate changes should create multiple binaries",
			codec: rutils.CodecPCM16,
			chunks: []chunkConfig{
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
				{sampleRate: 48000, numChannels: 1, dataSize: 1000},
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
			},
			expectedBinaries: 3,
		},
		{
			name:  "Channel count changes should create multiple binaries",
			codec: rutils.CodecPCM16,
			chunks: []chunkConfig{
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
				{sampleRate: 44100, numChannels: 2, dataSize: 1000},
			},
			expectedBinaries: 2,
		},
		{
			name:  "Same format should create single binary",
			codec: rutils.CodecPCM16,
			chunks: []chunkConfig{
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
				{sampleRate: 44100, numChannels: 1, dataSize: 1000},
			},
			expectedBinaries: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := tu.NewMockBuffer(t)
			codecAny, err := anypb.New(structpb.NewStringValue(tc.codec))
			test.That(t, err, test.ShouldBeNil)

			params := data.CollectorParams{
				DataType:      data.CaptureTypeBinary,
				ComponentName: componentName,
				ComponentType: "audio_in",
				MethodName:    "GetAudio",
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				QueueSize:     10,
				BufferSize:    10,
				MethodParams: map[string]*anypb.Any{
					"codec": codecAny,
				},
			}

			audioIn := newAudioIn(tc.chunks)

			collectorConstructor := data.CollectorLookup(data.MethodMetadata{
				API:        audioin.API,
				MethodName: "GetAudio",
			})
			test.That(t, collectorConstructor, test.ShouldNotBeNil)

			col, err := collectorConstructor(audioIn, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Read from the buffer
			var writes []*datasyncpb.SensorData
			select {
			case <-ctx.Done():
				t.Error("timeout waiting for data")
				t.FailNow()
			case writes = <-buf.Writes:
			}

			// Assert correct number of binaries were made
			test.That(t, len(writes), test.ShouldEqual, tc.expectedBinaries)

			buf.Close()
		})
	}
}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: time.Millisecond,
		DoCommandMap:    doCommandMap,
		Collector:       audioin.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newAudioIn(nil) },
	})
}
