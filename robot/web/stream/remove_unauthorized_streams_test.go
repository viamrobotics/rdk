package webstream

import (
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/test"

	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/web/stream/state"
)

// TestRemoveUnauthorizedStreams verifies the video-stream side of a user_permissions
// change: an active stream whose subscribing user has lost AddStream access is torn down
// (removed from the server's bookkeeping and detached from the peer connection), while a
// stream whose user still has access is left completely untouched.
func TestRemoveUnauthorizedStreams(t *testing.T) {
	logger := logging.NewTestLogger(t)

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, pc.Close(), test.ShouldBeNil) }()

	// addTrack attaches a video track to the peer connection and returns its sender, so
	// we can later assert whether the track was detached.
	addTrack := func(streamID string) *webrtc.RTPSender {
		track, err := webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", streamID)
		test.That(t, err, test.ShouldBeNil)
		sender, err := pc.AddTrack(track)
		test.That(t, err, test.ShouldBeNil)
		return sender
	}

	// A closed StreamState's Decrement returns immediately without engaging the stream
	// event loop, so RemoveUnauthorizedStreams never blocks in the test.
	newClosedState := func() *state.StreamState {
		ss := state.New(nil, nil, logger)
		test.That(t, ss.Close(), test.ShouldBeNil)
		return ss
	}

	keepID := rdkgrpc.Identity{Entity: "keeps-access"}
	dropID := rdkgrpc.Identity{Entity: "loses-access"}
	keepSender, dropSender := addTrack("cam-keep"), addTrack("cam-drop")

	server := &Server{
		logger: logger,
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{
			pc: {
				"cam-keep": {streamState: newClosedState(), senders: []*webrtc.RTPSender{keepSender}, authIdentity: keepID},
				"cam-drop": {streamState: newClosedState(), senders: []*webrtc.RTPSender{dropSender}, authIdentity: dropID},
			},
		},
	}

	// Simulate a user_permissions change under which only keepID still has access.
	server.RemoveUnauthorizedStreams(func(id rdkgrpc.Identity, _ string) bool {
		return id == keepID
	})

	remaining := server.activePeerStreams[pc]

	// The retained user's stream is left in place, track still attached.
	_, keptTracked := remaining["cam-keep"]
	test.That(t, keptTracked, test.ShouldBeTrue)
	test.That(t, keepSender.Track(), test.ShouldNotBeNil)

	// The unauthorized user's stream is removed from bookkeeping and its track detached.
	_, dropStillTracked := remaining["cam-drop"]
	test.That(t, dropStillTracked, test.ShouldBeFalse)
	test.That(t, dropSender.Track(), test.ShouldBeNil)
}
