package module

import (
	"context"
	"errors"
	"fmt"

	"github.com/pion/rtp"
	"github.com/viamrobotics/webrtc/v3"
	"go.uber.org/multierr"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"golang.org/x/exp/maps"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/module/trace"
	"go.viam.com/rdk/resource"
)

// errMaxSupportedWebRTCTrackLimit is the error returned when the MaxSupportedWebRTCTRacks limit is
// reached.
var errMaxSupportedWebRTCTrackLimit = fmt.Errorf("only %d WebRTC tracks are supported per peer connection",
	maxSupportedWebRTCTRacks)

type peerResourceState struct {
	// NOTE As I'm only suppporting video to start this will always be a single element
	// once we add audio we will need to make this a slice / map
	subID rtppassthrough.SubscriptionID
}

// ListStreams lists the streams.
func (m *Module) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	_, span := trace.StartSpan(ctx, "module::module::ListStreams")
	defer span.End()
	names := make([]string, 0, len(m.streamSourceByName))
	for _, n := range maps.Keys(m.streamSourceByName) {
		names = append(names, n.String())
	}
	return &streampb.ListStreamsResponse{Names: names}, nil
}

// AddStream adds a stream.
// Returns an error if:
// 1. there is no WebRTC peer connection with viam-sever
// 2. resource doesn't exist
// 3. the resource doesn't implement rtppassthrough.Source,
// 4. there are already the max number of supported tracks on the peer connection
// 5. SubscribeRTP returns an error
// 6. A webrtc track is unable to be created
// 7. Adding the track to the peer connection fails.
func (m *Module) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "module::module::AddStream")
	defer span.End()
	name, err := resource.NewFromString(req.GetName())
	if err != nil {
		return nil, err
	}
	m.registerMu.Lock()
	defer m.registerMu.Unlock()
	if m.pc == nil {
		return nil, errors.New("module has no peer connection")
	}
	vcss, ok := m.streamSourceByName[name]
	if !ok {
		err := errors.New("unknown stream for resource")
		m.logger.CWarnw(ctx, err.Error(), "name", name.String(), "streamSourceByName", fmt.Sprintf("%#v", m.streamSourceByName))
		return nil, err
	}

	if _, ok = m.activeResourceStreams[name]; ok {
		m.logger.CWarnw(ctx, "AddStream called with when there is already a stream for peer connection. NoOp", "name", name)
		return &streampb.AddStreamResponse{}, nil
	}

	if len(m.activeResourceStreams) >= maxSupportedWebRTCTRacks {
		return nil, errMaxSupportedWebRTCTrackLimit
	}

	tlsRTP, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/H264"}, "video", name.String())
	if err != nil {
		return nil, fmt.Errorf("error creating a new TrackLocalStaticRTP: %w", err)
	}

	sub, err := vcss.SubscribeRTP(ctx, rtpBufferSize, func(pkts []*rtp.Packet) {
		for _, pkt := range pkts {
			if err := tlsRTP.WriteRTP(pkt); err != nil {
				m.logger.CWarnw(ctx, "SubscribeRTP callback function WriteRTP", "err", err)
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("error setting up stream subscription: %w", err)
	}

	m.logger.CDebugw(ctx, "AddStream calling AddTrack", "name", name.String(), "subID", sub.ID.String())
	sender, err := m.pc.AddTrack(tlsRTP)
	if err != nil {
		err = fmt.Errorf("error adding track: %w", err)
		if unsubErr := vcss.Unsubscribe(ctx, sub.ID); unsubErr != nil {
			return nil, multierr.Combine(err, unsubErr)
		}
		return nil, err
	}

	removeTrackOnSubTerminate := func() {
		defer m.logger.Debugw("RemoveTrack called on ", "name", name.String(), "subID", sub.ID.String())
		// wait until either the module is shutting down, or the subscription terminates
		var msg string
		select {
		case <-sub.Terminated.Done():
			msg = "rtp_passthrough subscription expired, calling RemoveTrack"
		case <-m.shutdownCtx.Done():
			msg = "module closing calling RemoveTrack"
		}
		// remove the track from the peer connection so that viam-server clients know that the stream has terminated
		m.registerMu.Lock()
		defer m.registerMu.Unlock()
		m.logger.Debugw(msg, "name", name.String(), "subID", sub.ID.String())
		delete(m.activeResourceStreams, name)
		if err := m.pc.RemoveTrack(sender); err != nil {
			m.logger.Warnf("RemoveTrack returned error", "name", name.String(), "subID", sub.ID.String(), "err", err)
		}
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(removeTrackOnSubTerminate, m.activeBackgroundWorkers.Done)

	m.activeResourceStreams[name] = peerResourceState{subID: sub.ID}
	return &streampb.AddStreamResponse{}, nil
}

// RemoveStream removes a stream.
func (m *Module) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "module::module::RemoveStream")
	defer span.End()
	name, err := resource.NewFromString(req.GetName())
	if err != nil {
		return nil, err
	}
	m.registerMu.Lock()
	defer m.registerMu.Unlock()
	if m.pc == nil {
		return nil, errors.New("module has no peer connection")
	}
	vcss, ok := m.streamSourceByName[name]
	if !ok {
		return nil, fmt.Errorf("unknown stream for resource %s", name)
	}

	prs, ok := m.activeResourceStreams[name]
	if !ok {
		return nil, fmt.Errorf("stream %s is not active", name)
	}

	if err := vcss.Unsubscribe(ctx, prs.subID); err != nil {
		m.logger.CWarnw(ctx, "RemoveStream > Unsubscribe", "name", name.String(), "subID", prs.subID.String(), "err", err)
		return nil, err
	}

	delete(m.activeResourceStreams, name)
	return &streampb.RemoveStreamResponse{}, nil
}
