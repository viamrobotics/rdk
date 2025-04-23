package server

import (
	"context"
	"encoding/hex"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/pion/stun"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
)

// Characterizes the network through a series of general, UDP-based, and TCP-based network
// checks. Can and should be run asynchronously with server startup to avoid blocking.
func runNetworkChecks(ctx context.Context) {
	logger := logging.NewLogger("network-checks")
	if testing.Testing() {
		logger.Debug("Skipping network checks in a testing environment")
		return
	}

	logger.Info("Starting network checks")

	online := testGeneral(ctx)
	if !online {
		logger.Warn("Machine appears to be offline (cannot make connection to app.viam.com); skipping further network checks")
		return
	}

	if err := testUDP(ctx, logger.Sublogger("udp")); err != nil {
		logger.Errorw("Error running general network tests", "error", err)
	}

	if err := testTCP(ctx, logger.Sublogger("tcp")); err != nil {
		logger.Errorw("Error running general network tests", "error", err)
	}
}

// Tests general network connectivity (to app.viam.com.) Stolen from datamanager's
// sync/connectivity.go code. Returns false is offline and true if online.
func testGeneral(ctx context.Context) bool {
	timeout := 5 * time.Second
	attempts := 1
	if runtime.GOOS == "windows" {
		// TODO(RSDK-8344): this is temporary as we 1) debug connectivity issues on windows,
		// and 2) migrate to using the native checks on the underlying connection.
		timeout = 15 * time.Second
		attempts = 2
	}

	for i := range attempts {
		// Use DialDirectGRPC to make a connection to app.viam.com instead of a
		// basic net.Dial in order to ensure that the connection can be made
		// behind wifi or the BLE-SOCKS bridge (DialDirectGRPC can dial through
		// the BLE-SOCKS bridge.)
		ctx, cancel := context.WithTimeout(ctx, timeout)
		conn, err := rpc.DialDirectGRPC(ctx, "app.viam.com:443", nil)
		cancel()
		if err == nil {
			conn.Close() //nolint:gosec,errcheck
			return true
		}
		if i < attempts-1 {
			time.Sleep(time.Second)
		}
	}

	return false
}

var (
	stunServerURLsToTestUDP = []string{
		"global.stun.twilio.com:3478",
		"turn.viam.com:443",
		"stun.l.google.com:3478",
		"stun.l.google.com:19302",
		"stun.sipgate.net:3478",
		"stun.sipgate.net:3479",
	}
	stunServerURLsToTestTCP = []string{
		"turn.viam.com:443", // only STUN server that acceps TCP STUN traffic
	}
)

// Tests NAT over UDP against STUN servers.
func testUDP(ctx context.Context, logger logging.Logger) error {
	// Listen on arbitrary UDP port.
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		logger.Warn("Failed to listen over UDP on a port; UDP traffic may be blocked")
		return err
	}

	// `net.PacketConn`s do not function with contexts (only deadlines.) If passed-in
	// context expires (machine is likely shutting down,) _or_ tests finish, close the
	// underlying `net.PacketConn` asynchronously to stop ongoing network checks.
	testUDPDone := make(chan struct{})
	defer func() {
		testUDPDone <- struct{}{}
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-testUDPDone:
		}
		conn.Close() //nolint:gosec,errcheck
	}()

	// Set a deadline for all interactions of 10 seconds in the future.
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	// Build a STUN binding request to be used against all STUN servers.
	bindRequest, err := stun.Build([]stun.Setter{
		stun.TransactionID,
		stun.BindingRequest,
	}...)
	if err != nil {
		return err
	}
	bindRequestRaw, err := bindRequest.MarshalBinary()
	if err != nil {
		return err
	}

	var stunResponses []*STUNResponse
	for _, stunServerURLToTest := range stunServerURLsToTestUDP {
		if ctx.Err() != nil {
			logger.Info("Machine shutdown detected; stopping UDP network tests")
			return nil
		}

		logger := logger.WithFields("stun_server_url", stunServerURLToTest)

		stunResponse := NewSTUNResponse(stunServerURLToTest)
		stunResponses = append(stunResponses, stunResponse)

		udpAddr, err := net.ResolveUDPAddr("udp4", stunServerURLToTest)
		if err != nil {
			logger.Errorw("Error resolving URL to a UDP address", "error", err)
			continue
		}
		stunResponse.STUNServerAddr = udpAddr.String()

		// Write bind request on connection to UDP addr.
		bindStart := time.Now()
		n, err := conn.WriteTo(bindRequestRaw, udpAddr)
		if err != nil {
			logger.Errorw("Error writing to conn", "error", err)
			continue
		}
		if n != len(bindRequestRaw) {
			logger.Errorf("Only wrote %d/%d of bind request", n, len(bindRequestRaw))
			continue
		}

		// Receive response from connection.
		rawResponse := make([]byte, 2000 /* arbitrarily large */)
		_, _, err = conn.ReadFrom(rawResponse)
		if err != nil {
			logger.Errorw("Error reading from conn", "error", err)
			continue
		}

		response := &stun.Message{}
		if err := stun.Decode(rawResponse, response); err != nil {
			logger.Errorw("Error decoding STUN message", "error", err)
			continue
		}

		switch c := response.Type.Class; c {
		case stun.ClassSuccessResponse:
			var bindResponseAddr stun.XORMappedAddress
			if err := bindResponseAddr.GetFrom(response); err != nil {
				logger.Errorw("Error extracting address from STUN message", "error", err)
				continue
			}

			// Check for transaction ID mismatch.
			if bindRequest.TransactionID != response.TransactionID {
				logger.Errorf("Transaction ID mismatch (expected %s, got %s)",
					hex.EncodeToString(bindRequest.TransactionID[:]),
					hex.EncodeToString(response.TransactionID[:]),
				)
				continue
			}

			stunResponse.BindResponseAddr = bindResponseAddr.String()
			stunResponse.TimeToBindResponse = time.Since(bindStart)
		case stun.ClassErrorResponse, stun.ClassIndication, stun.ClassRequest:
			logger.Errorw("Unexpected STUN response received", "response_type", c)
		}
	}

	logSTUNResults(logger, stunResponses, "UDP")
	return nil
}

