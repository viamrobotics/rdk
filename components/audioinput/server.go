//go:build !no_media

package audioinput

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/go-audio/audio"
	"github.com/go-audio/transforms"
	"github.com/go-audio/wav"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioinput/v1"
	"go.viam.com/utils"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/src-d/go-billy.v4/memfs"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// HostEndian indicates the byte ordering this host natively uses.
var HostEndian binary.ByteOrder

func init() {
	// from github.com/pion/mediadevices/pkg/wave/decoder.go
	//nolint:gosec
	switch v := *(*uint16)(unsafe.Pointer(&([]byte{0x12, 0x34}[0]))); v {
	case 0x1234:
		HostEndian = binary.BigEndian
	case 0x3412:
		HostEndian = binary.LittleEndian
	default:
		panic(fmt.Sprintf("failed to determine host endianness: %x", v))
	}
}

// serviceServer implements the AudioInputService from audioinput.proto.
type serviceServer struct {
	pb.UnimplementedAudioInputServiceServer
	coll resource.APIResourceCollection[AudioInput]
}

// NewRPCServiceServer constructs an audio input gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[AudioInput]) interface{} {
	return &serviceServer{coll: coll}
}

// Chunks returns audio chunks (samples) forever from an audio input of the underlying robot. A specific sampling
// format can be requested but may not necessarily be the same one returned.
func (s *serviceServer) Chunks(req *pb.ChunksRequest, server pb.AudioInputService_ChunksServer) error {
	audioInput, err := s.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	chunkStream, err := audioInput.Stream(server.Context())
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(chunkStream.Close(server.Context()))
	}()

	firstChunk, release, err := chunkStream.Next(server.Context())
	if err != nil {
		return err
	}
	info := firstChunk.ChunkInfo()
	release()

	var sf wave.SampleFormat
	sfProto := req.SampleFormat
	switch req.SampleFormat {
	case pb.SampleFormat_SAMPLE_FORMAT_UNSPECIFIED:
		sfProto = pb.SampleFormat_SAMPLE_FORMAT_INT16_INTERLEAVED
		fallthrough
	case pb.SampleFormat_SAMPLE_FORMAT_INT16_INTERLEAVED:
		sf = wave.Int16SampleFormat
	case pb.SampleFormat_SAMPLE_FORMAT_FLOAT32_INTERLEAVED:
		sf = wave.Float32SampleFormat
	default:
		return errors.Errorf("unknown type of audio sample format %v", req.SampleFormat)
	}

	if err := server.Send(&pb.ChunksResponse{
		Type: &pb.ChunksResponse_Info{
			Info: &pb.AudioChunkInfo{
				SampleFormat: sfProto,
				Channels:     uint32(info.Channels),
				SamplingRate: int64(info.SamplingRate),
			},
		},
	}); err != nil {
		return err
	}

	sendNextChunk := func() error {
		chunk, release, err := chunkStream.Next(server.Context())
		if err != nil {
			return err
		}
		defer release()

		var outBytes []byte
		switch c := chunk.(type) {
		case *wave.Int16Interleaved:
			outBytes = make([]byte, len(c.Data)*2)
			buf := bytes.NewBuffer(outBytes[:0])
			chunkCopy := c
			if sf != chunk.SampleFormat() {
				chunkCopy = wave.NewInt16Interleaved(info)
				// convert
				for i := 0; i < c.Size.Len; i++ {
					for j := 0; j < c.Size.Channels; j++ {
						chunkCopy.Set(i, j, chunk.At(i, j))
					}
				}
			}
			if err := binary.Write(buf, HostEndian, chunkCopy.Data); err != nil {
				return err
			}
		case *wave.Float32Interleaved:
			outBytes = make([]byte, len(c.Data)*4)
			buf := bytes.NewBuffer(outBytes[:0])
			chunkCopy := c
			if sf != chunk.SampleFormat() {
				chunkCopy = wave.NewFloat32Interleaved(info)
				// convert
				for i := 0; i < c.Size.Len; i++ {
					for j := 0; j < c.Size.Channels; j++ {
						chunkCopy.Set(i, j, chunk.At(i, j))
					}
				}
			}
			if err := binary.Write(buf, HostEndian, chunkCopy.Data); err != nil {
				return err
			}
		default:
			return errors.Errorf("unknown type of audio buffer %T", chunk)
		}

		return server.Send(&pb.ChunksResponse{
			Type: &pb.ChunksResponse_Chunk{
				Chunk: &pb.AudioChunk{
					Data:   outBytes,
					Length: uint32(info.Len),
				},
			},
		})
	}

	for {
		if err := sendNextChunk(); err != nil {
			return err
		}
	}
}

