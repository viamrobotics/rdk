package grpc

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/pion/interceptor"
	pionLogging "github.com/pion/logging"
	"github.com/viamrobotics/webrtc/v3"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// OnTrackCB is the signature of the OnTrack callback function
// a resource may register with a SharedConn which supports WebRTC.
type OnTrackCB func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver)

// SharedConn wraps both a GRPC connection & (optionally) a peer connection & controls access to both.
// For modules, the grpc connection is over a Unix socket. The WebRTC `PeerConnection` is made
// separately. The `SharedConn` continues to implement the `rpc.ClientConn` interface by pairing up
// the two underlying connections a client may want to communicate over.
//
// Users of SharedConn must serialize calls to `ResetConn`, `GenerateEncodedOffer`,
// `ProcessEncodedAnswer` and `Close`.
//
// The lifetime of SharedConn objects are a bit nuanced. SharedConn objects are expected to be
// handed to resource Client objects. When connections break and become reestablished, those client
// objects are not recreated, but rather we swap in new connection objects under the hood.
//
// This is because resource management controls updating the graph nodes with Client objects only
// when calls to `Reconfigure` are made. But connections can break and heal without any calls to
// Reconfigure.
//
// Thus the lifetime of a SharedConn for a client is _not_:
// 1) Object initialize
// 2) Working
// 3) Close
// 4) Object destruction
// And back to initialization.
//
// But rather:
// 1) Object initialization
// 2) `ResetConn`
// 3) `Close`
// 4) `ResetConn`
// ...
// 5) Object destruction (at shutdown or resource removed from config).
//
// Each call to `ResetConn` is a new "generation" of connections. When a new gRPC connection is
// handed to a SharedConn, the SharedConn will start the process of establishing a new
// PeerConnection. A call to access the PeerConnection will block until this process completes,
// either successfully or not.
//
// When a PeerConnection cannot be established, the SharedConn is in a degraded state where the gRPC
// connection is still valid. The caller has the option of either disabling functionality intended
// for the PeerConnection, or implementing that functionality over gRPC (which would presumably be
// less performant).
type SharedConn struct {
	grpcConn ReconfigurableClientConn

	// `peerConnMu` synchronizes changes to the underlying `peerConn`. Such that calls consecutive
	// calls to `GrpcConn` and `PeerConn` will return connections from the same (or newer, but not
	// prior) "generations".
	peerConnMu     sync.Mutex
	peerConn       *webrtc.PeerConnection
	peerConnReady  <-chan struct{}
	peerConnClosed <-chan struct{}
	// peerConnFailed gets closed when a PeerConnection fails to connect. The peerConn pointer is
	// set to nil before this channel is closed.
	peerConnFailed chan struct{}

	onTrackCBByTrackNameMu sync.Mutex
	onTrackCBByTrackName   map[string]OnTrackCB

	logger logging.Logger
}

// Invoke forwards to the underlying GRPC Connection.
func (sc *SharedConn) Invoke(
	ctx context.Context,
	method string,
	args, reply interface{},
	opts ...googlegrpc.CallOption,
) error {
	return sc.grpcConn.Invoke(ctx, method, args, reply, opts...)
}

// NewStream forwards to the underlying GRPC Connection.
func (sc *SharedConn) NewStream(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	method string,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	return sc.grpcConn.NewStream(ctx, desc, method, opts...)
}

