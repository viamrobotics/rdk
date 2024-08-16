package gostream

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/viamrobotics/webrtc/v3"
	"go.uber.org/multierr"
	"go.viam.com/rdk/logging"
)

// Adapted from https://github.com/pion/webrtc/blob/master/track_local_static.go
// TODO(https://github.com/edaniels/gostream/issues/4): go through these comments
// and write them in your own words so that it's consistent and you understand
// what's going on here.

// trackBinding is a single bind for a Track
// Bind can be called multiple times, this stores the
// result for a single bind call so that it can be used when writing.
type trackBinding struct {
	id          string
	ssrc        webrtc.SSRC
	payloadType webrtc.PayloadType
	writeStream webrtc.TrackLocalWriter
}

// trackLocalStaticRTP  is a TrackLocal that has a pre-set codec and accepts RTP Packets.
// If you wish to send a media.Sample use trackLocalStaticSample.
type trackLocalStaticRTP struct {
	mu                sync.RWMutex
	bindings          []trackBinding
	codec             webrtc.RTPCodecCapability
	id, rid, streamID string

	// testing
	sequenceNumberOffset  func(uint16) uint16
	streamSourceChanged   atomic.Bool
	highestSequenceNumber uint16
}

// newtrackLocalStaticRTP returns a trackLocalStaticRTP.
func newtrackLocalStaticRTP(c webrtc.RTPCodecCapability, id, streamID string) *trackLocalStaticRTP {
	return &trackLocalStaticRTP{
		sequenceNumberOffset: func(u uint16) uint16 { return u },
		codec:                c,
		bindings:             []trackBinding{},
		id:                   id,
		streamID:             streamID,
	}
}

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call.
func (s *trackLocalStaticRTP) Bind(t webrtc.TrackLocalContext) (webrtc.RTPCodecParameters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parameters := webrtc.RTPCodecParameters{RTPCodecCapability: s.codec}
	if codec, err := codecParametersFuzzySearch(parameters, t.CodecParameters()); err == nil {
		s.bindings = append(s.bindings, trackBinding{
			ssrc:        t.SSRC(),
			payloadType: codec.PayloadType,
			writeStream: t.WriteStream(),
			id:          t.ID(),
		})
		return codec, nil
	}

	return webrtc.RTPCodecParameters{}, webrtc.ErrUnsupportedCodec
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *trackLocalStaticRTP) Unbind(t webrtc.TrackLocalContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bindings {
		if s.bindings[i].id == t.ID() {
			s.bindings[i] = s.bindings[len(s.bindings)-1]
			s.bindings = s.bindings[:len(s.bindings)-1]
			return nil
		}
	}

	return webrtc.ErrUnbindFailed
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'.
func (s *trackLocalStaticRTP) ID() string { return s.id }

// RID is the RTP stream identifier.
func (s *trackLocalStaticRTP) RID() string { return s.rid }

// StreamID is the group this track belongs too. This must be unique.
func (s *trackLocalStaticRTP) StreamID() string { return s.streamID }

// Kind controls if this TrackLocal is audio or video.
func (s *trackLocalStaticRTP) Kind() webrtc.RTPCodecType {
	switch {
	case strings.HasPrefix(s.codec.MimeType, "audio/"):
		return webrtc.RTPCodecTypeAudio
	case strings.HasPrefix(s.codec.MimeType, "video/"):
		return webrtc.RTPCodecTypeVideo
	default:
		return webrtc.RTPCodecType(0)
	}
}

// Codec gets the Codec of the track.
func (s *trackLocalStaticRTP) Codec() webrtc.RTPCodecCapability {
	return s.codec
}

func sequenceNumberOffset(lastMaxPacketSequenceNumber uint16, firstNewPacketSequenceNumber uint16) func(uint16) uint16 {
	if lastMaxPacketSequenceNumber > firstNewPacketSequenceNumber {
		logging.Global().Warnf("new sequenceNumberGenerator which is not identity lastMaxPacketSequenceNumber(%d),  firstNewPacketSequenceNumber(%d)", lastMaxPacketSequenceNumber, firstNewPacketSequenceNumber)
		// continue with prev
		return func(sequenceNumber uint16) uint16 {
			return lastMaxPacketSequenceNumber + 1 + sequenceNumber - firstNewPacketSequenceNumber
		}

	}
	return func(u uint16) uint16 { return u }
}

func (s *trackLocalStaticRTP) StreamSourceChanged() {
	s.streamSourceChanged.Store(true)
}

func wrapped(currentSequenceNumber uint16, hightestSequenceNumber uint16) bool {
	// if the current sequence number is smaller than the currentHighestSequenceNumber by more
	// than half the u16, assume that the sequence number has wrapped
	if currentSequenceNumber > hightestSequenceNumber {
		return false
	}
	return hightestSequenceNumber-currentSequenceNumber > math.MaxUint16/2
}

func (s *trackLocalStaticRTP) WriteRTP(p *rtp.Packet) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writeErrs := []error{}
	outboundPacket := *p

	for _, b := range s.bindings {
		outboundPacket.Header.SSRC = uint32(b.ssrc)
		outboundPacket.Header.PayloadType = uint8(b.payloadType)
		if _, err := b.writeStream.WriteRTP(&outboundPacket.Header, outboundPacket.Payload); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return multierr.Combine(writeErrs...)
}

// WriteRTP writes a RTP Packet to the trackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them.
func (s *trackLocalStaticRTP) WriteRTPModified(p *rtp.Packet) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	originalSequenceNumber := p.Header.SequenceNumber

	// create new generator if the stream has changed needed
	if s.streamSourceChanged.CompareAndSwap(true, false) {
		// TODO: Rollover
		s.sequenceNumberOffset = sequenceNumberOffset(s.highestSequenceNumber, p.Header.SequenceNumber)
	}

	// Update the header's sequence number to
	p.Header.SequenceNumber = s.sequenceNumberOffset(p.Header.SequenceNumber)

	// set the currentHighestSequenceNumber to the current packet's sequence number
	// if the packet's sequence number is greater than the current highest sequence number
	// or if the sequence number wrapped
	setHighest := false
	if p.Header.SequenceNumber > s.highestSequenceNumber {
		setHighest = true
	}

	if wrapped(p.Header.SequenceNumber, s.highestSequenceNumber) {
		logging.Global().Warnf("updating highestSequenceNumber to %d due to wrapping, prevhighestSequenceNumber: %d, originalSN: %d", p.Header.SequenceNumber, s.highestSequenceNumber, originalSequenceNumber)
		setHighest = true
	}

	if setHighest {
		s.highestSequenceNumber = p.Header.SequenceNumber
	} else {
		logging.Global().Warnf("publishing out of order message p.Header.SequenceNumber(%d), originalSN: %d, highestSequenceNumber: %d", p.Header.SequenceNumber, originalSequenceNumber, s.highestSequenceNumber)
	}
	logging.Global().Debugf("SN: %d", p.Header.SequenceNumber)

	writeErrs := []error{}
	outboundPacket := *p

	for _, b := range s.bindings {
		outboundPacket.Header.SSRC = uint32(b.ssrc)
		outboundPacket.Header.PayloadType = uint8(b.payloadType)
		if _, err := b.writeStream.WriteRTP(&outboundPacket.Header, outboundPacket.Payload); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return multierr.Combine(writeErrs...)
}

// Write writes a RTP Packet as a buffer to the trackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them.
func (s *trackLocalStaticRTP) Write(b []byte) (n int, err error) {
	packet := &rtp.Packet{}
	if err = packet.Unmarshal(b); err != nil {
		return 0, err
	}

	return len(b), s.WriteRTP(packet)
}

// trackLocalStaticSample is a TrackLocal that has a pre-set codec and accepts Samples.
// If you wish to send a RTP Packet use trackLocalStaticRTP.
type trackLocalStaticSample struct {
	packetizer   rtp.Packetizer
	rtpTrack     *trackLocalStaticRTP
	sampler      samplerFunc
	isAudio      bool
	clockRate    uint32
	audioLatency time.Duration
}

// newVideoTrackLocalStaticSample returns a trackLocalStaticSample for video.
func newVideoTrackLocalStaticSample(c webrtc.RTPCodecCapability, id, streamID string) *trackLocalStaticSample {
	return &trackLocalStaticSample{
		rtpTrack: newtrackLocalStaticRTP(c, id, streamID),
	}
}

// newAudioTrackLocalStaticSample returns a trackLocalStaticSample for audio.
func newAudioTrackLocalStaticSample(
	c webrtc.RTPCodecCapability,
	id, streamID string,
) *trackLocalStaticSample {
	return &trackLocalStaticSample{
		rtpTrack: newtrackLocalStaticRTP(c, id, streamID),
		isAudio:  true,
	}
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'.
func (s *trackLocalStaticSample) ID() string { return s.rtpTrack.ID() }

// StreamID is the group this track belongs too. This must be unique.
func (s *trackLocalStaticSample) StreamID() string { return s.rtpTrack.StreamID() }

// RID is the RTP stream identifier.
func (s *trackLocalStaticSample) RID() string { return s.rtpTrack.RID() }

// Kind controls if this TrackLocal is audio or video.
func (s *trackLocalStaticSample) Kind() webrtc.RTPCodecType { return s.rtpTrack.Kind() }

// Codec gets the Codec of the track.
func (s *trackLocalStaticSample) Codec() webrtc.RTPCodecCapability {
	return s.rtpTrack.Codec()
}

const rtpOutboundMTU = 1200

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call.
func (s *trackLocalStaticSample) Bind(t webrtc.TrackLocalContext) (webrtc.RTPCodecParameters, error) {
	codec, err := s.rtpTrack.Bind(t)
	if err != nil {
		return codec, err
	}

	s.rtpTrack.mu.Lock()
	defer s.rtpTrack.mu.Unlock()

	// We only need one packetizer. But isn't that confusing with other clock rates
	// from other codecs?
	if s.packetizer != nil {
		return codec, nil
	}

	payloader, err := payloaderForCodec(codec.RTPCodecCapability)
	if err != nil {
		return codec, err
	}

	// TODO(erd): I think we need to do this for each bind
	s.packetizer = rtp.NewPacketizer(
		rtpOutboundMTU,
		uint8(codec.PayloadType),
		uint32(t.SSRC()),
		payloader,
		rtp.NewRandomSequencer(),
		codec.ClockRate,
	)

	s.clockRate = codec.RTPCodecCapability.ClockRate
	return codec, nil
}

func (s *trackLocalStaticSample) setAudioLatency(latency time.Duration) {
	s.rtpTrack.mu.Lock()
	defer s.rtpTrack.mu.Unlock()
	s.audioLatency = latency
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *trackLocalStaticSample) Unbind(t webrtc.TrackLocalContext) error {
	return s.rtpTrack.Unbind(t)
}

// WriteData writes already encoded data to the trackLocalStaticSample
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them.
func (s *trackLocalStaticSample) WriteData(frame []byte) error {
	s.rtpTrack.mu.Lock()
	p := s.packetizer
	if p == nil {
		s.rtpTrack.mu.Unlock()
		return nil
	}
	if s.isAudio && s.audioLatency == 0 {
		return nil
	}
	sampler := s.sampler
	if sampler == nil {
		if s.isAudio {
			s.sampler = newAudioSampler(s.clockRate, s.audioLatency)
		} else {
			s.sampler = newVideoSampler(s.clockRate)
		}
	}

	s.rtpTrack.mu.Unlock()

	if s.sampler == nil {
		return nil
	}
	samples := s.sampler()
	packets := p.Packetize(frame, samples)

	writeErrs := []error{}
	for _, p := range packets {
		if err := s.rtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return multierr.Combine(writeErrs...)
}

// Do a fuzzy find for a codec in the list of codecs
// Used for lookup up a codec in an existing list to find a match.
func codecParametersFuzzySearch(needle webrtc.RTPCodecParameters, haystack []webrtc.RTPCodecParameters) (webrtc.RTPCodecParameters, error) {
	// First attempt to match on MimeType + SDPFmtpLine
	for _, c := range haystack {
		if strings.EqualFold(c.RTPCodecCapability.MimeType, needle.RTPCodecCapability.MimeType) &&
			c.RTPCodecCapability.SDPFmtpLine == needle.RTPCodecCapability.SDPFmtpLine {
			return c, nil
		}
	}

	// Fallback to just MimeType
	for _, c := range haystack {
		if strings.EqualFold(c.RTPCodecCapability.MimeType, needle.RTPCodecCapability.MimeType) {
			return c, nil
		}
	}

	return webrtc.RTPCodecParameters{}, webrtc.ErrCodecNotFound
}

func payloaderForCodec(codec webrtc.RTPCodecCapability) (rtp.Payloader, error) {
	switch strings.ToLower(codec.MimeType) {
	case strings.ToLower(webrtc.MimeTypeH264):
		return &codecs.H264Payloader{}, nil
	case strings.ToLower(webrtc.MimeTypeOpus):
		return &codecs.OpusPayloader{}, nil
	case strings.ToLower(webrtc.MimeTypeVP8):
		return &codecs.VP8Payloader{}, nil
	case strings.ToLower(webrtc.MimeTypeVP9):
		return &codecs.VP9Payloader{}, nil
	case strings.ToLower(webrtc.MimeTypeG722):
		return &codecs.G722Payloader{}, nil
	case strings.ToLower(webrtc.MimeTypePCMU), strings.ToLower(webrtc.MimeTypePCMA):
		return &codecs.G711Payloader{}, nil
	default:
		return nil, webrtc.ErrNoPayloaderForCodec
	}
}

type samplerFunc func() uint32

// newVideoSampler creates a video sampler that uses the actual video frame rate and
// the codec's clock rate to come up with a duration for each sample.
func newVideoSampler(clockRate uint32) samplerFunc {
	clockRateFloat := float64(clockRate)
	lastTimestamp := time.Now()

	return samplerFunc(func() uint32 {
		now := time.Now()
		duration := now.Sub(lastTimestamp).Seconds()
		samples := uint32(math.Round(clockRateFloat * duration))
		lastTimestamp = now
		return samples
	})
}

// newAudioSampler creates a audio sampler that uses a fixed latency and
// the codec's clock rate to come up with a duration for each sample.
func newAudioSampler(clockRate uint32, latency time.Duration) samplerFunc {
	samples := uint32(math.Round(float64(clockRate) * latency.Seconds()))
	return samplerFunc(func() uint32 {
		return samples
	})
}
