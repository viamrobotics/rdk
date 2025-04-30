package webstream

import (
	"sync"
	"testing"

	"github.com/viamrobotics/webrtc/v3"

	"go.viam.com/rdk/logging"
)

var (
	logLeakStart = make(chan byte)
	logLeakWG    sync.WaitGroup
)

func TestMain(m *testing.M) {
	// Run all tests
	m.Run()
	// Release any goroutines waiting to run after their surrounding
	// tests have completed.
	close(logLeakStart)
	// Wait for all pending goroutines to finish so the test program
	// doesn't miss a race due to shutting down early.
	logLeakWG.Wait()
}

func TestRemoveStreamsOnPCDisconnectRace(t *testing.T) {
	// If this test is failing it's probably because a change to removeStreamsOnPCDisconnect
	// has it using the logger or other data on the server struct without locking server.mu
	// and checking server.isAlive.
	// This test can be safely removed if webrtc changes to properly wait on its goroutines
	// during shutdown.
	logger := logging.NewTestLogger(t).Sublogger("TestRemoveStreamsOnPCDisconnect")
	server := &Server{
		logger:  logger,
		isAlive: false,
	}
	pc := &webrtc.PeerConnection{}
	logLeakWG.Add(1)
	// Simulate how webrtc runs this callback in an unmanaged goroutine but force
	// it to outlive the surrounding test by waiting on the channel.
	go func() {
		<-logLeakStart
		removeStreamsOnPCDisconnect(server, pc, webrtc.PeerConnectionStateClosed)
		logLeakWG.Done()
	}()
}
