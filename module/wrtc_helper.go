package module

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/pion/sctp"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils"
)

// ConfigureForRenegotiation sets up PeerConnection callbacks for updating local descriptions and
// sending offers when a negotiation is needed (e.g: adding a video track). As well as listening for
// offers/answers to update remote descriptions (e.g: when the peer adds a video track).
//
// If successful, a Go channel is returned. The Go channel will close when the negotiation
// DataChannel is open and available for renegotiation.
func ConfigureForRenegotiation(peerConn *webrtc.PeerConnection, logger logging.Logger) (<-chan struct{}, error) {
	var negMu sync.Mutex

	// All of Viam's PeerConnections hard code the `data` channel to be ID 0 and the `negotiation`
	// channel to be ID 1. Thus these channels are "pre-negotiated".
	negotiated := true
	// Packets over this channel must be processed in order (à la TCP).
	ordered := true
	negotiationChannelID := uint16(1)
	negotiationChannel, err := peerConn.CreateDataChannel("negotiation", &webrtc.DataChannelInit{
		ID:         &negotiationChannelID,
		Negotiated: &negotiated,
		Ordered:    &ordered,
	})
	if err != nil {
		return nil, err
	}

	negotiationChannel.OnError(initialDataChannelOnError(peerConn, logger))

	// The pion webrtc library may invoke `OnNegotiationNeeded` prior to the connection being
	// established. We drop those requests on the floor. The original connection is established with
	// our signaling and answering machinery.
	//
	// Additionally, just because a PeerConnection has moved into the `connected` state, that does
	// not imply the pre-negotiated `negotiation` DataChannel is available for use. We return this
	// `negOpened` channel to let tests create a happens-before relationship. Such that these tests
	// can know when a PeerConnection method that invokes `OnNegotiationNeeded` can utilize this
	// negotiation channel.
	negOpened := make(chan struct{})
	negotiationChannel.OnOpen(func() {
		close(negOpened)
	})

	// OnNegotiationNeeded is webrtc callback for when a PeerConnection is mutated in a way such
	// that its local description should change. Such as when a video track is added that should be
	// streamed to the peer.
	peerConn.OnNegotiationNeeded(func() {
		select {
		case <-negOpened:
		default:
			// Negotiation cannot occur over the negotiation channel until after the channel is in
			// operation.
			return
		}

		negMu.Lock()
		defer negMu.Unlock()
		// Creating an offer will generate the desired local description that includes the
		// modifications responsible for entering the callback. Such as adding a video track.
		offer, err := peerConn.CreateOffer(nil)
		if err != nil {
			logger.Errorw("renegotiation: error creating offer", "error", err)
			return
		}

		// Dan: It's not clear to me why an offer is created from a `PeerConnection` just to call
		// `PeerConnection.SetLocalDescription`. And then when encoding the `Description` ("SDP")
		// for sending to the peer, we must call `PeerConnection.LocalDescription` rather than using
		// the `offer`. But it's easy to see that the `offer` and `peerConn.LocalDescription()` are
		// different (e.g: the latter includes ICE candidates), so it must be done this way.
		if err := peerConn.SetLocalDescription(offer); err != nil {
			logger.Errorw("renegotiation: error setting local description", "error", err)
			return
		}

		// Encode and send the new local description to the peer over the `negotiation` channel. The
		// peer will respond over the negotiation channel with an answer. That answer will be used to
		// update the remote description.
		encodedSDP, err := EncodeSDP(peerConn.LocalDescription())
		if err != nil {
			logger.Errorw("renegotiation: error encoding SDP", "error", err)
			return
		}
		if err := negotiationChannel.SendText(encodedSDP); err != nil {
			logger.Errorw("renegotiation: error sending SDP", "error", err)
			return
		}
	})

	negotiationChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		negMu.Lock()
		defer negMu.Unlock()

		description := webrtc.SessionDescription{}
		if err := DecodeSDP(string(msg.Data), &description); err != nil {
			logger.Errorw("renegotiation: error decoding SDP", "error", err)
			return
		}

		// A new description was received over the negotiation channel. Use that to update the remote
		// description.
		if err := peerConn.SetRemoteDescription(description); err != nil {
			logger.Errorw("renegotiation: error setting remote description", "error", err)
			return
		}

		// If the message was an offer, generate an answer, set it as the local description and
		// respond. Such that the peer can update its remote description.
		//
		// If the incoming message was an answer, the peers are now in sync and no further messages
		// are required.
		if description.Type != webrtc.SDPTypeOffer {
			return
		}

		// Dan: It's unclear to me how error handling should happen here. Receiving an offer implies
		// the peer's local description is not in sync with our remote description for that
		// peer. I'm unsure of the long-term consequence of a pair of PeerConnections being in this
		// inconsistent state.
		answer, err := peerConn.CreateAnswer(nil)
		if err != nil {
			logger.Errorw("renegotiation: error creating answer", "error", err)
			return
		}
		if err := peerConn.SetLocalDescription(answer); err != nil {
			logger.Errorw("renegotiation: error setting local description", "error", err)
			return
		}

		encodedSDP, err := EncodeSDP(peerConn.LocalDescription())
		if err != nil {
			logger.Errorw("renegotiation: error encoding SDP", "error", err)
			return
		}
		if err := negotiationChannel.SendText(encodedSDP); err != nil {
			logger.Errorw("renegotiation: error sending SDP", "error", err)
			return
		}
	})

	return negOpened, nil
}

func initialDataChannelOnError(pc io.Closer, logger logging.Logger) func(err error) {
	return func(err error) {
		if errors.Is(err, sctp.ErrResetPacketInStateNotExist) ||
			isUserInitiatedAbortChunkErr(err) {
			return
		}
		logger.Errorw("premature data channel error before WebRTC channel association", "error", err)
		utils.UncheckedError(pc.Close())
	}
}

// EncodeSDP encodes the given SDP in base64.
func EncodeSDP(sdp *webrtc.SessionDescription) (string, error) {
	b, err := json.Marshal(sdp)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeSDP decodes the input from base64 into the given SDP.
func DecodeSDP(in string, sdp *webrtc.SessionDescription) error {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, sdp)
	if err != nil {
		return err
	}
	return err
}

func isUserInitiatedAbortChunkErr(err error) bool {
	return err != nil && errors.Is(err, sctp.ErrChunk) &&
		strings.Contains(err.Error(), "User Initiated Abort: Close called")
}
