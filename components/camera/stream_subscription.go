package camera

// StreamSubscription executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type StreamSubscription struct{}

// NewVideoCodecStreamSubscription allocates a VideoCodecStreamSubscription.
func NewVideoCodecStreamSubscription(queueSize int) (*StreamSubscription, error) {
	return &StreamSubscription{}, nil
}