// AddOnTrackSub adds an OnTrack subscription for the track.
func (sc *SharedConn) AddOnTrackSub(trackName string, onTrackCB OnTrackCB) {
	sc.onTrackCBByTrackNameMu.Lock()
	defer sc.onTrackCBByTrackNameMu.Unlock()
	sc.onTrackCBByTrackName[trackName] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the track.
func (sc *SharedConn) RemoveOnTrackSub(trackName string) {
	sc.onTrackCBByTrackNameMu.Lock()
	defer sc.onTrackCBByTrackNameMu.Unlock()
	delete(sc.onTrackCBByTrackName, trackName)
}

// GrpcConn returns a gRPC capable client connection.
func (sc *SharedConn) GrpcConn() *ReconfigurableClientConn {
	return &sc.grpcConn
}

// PeerConn returns a WebRTC PeerConnection capable of sending video/audio data.  A call to PeerConn
// concurrent with `ResetConn` or `Close` is allowed to return either the old or new connection.
func (sc *SharedConn) PeerConn() *webrtc.PeerConnection {
	// Grab a snapshot of the SharedConn state. Release locks before blocking on channel reads.
	sc.peerConnMu.Lock()
	readyCh := sc.peerConnReady
	failCh := sc.peerConnFailed
	ret := sc.peerConn
	sc.peerConnMu.Unlock()

	// Block until the peer connection result is known.
	select {
	case <-readyCh:
	case <-failCh:
	}

	return ret
}

// ResetConn acts as a constructor for `SharedConn`. ResetConn replaces the underlying
// connection objects in addition to some other initialization.
//
// The first call to `ResetConn` is guaranteed to happen before any access to connection objects
// happens. But subequent calls can be entirely asynchronous to components/services accessing
// `SharedConn` for connection objects.
func (sc *SharedConn) ResetConn(conn rpc.ClientConn, moduleLogger logging.Logger) {
	sc.grpcConn.ReplaceConn(conn)
	if sc.logger == nil {
		// The first call to `ResetConn` happens before anything can access `sc.logger`. So long as
		// we never write to the member variable, everything can continue to access this without
		// locks.
		sc.logger = moduleLogger.Sublogger("networking.conn")
	}

	// It is safe to access this without a mutex as it is only ever nil once at the beginning of the
	// SharedConn's lifetime
	if sc.onTrackCBByTrackName == nil {
		// Same initilization argument as above with the logger.
		sc.onTrackCBByTrackName = make(map[string]OnTrackCB)
	}

	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()

	sc.peerConnFailed = make(chan struct{})
	// If initializing a PeerConnection fails for any reason, we perform the following cleanup
	// steps.
	guard := rutils.NewGuard(func() {
		if sc.peerConn != nil {
			utils.UncheckedError(sc.peerConn.GracefulClose())
		}
		sc.peerConn = nil
		close(sc.peerConnFailed)
	})
	defer guard.OnFail()

	if sc.peerConn != nil {
		sc.logger.Warn("SharedConn is being reset with an active peer connection.")
		utils.UncheckedError(sc.peerConn.GracefulClose())
		sc.peerConn = nil
	}

	peerConn, err := NewLocalPeerConnection(sc.logger)
	if err != nil {
		sc.logger.Warnw("Unable to create optional peer connection for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn = peerConn
	sc.peerConnReady, sc.peerConnClosed, err = rpc.ConfigureForRenegotiation(peerConn, rpc.PeerRoleClient, sc.logger)
	if err != nil {
		sc.logger.Warnw("Unable to create optional renegotiation channel for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn.OnTrack(func(trackRemote *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		sc.onTrackCBByTrackNameMu.Lock()
		onTrackCB, ok := sc.onTrackCBByTrackName[trackRemote.StreamID()]
		sc.onTrackCBByTrackNameMu.Unlock()
		if !ok {
			msg := "Callback not found for StreamID: %s, keys(resOnTrackCBs): %#v"
			sc.logger.Errorf(msg, trackRemote.StreamID(), maps.Keys(sc.onTrackCBByTrackName))
			return
		}
		onTrackCB(trackRemote, rtpReceiver)
	})

	guard.Success()
}

// GenerateEncodedOffer creates a WebRTC offer that's JSON + base64 encoded. If an error is
// returned, `SharedConn.PeerConn` will return nil until a following `Reset`.
func (sc *SharedConn) GenerateEncodedOffer() (string, error) {
	// If this generating an offer fails for any reason, we perform the following cleanup steps.
	guard := rutils.NewGuard(func() {
		utils.UncheckedError(sc.peerConn.GracefulClose())
		sc.peerConnMu.Lock()
		sc.peerConn = nil
		sc.peerConnMu.Unlock()
		close(sc.peerConnFailed)
	})
	defer guard.OnFail()

	// Accessing the pointer without a lock is safe. Only `ResetConn`, `GenerateEncodedOffer`,
	// `ProcessEncodedAnswer` and `Close` can change the `peerConn` pointer. Users of SharedConn
	// must serialize calls to those methods.
	//
	// We copy this pointer for convenience, not correctness.
	pc := sc.peerConn
	if pc == nil {
		return "", errors.New("peer connections disabled")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-webrtc.GatheringCompletePromise(pc)
	ret, err := rpc.EncodeSDP(pc.LocalDescription())
	if err != nil {
		return "", err
	}

	guard.Success()
	return ret, nil
}

// ProcessEncodedAnswer sets the remote description to the answer and waits for the connection to be
// ready as per the negotiation channel being opened. The answer is expected to be JSON + base64
// encoded. If an error is returned, `SharedConn.PeerConn` will return nil until a following
// `Reset`.
func (sc *SharedConn) ProcessEncodedAnswer(encodedAnswer string) error {
	guard := rutils.NewGuard(func() {
		utils.UncheckedError(sc.peerConn.GracefulClose())
		sc.peerConnMu.Lock()
		sc.peerConn = nil
		sc.peerConnMu.Unlock()
		close(sc.peerConnFailed)
	})
	defer guard.OnFail()

	// Accessing the pointer without a lock is safe. Only `ResetConn`, `GenerateEncodedOffer`,
	// `ProcessEncodedAnswer` and `Close` can change the `peerConn` pointer. Users of SharedConn
	// must serialize calls to those methods.
	//
	// We copy this pointer for convenience, not correctness.
	pc := sc.peerConn
	if pc == nil {
		return errors.New("PeerConnection was not initialized")
	}

	answer := webrtc.SessionDescription{}
	if err := rpc.DecodeSDP(encodedAnswer, &answer); err != nil {
		return err
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		return err
	}

	guard.Success()
	return nil
}

// Close closes a shared connection.
func (sc *SharedConn) Close() error {
	var err error
	sc.peerConnMu.Lock()
	if sc.peerConn != nil {
		err = sc.peerConn.GracefulClose()
		sc.peerConn = nil
		close(sc.peerConnFailed)
	}
	sc.peerConnMu.Unlock()

	return multierr.Combine(err, sc.grpcConn.Close())
}

// NewLocalPeerConnection creates a peer connection that only accepts loopback
// address candidates.
func NewLocalPeerConnection(logger logging.Logger) (*webrtc.PeerConnection, error) {
	m := webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	i := interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(&m, &i); err != nil {
		return nil, err
	}

	var settingEngine webrtc.SettingEngine
	settingEngine.SetIncludeLoopbackCandidate(true)
	settingEngine.SetRelayAcceptanceMinWait(3 * time.Second)
	settingEngine.SetIPFilter(func(ip net.IP) bool {
		// Disallow ipv6 addresses since grpc-go does not currently support IPv6 scoped literals.
		// See related grpc-go issue: https://github.com/grpc/grpc-go/issues/3272.
		//
		// Stolen from net/ip.go, `IP.String` method.
		// Disallow non loopback addresses as this is a local peer connection
		if p4 := ip.To4(); len(p4) == net.IPv4len && p4.IsLoopback() {
			logger.Debugf("SetIPFilter allowing loppback ip: %s", ip.String())
			return true
		}
		logger.Debugf("SetIPFilter disallowing ip: %s", ip.String())

		return false
	})

	// RSDK-8547: WebRTC video streams expect an "increasing" SRTP value. When we forward RTP
	// packets through different hops, those values are copied as-is. If one connection in this
	// chain of hops has a blip, it will reset its SRTP value. Other hops in the chain that were not
	// disconnected will see these unexpected change in SRTP values and interpret it as a replay
	// attack, dropping the data. We disable this "protection" as per the justification in the noted
	// ticket.
	settingEngine.DisableSRTPReplayProtection(true)

	options := []func(a *webrtc.API){webrtc.WithMediaEngine(&m), webrtc.WithInterceptorRegistry(&i)}
	if utils.Debug {
		settingEngine.LoggerFactory = WebRTCLoggerFactory{logger}
	}
	options = append(options, webrtc.WithSettingEngine(settingEngine))
	webAPI := webrtc.NewAPI(options...)

	// attempt to construct a PeerConnection
	pc, err := webAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		logger.Debugw("Unable to create optional peer connection for module. Skipping WebRTC for module...", "err", err)
		return nil, err
	}
	return pc, nil
}

// WebRTCLoggerFactory wraps a logging.Logger for use with pion's webrtc logging system.
type WebRTCLoggerFactory struct {
	Logger logging.Logger
}

type webrtcLogger struct {
	logger logging.Logger
}

func (l webrtcLogger) loggerWithSkip() logging.Logger {
	logger := logging.FromZapCompatible(l.logger.WithOptions(zap.AddCallerSkip(1)))
	return logger
}

func (l webrtcLogger) Trace(msg string) {
	l.loggerWithSkip().Debug(msg)
}

func (l webrtcLogger) Tracef(format string, args ...interface{}) {
	l.loggerWithSkip().Debugf(format, args...)
}

func (l webrtcLogger) Debug(msg string) {
	l.loggerWithSkip().Debug(msg)
}

func (l webrtcLogger) Debugf(format string, args ...interface{}) {
	l.loggerWithSkip().Debugf(format, args...)
}

func (l webrtcLogger) Info(msg string) {
	l.loggerWithSkip().Info(msg)
}

func (l webrtcLogger) Infof(format string, args ...interface{}) {
	l.loggerWithSkip().Infof(format, args...)
}

func (l webrtcLogger) Warn(msg string) {
	l.loggerWithSkip().Warn(msg)
}

func (l webrtcLogger) Warnf(format string, args ...interface{}) {
	l.loggerWithSkip().Warnf(format, args...)
}

func (l webrtcLogger) Error(msg string) {
	l.loggerWithSkip().Error(msg)
}

func (l webrtcLogger) Errorf(format string, args ...interface{}) {
	l.loggerWithSkip().Errorf(format, args...)
}

// NewLogger returns a new webrtc logger under the given scope.
func (lf WebRTCLoggerFactory) NewLogger(scope string) pionLogging.LeveledLogger {
	logger := lf.Logger.Sublogger(scope)
	return webrtcLogger{logger}
}
