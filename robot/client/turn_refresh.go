package client

import (
	"context"
	"strconv"
	"time"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils"
)

// A connection relayed through a TURN server holds a relay allocation whose time-limited
// credentials (the username is the unix expiry timestamp, per the TURN REST convention) eventually
// expire; once they do the relay path drops. There is no way to swap credentials on a live
// allocation, so we refresh by re-dialing — which fetches fresh credentials via the signaling
// server — shortly before they lapse.
//
// We only do this when the connection is actually using a relay (its selected ICE candidate pair is
// a relay). A connection running over a host/srflx pair may keep a background relay allocation
// alive, but that allocation's expiry is harmless and re-dialing it would be wasteful churn.

const (
	// turnReconnectLeadFactor is how early, as a fraction of the credential's remaining lifetime, to
	// re-dial. 0.5 == halfway to expiry.
	turnReconnectLeadFactor = 0.5
	// turnReconnectMinWait avoids busy-looping on already-expired or near-expired credentials.
	turnReconnectMinWait = time.Minute
	// turnReconnectMaxWait bounds the pre-expiry re-dial schedule.
	turnReconnectMaxWait = 24 * time.Hour
	// turnReconnectRecheckWait is how often to re-evaluate a connection that isn't currently using a
	// relay, so we notice if ICE later settles on (or fails over to) one.
	turnReconnectRecheckWait = time.Hour
)

// relaySelected reports whether the connection's currently selected ICE candidate pair uses a relay
// (TURN) candidate at either end. Credentials only need refreshing when the relay is the active
// path, but the relay may belong to either peer: our own local candidate, or the remote candidate
// (e.g. an answerer/robot behind a proxy that gathered a relay). A full re-dial refreshes both,
// since the answerer re-fetches its credentials when it answers the new connection.
func relaySelected(peerConn *webrtc.PeerConnection) bool {
	if peerConn == nil || peerConn.ICEConnectionState() != webrtc.ICEConnectionStateConnected {
		return false
	}
	sctp := peerConn.SCTP()
	if sctp == nil || sctp.Transport() == nil || sctp.Transport().ICETransport() == nil {
		return false
	}
	pair, err := sctp.Transport().ICETransport().GetSelectedCandidatePair()
	if err != nil || pair == nil {
		return false
	}
	return (pair.Local != nil && pair.Local.Typ == webrtc.ICECandidateTypeRelay) ||
		(pair.Remote != nil && pair.Remote.Typ == webrtc.ICECandidateTypeRelay)
}

// earliestTURNCredentialExpiry returns the soonest expiry encoded in any of the connection's TURN
// credential usernames. ok is false when there is no time-limited TURN credential (e.g. a
// non-timestamp username scheme), in which case there is nothing to refresh.
func earliestTURNCredentialExpiry(peerConn *webrtc.PeerConnection) (time.Time, bool) {
	if peerConn == nil {
		return time.Time{}, false
	}
	var earliest time.Time
	found := false
	for _, server := range peerConn.GetConfiguration().ICEServers {
		if server.Username == "" {
			continue
		}
		secs, err := strconv.ParseInt(server.Username, 10, 64)
		if err != nil {
			continue
		}
		expiry := time.Unix(secs, 0)
		if !found || expiry.Before(earliest) {
			earliest, found = expiry, true
		}
	}
	return earliest, found
}

// nextTURNReconnectWait computes how long to wait before the next proactive re-dial. ok is true only
// when the connection is actively relayed through a TURN server with a time-limited credential — the
// only case that warrants re-dialing. Otherwise it returns a recheck interval with ok=false.
func (rc *RobotClient) nextTURNReconnectWait() (time.Duration, bool) {
	peerConn := rc.conn.PeerConn()
	if !relaySelected(peerConn) {
		return turnReconnectRecheckWait, false
	}
	// We schedule off our own credential expiry even when the relay belongs to the remote peer (we
	// can't see the peer's expiry). Both peers fetch from the same signaling server with the same
	// TTL within seconds of each other, so this is a close proxy; if it were ever badly skewed, the
	// reactive checkConnection reconnect still catches a drop.
	expiry, ok := earliestTURNCredentialExpiry(peerConn)
	if !ok {
		return turnReconnectRecheckWait, false
	}
	remaining := time.Until(expiry)
	if remaining <= 0 {
		return turnReconnectMinWait, true
	}
	wait := time.Duration(float64(remaining) * turnReconnectLeadFactor)
	if wait < turnReconnectMinWait {
		wait = turnReconnectMinWait
	}
	if wait > turnReconnectMaxWait {
		wait = turnReconnectMaxWait
	}
	return wait, true
}

// maintainTURNCredentials re-dials the connection before its time-limited TURN credentials expire,
// but only while the connection is actually using a relay. Re-dialing fetches fresh credentials from
// the signaling server, so the new connection's relay allocation is valid again. For connections not
// using a relay it does no work beyond a periodic check of the selected candidate pair, so callers
// need not know in advance whether they will connect over a relay.
func (rc *RobotClient) maintainTURNCredentials(ctx context.Context) {
	for {
		wait, relayed := rc.nextTURNReconnectWait()
		if !utils.SelectContextOrWait(ctx, wait) {
			return
		}
		// Re-check after sleeping: a long wait may have crossed a (dis)connect or a change in the
		// selected pair. Only re-dial if the connection is up and still relayed.
		if !relayed || !rc.connected.Load() || !relaySelected(rc.conn.PeerConn()) {
			continue
		}
		rc.Logger().CInfow(ctx,
			"proactively re-dialing relayed connection to refresh TURN credentials before they expire",
			"address", rc.address)
		if err := rc.Connect(ctx); err != nil {
			rc.Logger().CErrorw(ctx, "proactive TURN credential re-dial failed", "error", err, "address", rc.address)
		}
	}
}