// Properties returns properties of an audio input of the underlying robot.
func (s *serviceServer) Properties(
	ctx context.Context,
	req *pb.PropertiesRequest,
) (*pb.PropertiesResponse, error) {
	audioInput, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	props, err := audioInput.MediaProperties(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.PropertiesResponse{
		ChannelCount:  uint32(props.ChannelCount),
		Latency:       durationpb.New(props.Latency),
		SampleRate:    uint32(props.SampleRate),
		SampleSize:    uint32(props.SampleSize),
		IsBigEndian:   props.IsBigEndian,
		IsFloat:       props.IsFloat,
		IsInterleaved: props.IsInterleaved,
	}, nil
}

// Record renders an audio chunk from an audio input of the underlying robot
// to an HTTP response. A specific MIME type cannot be requested and may not necessarily
// be the same one returned each time.
func (s *serviceServer) Record(
	ctx context.Context,
	req *pb.RecordRequest,
) (*httpbody.HttpBody, error) {
	ctx, span := trace.StartSpan(ctx, "audioinput::server::Record")
	defer span.End()
	audioInput, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	chunkStream, err := audioInput.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.UncheckedError(chunkStream.Close(ctx))
	}()

	firstChunk, release, err := chunkStream.Next(ctx)
	if err != nil {
		return nil, err
	}
	info := firstChunk.ChunkInfo()
	release()

	duration := req.Duration.AsDuration()
	if duration == 0 {
		duration = time.Second
	}
	if duration > 5*time.Second {
		return nil, errors.New("can only record up to 5 seconds")
	}

	ms := memfs.New()
	fd, err := ms.Create("dummy")
	if err != nil {
		return nil, err
	}

	wavEnc := wav.NewEncoder(fd,
		info.SamplingRate,
		24,
		info.Channels,
		1, // PCM
	)

	nextChunk := func() error {
		chunk, release, err := chunkStream.Next(ctx)
		if err != nil {
			return err
		}
		defer release()

		switch c := chunk.(type) {
		case *wave.Int16Interleaved:
			cData := make([]int, len(c.Data))
			for i := 0; i < len(c.Data); i++ {
				cData[i] = int(c.Data[i])
			}
			buf := &audio.IntBuffer{
				Format: &audio.Format{
					NumChannels: info.Channels,
					SampleRate:  info.SamplingRate,
				},
				Data:           cData,
				SourceBitDepth: 16,
			}

			return wavEnc.Write(buf)
		case *wave.Float32Interleaved:
			dataCopy := make([]float32, len(c.Data))
			copy(dataCopy, c.Data)
			buf := &audio.Float32Buffer{
				Format: &audio.Format{
					NumChannels: info.Channels,
					SampleRate:  info.SamplingRate,
				},
				Data:           dataCopy,
				SourceBitDepth: 32,
			}
			if err := transforms.PCMScaleF32(buf, 24); err != nil {
				return err
			}

			return wavEnc.Write(buf.AsIntBuffer())
		default:
			return errors.Errorf("unknown type of audio buffer %T", chunk)
		}
	}
	numChunks := int(duration.Seconds() * float64(info.SamplingRate/info.Len))
	for i := 0; i < numChunks; i++ {
		if err := nextChunk(); err != nil {
			return nil, err
		}
	}

	if err := wavEnc.Close(); err != nil {
		return nil, err
	}
	if _, err := fd.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	rd, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: "audio/wav",
		Data:        rd,
	}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	audioInput, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, audioInput, req)
}
