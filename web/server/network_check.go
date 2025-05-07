package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pion/stun"

	"go.viam.com/rdk/logging"
)

// Characterizes the network through a series of UDP and TCP STUN network checks. Can and
// should be run asynchronously with server startup to avoid blocking.
func runNetworkChecks(ctx context.Context, rdkLogger logging.Logger) {
	logger := rdkLogger.Sublogger("network-checks")
	if testing.Testing() {
		logger.Debug("Skipping network checks in a testing environment")
		return
	}

	logger.Info("Starting network checks")

	if err := testUDP(ctx, logger.Sublogger("udp")); err != nil {
		logger.Errorw("Error running udp network tests", "error", err)
	}

	if err := testTCP(ctx, logger.Sublogger("tcp")); err != nil {
		logger.Errorw("Error running tcp network tests", "error", err)
	}
}

// All reads, both UDP and TCP, get 5 seconds before being considered a timeout.
const readTimeout = 5 * time.Second

var (
	// TODO(RSDK-10657): Attempt raw IPs of STUN server URLs in the event that DNS
	// resolution does not work.
	stunServerURLsToTestUDP = []string{
		"global.stun.twilio.com:3478",
		"turn.viam.com:443",
		"turn.viam.com:3478",
		"stun.l.google.com:3478",
		"stun.l.google.com:19302",
		"stun.sipgate.net:3478",
		"stun.sipgate.net:3479",
	}
	stunServerURLsToTestTCP = []string{
		// Viam's coturn is the only STUN server that accepts TCP STUN traffic.
		"turn.viam.com:443",
		"turn.viam.com:3478",
	}
)

