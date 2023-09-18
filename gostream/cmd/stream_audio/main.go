// Package main streams audio.
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"unsafe"

	"github.com/edaniels/golog"
	"github.com/gen2brain/malgo"
	// register microphone drivers.
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"
	hopus "gopkg.in/hraban/opus.v2"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
)

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 5555
	logger      = golog.Global().Named("server")
)

// Arguments for the command.
type Arguments struct {
	Port     goutils.NetPortFlag `flag:"0"`
	Dump     bool                `flag:"dump"`
	Playback bool                `flag:"playback"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := goutils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Dump {
		all := gostream.QueryAudioDevices()
		for _, info := range all {
			logger.Debugf("%s", info.ID)
			logger.Debugf("\t labels: %v", info.Labels)
			logger.Debugf("\t priority: %v", info.Priority)
			for _, p := range info.Properties {
				logger.Debugf("\t %+v", p.Audio)
			}
		}
		return nil
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = goutils.NetPortFlag(defaultPort)
	}

	return runServer(
		ctx,
		int(argsParsed.Port),
		argsParsed.Playback,
		logger,
	)
}

func runServer(
	ctx context.Context,
	port int,
	playback bool,
	logger golog.Logger,
) (err error) {
	audioSource, err := gostream.GetAnyAudioSource(gostream.DefaultConstraints, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, audioSource.Close(ctx))
	}()

	config := opus.DefaultStreamConfig
	stream, err := gostream.NewStream(config)
	if err != nil {
		return err
	}

	peerCancelations := map[*webrtc.PeerConnection][]func(){}
	var peerCancelationMu sync.Mutex
	var activePlayers sync.WaitGroup
	defer activePlayers.Wait()

	var serverOpts []gostream.StandaloneStreamServerOption
	if playback {
		serverOpts = append(serverOpts, gostream.WithStandaloneAllowReceive(true))

		serverOpts = append(serverOpts, gostream.WithStandaloneOnPeerAdded(func(pc *webrtc.PeerConnection) {
			pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
				cancelCtx, cancelFunc := context.WithCancel(ctx)
				peerCancelationMu.Lock()
				peerCancelations[pc] = append(peerCancelations[pc], cancelFunc)
				peerCancelationMu.Unlock()
				activePlayers.Add(1)
				defer activePlayers.Done()
				decodeAndPlayTrack(cancelCtx, track)
			})
		}))
		serverOpts = append(serverOpts, gostream.WithStandaloneOnPeerRemoved(func(pc *webrtc.PeerConnection) {
			peerCancelationMu.Lock()
			cancelations := peerCancelations[pc]
			peerCancelationMu.Unlock()
			for _, cancel := range cancelations {
				cancel()
			}
		}))
	}
	server, err := gostream.NewStandaloneStreamServer(port, logger, serverOpts, stream)
	if err != nil {
		return err
	}

	if err := server.Start(ctx); err != nil {
		return err
	}

	defer func() { err = multierr.Combine(err, server.Stop(ctx)) }()
	return gostream.StreamAudioSource(ctx, audioSource, stream)
}

func decodeAndPlayTrack(ctx context.Context, track *webrtc.TrackRemote) {
	var hostEndian binary.ByteOrder

	//nolint:gosec
	switch v := *(*uint16)(unsafe.Pointer(&([]byte{0x12, 0x34}[0]))); v {
	case 0x1234:
		hostEndian = binary.BigEndian
	case 0x3412:
		hostEndian = binary.LittleEndian
	default:
		panic(fmt.Sprintf("failed to determine host endianness: %x", v))
	}

	switch track.Kind() {
	case webrtc.RTPCodecTypeAudio:
		{
			mCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
				logger.Debugw("malgo", "msg", message)
			})
			if err != nil {
				panic(err)
			}
			defer func() {
				goutils.UncheckedErrorFunc(mCtx.Uninit)
				mCtx.Free()
			}()

			codec := track.Codec()
			channels := codec.Channels
			sampleRate := codec.ClockRate

			dec, err := hopus.NewDecoder(int(sampleRate), int(channels))
			if err != nil {
				panic(err)
			}

			const maxOpusFrameSizeMs = 60
			maxFrameSize := float32(channels) * maxOpusFrameSizeMs * float32(sampleRate) / 1000
			dataPool := sync.Pool{
				New: func() interface{} {
					newData := make([]float32, int(maxFrameSize))
					return &newData
				},
			}
			decodeRTPData := func() (*[]float32, int, error) {
				data, _, err := track.ReadRTP()
				if err != nil {
					return nil, 0, err
				}
				if len(data.Payload) == 0 {
					return nil, 0, nil
				}

				pcmData := dataPool.Get().(*[]float32)
				n, err := dec.DecodeFloat32(data.Payload, *pcmData)
				pcmActual := (*pcmData)[:n*int(channels)]
				return &pcmActual, n, err
			}

			var periodSizeInFrames int
			for {
				// we assume all packets will contain this amount of samples going forward
				// if it's anything larger than one RTP packet (or close to MTU?) then
				// this could fail.
				_, numSamples, err := decodeRTPData()
				if err != nil {
					panic(err)
				}
				if numSamples > 0 {
					periodSizeInFrames = numSamples
					break
				}
			}

			deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
			deviceConfig.Playback.Format = malgo.FormatF32 // tied to what we opus decode to
			deviceConfig.Playback.Channels = uint32(channels)
			deviceConfig.SampleRate = sampleRate
			deviceConfig.PeriodSizeInFrames = uint32(periodSizeInFrames)
			sizeInBytes := malgo.SampleSizeInBytes(deviceConfig.Playback.Format)

			pcmChan := make(chan *[]float32)

			onSendFrames := func(pOutput, _ []byte, frameCount uint32) {
				if ctx.Err() != nil {
					return
				}
				samplesRequested := frameCount * deviceConfig.Playback.Channels * uint32(sizeInBytes)
				select {
				case <-ctx.Done():
					return
				case pcm := <-pcmChan:
					pcmToWrite := *pcm
					if len(pcmToWrite) > int(samplesRequested) {
						logger.Errorw("not enough samples requested; trimming our own data", "samples_requested", samplesRequested)
						pcmToWrite = pcmToWrite[:samplesRequested]
					}
					pOutput = pOutput[:0]
					buf := bytes.NewBuffer(pOutput)
					if err := binary.Write(buf, hostEndian, pcmToWrite); err != nil {
						logger.Errorw("error writing to pcm buf", "error", err)
					}
					dataPool.Put(pcm)
				}
			}

			playbackCallbacks := malgo.DeviceCallbacks{
				Data: onSendFrames,
			}

			device, err := malgo.InitDevice(mCtx.Context, deviceConfig, playbackCallbacks)
			if err != nil {
				panic(err)
			}

			err = device.Start()
			if err != nil {
				panic(err)
			}

			defer device.Uninit()

			for {
				if ctx.Err() != nil {
					return
				}
				pcmData, numSamples, err := decodeRTPData()
				if errors.Is(err, io.EOF) {
					return
				}
				if numSamples == 0 {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case pcmChan <- pcmData:
				}
			}
		}
	case webrtc.RTPCodecTypeVideo:
		fallthrough
	default:
		panic(errors.Errorf("Unsupported track kind %v", track.Kind()))
	}
}