// Tests NAT over TCP against STUN servers.
func testTCP(ctx context.Context, logger logging.Logger) error {
	// Create a dialer with a consistent port (randomly chosen) from
	// which to dial over tcp.
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP: net.ParseIP("0.0.0.0"),
		},
	}

	// Build a STUN binding request to be used against all STUN servers.
	bindRequest, err := stun.Build([]stun.Setter{
		stun.TransactionID,
		stun.BindingRequest,
	}...)
	if err != nil {
		return err
	}
	bindRequestRaw, err := bindRequest.MarshalBinary()
	if err != nil {
		return err
	}

	var stunResponses []*STUNResponse
	for _, stunServerURLToTest := range stunServerURLsToTestTCP {
		if ctx.Err() != nil {
			logger.Info("Machine shutdown detected; stopping TCP network tests")
			return nil
		}

		logger := logger.WithFields("stun_server_url", stunServerURLToTest)

		// Unlike with UDP, TCP needs a new `conn` for every STUN server test (all
		// derived from the same dialer that uses the same local address.)
		conn, err := dialer.DialContext(ctx, "tcp", stunServerURLToTest)
		if err != nil {
			logger.Error("Error dialing STUN server via tcp")
			continue
		}

		// `net.Conn`s do not function with contexts (only deadlines.) If passed-in context
		// expires (machine is likely shutting down,) _or_ tests finish, close the underlying
		// `net.Conn` asynchronously to stop ongoing network checks.
		testTCPDone := make(chan struct{})
		defer func() {
			testTCPDone <- struct{}{}
		}()
		go func() {
			select {
			case <-ctx.Done():
			case <-testTCPDone:
			}
			conn.Close() //nolint:gosec,errcheck
		}()

		// Set a deadline for this interaction of 5 seconds in the future.
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			logger.Error("Error setting read deadline on TCP connection")
			continue
		}

		stunResponse := NewSTUNResponse(stunServerURLToTest)
		stunResponses = append(stunResponses, stunResponse)

		tcpAddr, err := net.ResolveTCPAddr("tcp", stunServerURLToTest)
		if err != nil {
			logger.Errorw("Error resolving URL to a TCP address", "error", err)
			continue
		}
		stunResponse.STUNServerAddr = tcpAddr.String()

		// Write bind request on connection to TCP addr.
		bindStart := time.Now()
		n, err := conn.Write(bindRequestRaw)
		if err != nil {
			logger.Errorw("Error writing to conn", "error", err)
			continue
		}
		if n != len(bindRequestRaw) {
			logger.Errorf("Only wrote %d/%d of bind request", n, len(bindRequestRaw))
			continue
		}

		// Receive response from connection.
		rawResponse := make([]byte, 2000 /* arbitrarily large */)
		_, err = conn.Read(rawResponse)
		if err != nil {
			logger.Errorw("Error reading from conn", "error", err)
			continue
		}

		response := &stun.Message{}
		if err := stun.Decode(rawResponse, response); err != nil {
			logger.Errorw("Error decoding STUN message", "error", err)
			continue
		}

		switch c := response.Type.Class; c {
		case stun.ClassSuccessResponse:
			var bindResponseAddr stun.XORMappedAddress
			if err := bindResponseAddr.GetFrom(response); err != nil {
				logger.Errorw("Error extracting address from STUN message", "error", err)
				continue
			}

			// Check for transaction ID mismatch.
			if bindRequest.TransactionID != response.TransactionID {
				logger.Errorf("Transaction ID mismatch (expected %s, got %s)",
					hex.EncodeToString(bindRequest.TransactionID[:]),
					hex.EncodeToString(response.TransactionID[:]),
				)
				continue
			}

			stunResponse.BindResponseAddr = bindResponseAddr.String()
			stunResponse.TimeToBindResponse = time.Since(bindStart)
		case stun.ClassErrorResponse, stun.ClassIndication, stun.ClassRequest:
			logger.Errorw("Unexpected STUN response received", "response_type", c)
		}
	}

	logSTUNResults(logger, stunResponses, "TCP")
	return nil
}