// Sends the provided bindRequest to the provided STUN server with the provided packet
// connection and expects the provided transaction ID. Uses cached resolved IPs if
// possible.
func sendUDPBindRequest(
	bindRequest []byte,
	stunServerURLToTest string,
	conn net.PacketConn,
	transactionID [12]byte,
	cachedResolvedIPs map[string]net.IP,
) (stunResponse *STUNResponse) {
	stunResponse = &STUNResponse{STUNServerURL: stunServerURLToTest}

	// Create UDP addr for bind request (or get it from cache).
	stunServerHost, stunServerPortString, err := net.SplitHostPort(stunServerURLToTest)
	if err != nil {
		errorString := fmt.Sprintf("error splitting STUN server URL: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	udpAddr := &net.UDPAddr{}
	udpAddr.Port, err = strconv.Atoi(stunServerPortString)
	if err != nil {
		errorString := fmt.Sprintf("error parsing STUN server port: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	var exists bool
	udpAddr.IP, exists = cachedResolvedIPs[stunServerHost]
	if !exists {
		ipAddr, err := net.ResolveIPAddr("ip", stunServerHost)
		if err != nil {
			// TODO(RSDK-10657): Attempt raw IPs of STUN server URLs in the event that DNS
			// resolution does not work.
			errorString := fmt.Sprintf("error resolving address: %v", err.Error())
			stunResponse.ErrorString = &errorString
			return
		}
		udpAddr.IP = ipAddr.IP
		cachedResolvedIPs[stunServerHost] = ipAddr.IP
	}
	stunServerAddr := udpAddr.String()
	stunResponse.STUNServerAddr = &stunServerAddr

	// Write bind request on connection to UDP addr.
	bindStart := time.Now()
	n, err := conn.WriteTo(bindRequest, udpAddr)
	if err != nil {
		errorString := fmt.Sprintf("error writing to packet conn: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	if n != len(bindRequest) {
		errorString := fmt.Sprintf(
			"did not finish writing to packet conn (%d/%d bytes)",
			n,
			len(bindRequest),
		)
		stunResponse.ErrorString = &errorString
		return
	}

	// Set a read deadline for reading on the conn in this test and remove that deadline at
	// the end of this test.
	if err = conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		errorString := fmt.Sprintf("error setting read deadline on packet conn: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	defer func() {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			errorString := fmt.Sprintf("error (un)setting read deadline on packet conn: %v", err.Error())
			if stunResponse.ErrorString != nil { // already set
				errorString = *stunResponse.ErrorString + " and " + errorString
			}
			stunResponse.ErrorString = &errorString
		}
	}()

	// Receive response from connection.
	rawResponse := make([]byte, 2000 /* arbitrarily large */)
	if _, _, err = conn.ReadFrom(rawResponse); err != nil {
		errorString := fmt.Sprintf("error reading from packet conn: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}

	response := &stun.Message{}
	if err := stun.Decode(rawResponse, response); err != nil {
		errorString := fmt.Sprintf("error decoding STUN message: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}

	switch c := response.Type.Class; c {
	case stun.ClassSuccessResponse:
		var bindResponseAddr stun.XORMappedAddress
		if err := bindResponseAddr.GetFrom(response); err != nil {
			errorString := fmt.Sprintf("error extracting mapped address from STUN response: %v",
				err.Error())
			stunResponse.ErrorString = &errorString
			return
		}

		// Check for transaction ID mismatch.
		if transactionID != response.TransactionID {
			errorString := fmt.Sprintf("transaction ID mismatch (expected %s, got %s)",
				hex.EncodeToString(transactionID[:]),
				hex.EncodeToString(response.TransactionID[:]),
			)
			stunResponse.ErrorString = &errorString
			return
		}

		bindResponseAddrString := bindResponseAddr.String()
		bindTimeMS := time.Since(bindStart).Milliseconds()
		stunResponse.BindResponseAddr = &bindResponseAddrString
		stunResponse.TimeToBindResponseMS = &bindTimeMS
	case stun.ClassErrorResponse, stun.ClassIndication, stun.ClassRequest:
		errorString := fmt.Sprintf("unexpected STUN response received: %s", c)
		stunResponse.ErrorString = &errorString
	}

	return
}

// Tests NAT over UDP against STUN servers.
func testUDP(ctx context.Context, logger logging.Logger) error {
	// Listen on arbitrary UDP port.
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		logger.Warnw("Failed to listen over UDP on a port; UDP traffic may be blocked",
			"error", err)
		return nil
	}
	sourceAddress := conn.LocalAddr().String()

	// `net.PacketConn`s do not function with contexts (only deadlines). If passed-in
	// context expires (machine is likely shutting down), _or_ tests finish, close the
	// underlying `net.PacketConn` asynchronously to stop ongoing network checks.
	testUDPDone := make(chan struct{})
	defer close(testUDPDone)
	go func() {
		select {
		case <-ctx.Done():
		case <-testUDPDone:
		}
		conn.Close() //nolint:gosec,errcheck
	}()

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
	cachedResolvedIPs := make(map[string]net.IP)
	for _, stunServerURLToTest := range stunServerURLsToTestUDP {
		if ctx.Err() != nil {
			logger.Info("Machine shutdown detected; stopping UDP network tests")
			return nil
		}

		stunResponse := sendUDPBindRequest(
			bindRequestRaw,
			stunServerURLToTest,
			conn,
			bindRequest.TransactionID,
			cachedResolvedIPs,
		)
		stunResponses = append(stunResponses, stunResponse)
	}

	logSTUNResults(logger, stunResponses, sourceAddress, "udp")
	return nil
}

func sendTCPBindRequest(
	bindRequest []byte,
	stunServerURLToTest string,
	conn net.Conn,
	transactionID [12]byte,
) (stunResponse *STUNResponse) {
	stunResponse = &STUNResponse{STUNServerURL: stunServerURLToTest}

	tcpSourceAddress := conn.LocalAddr().String()
	stunResponse.TCPSourceAddress = &tcpSourceAddress

	tcpAddr, err := net.ResolveTCPAddr("tcp", stunServerURLToTest)
	if err != nil {
		// TODO(RSDK-10657): Attempt raw IPs of STUN server URLs in the event that DNS
		// resolution does not work.
		errorString := fmt.Sprintf("error resolving address: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	stunServerAddr := tcpAddr.String()
	stunResponse.STUNServerAddr = &stunServerAddr

	// Write bind request on connection.
	bindStart := time.Now()
	n, err := conn.Write(bindRequest)
	if err != nil {
		errorString := fmt.Sprintf("error writing to connection: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	if n != len(bindRequest) {
		errorString := fmt.Sprintf(
			"did not finish writing to connection (%d/%d bytes)",
			n,
			len(bindRequest),
		)
		stunResponse.ErrorString = &errorString
		return
	}

	// Set a read deadline for reading on the conn in this test and remove that deadline at
	// the end of this test.
	if err = conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		errorString := fmt.Sprintf("error setting read deadline on connection: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	defer func() {
		// This might be unnecessary since the next test will use a new `net.Conn`.
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			errorString := fmt.Sprintf("error (un)setting read deadline on connection: %v", err.Error())
			if stunResponse.ErrorString != nil { // already set
				errorString = *stunResponse.ErrorString + " and " + errorString
			}
			stunResponse.ErrorString = &errorString
		}
	}()

	// Receive response from connection.
	rawResponse := make([]byte, 2000 /* arbitrarily large */)
	if _, err = conn.Read(rawResponse); err != nil {
		errorString := fmt.Sprintf("error reading from connection: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}

	response := &stun.Message{}
	if err := stun.Decode(rawResponse, response); err != nil {
		errorString := fmt.Sprintf("error decoding STUN message: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}

	switch c := response.Type.Class; c {
	case stun.ClassSuccessResponse:
		var bindResponseAddr stun.XORMappedAddress
		if err := bindResponseAddr.GetFrom(response); err != nil {
			errorString := fmt.Sprintf("error extracting mapped address from STUN response: %v",
				err.Error())
			stunResponse.ErrorString = &errorString
			return
		}

		// Check for transaction ID mismatch.
		if transactionID != response.TransactionID {
			errorString := fmt.Sprintf("transaction ID mismatch (expected %s, got %s)",
				hex.EncodeToString(transactionID[:]),
				hex.EncodeToString(response.TransactionID[:]),
			)
			stunResponse.ErrorString = &errorString
			return
		}

		bindResponseAddrString := bindResponseAddr.String()
		bindTimeMS := time.Since(bindStart).Milliseconds()
		stunResponse.BindResponseAddr = &bindResponseAddrString
		stunResponse.TimeToBindResponseMS = &bindTimeMS
	case stun.ClassErrorResponse, stun.ClassIndication, stun.ClassRequest:
		errorString := fmt.Sprintf("unexpected STUN response received: %s", c)
		stunResponse.ErrorString = &errorString
		return
	}

	return
}

// Tests NAT over TCP against STUN servers.
func testTCP(ctx context.Context, logger logging.Logger) error {
	// Each TCP test will create their own TCP connection through this `net.Conn` variable.
	// `net.Conn`s do not function with contexts (only deadlines). If passed-in context
	// expires (machine is likely shutting down), _or_ tests finish, close the underlying
	// `net.Conn` asynchronously to stop ongoing network checks.
	var conn net.Conn
	var connMu sync.Mutex
	testTCPDone := make(chan struct{})
	defer close(testTCPDone)
	go func() {
		select {
		case <-ctx.Done():
		case <-testTCPDone:
		}
		connMu.Lock()
		if conn != nil {
			conn.Close() //nolint:gosec,errcheck
		}
		connMu.Unlock()
	}()

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

		connMu.Lock()
		dialCtx, cancel := context.WithTimeout(ctx, readTimeout)
		dialer := &net.Dialer{} // use an empty dialer to get access to the `DialContext` method
		conn, err = dialer.DialContext(dialCtx, "tcp", stunServerURLToTest)
		cancel()
		if err != nil {
			connMu.Unlock()

			// Error to TCP dial to the STUN server should be reported in test results.
			errorString := fmt.Sprintf("error dialing: %v", err.Error())
			stunResponses = append(stunResponses, &STUNResponse{
				STUNServerURL: stunServerURLToTest,
				ErrorString:   &errorString,
			})
			continue
		}
		connMu.Unlock()

		stunResponse := sendTCPBindRequest(
			bindRequestRaw,
			stunServerURLToTest,
			conn,
			bindRequest.TransactionID,
		)
		stunResponses = append(stunResponses, stunResponse)

		connMu.Lock()
		conn.Close() //nolint:gosec,errcheck
		connMu.Unlock()
	}

	logSTUNResults(logger, stunResponses, "" /* no udpSourceAddress */, "tcp")
	return nil
}
