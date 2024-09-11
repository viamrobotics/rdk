package grpc

// Tracker allows callback functions to a WebRTC peer connection's OnTrack callback
// function by track name.
// Both grpc.SharedConn and grpc.ReconfigurableClientConn implement tracker.
type Tracker interface {
	AddOnTrackSub(trackName string, onTrackCB OnTrackCB)
	RemoveOnTrackSub(trackName string)
}
