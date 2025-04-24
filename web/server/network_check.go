package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/pion/stun"

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

	if err := testUDP(ctx, logger.Sublogger("udp")); err != nil {
		logger.Errorw("Error running general network tests", "error", err)
	}

	//if err := testTCP(ctx, logger.Sublogger("tcp")); err != nil {
	//logger.Errorw("Error running general network tests", "error", err)
	//}
}

// All reads, both UDP and TCP, get 5 seconds before being considered a timeout.
const readTimeout = 5 * time.Second

var (
	// TODO(RSDK-XXXXX): Attempt raw IPs of STUN server URLs in the event that DNS
	// resolution does not work.
	stunServerURLsToTestUDP = []string{
		"global.stun.twilio.com:3478",
		"turn.viam.com:443",
		"turn.viam.com:3478",
		"stun.l.google.com:3478",
		"stun.l.google.com:19302",
		"stun.sipgate.net:3478",
		"stun.sipgate.net:3479",
		// TODO(benji): Remove these testing error cases.
		//"stun.does.not.exist:3479",
		//"stun.sipgate.net:6500",
	}
	stunServerURLsToTestTCP = []string{
		// Viam's coturn is the only STUN server that accepts TCP STUN traffic.
		"turn.viam.com:443",
		"turn.viam.com:3478",
	}
)

// Sends the provided bindRequest to the provided STUN server with the provided packet
// connection and expects the provided transaction ID.
//
// Unexpected errors are returned from this function (failure to set a read deadline on
// the packet conn). Meaningful errors from STUN interactions are stored in the returned
// `STUNResponse.ErrorString` field.
func sendUDPBindRequest(
	bindRequest []byte,
	stunServerURLToTest string,
	conn net.PacketConn,
	transactionID [12]byte,
) (stunResponse *STUNResponse, retErr error) {
	stunResponse = &STUNResponse{STUNServerURL: stunServerURLToTest}

	udpAddr, err := net.ResolveUDPAddr("udp4", stunServerURLToTest)
	if err != nil {
		// TODO(RSDK-XXXXX): Attempt raw IPs of STUN server URLs in the event that DNS
		// resolution does not work.
		errorString := fmt.Sprintf("error resolving address: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	stunServerAddr := udpAddr.String()
	stunResponse.STUNServerAddr = &stunServerAddr

	// Write bind request on connection to UDP addr.
	bindStart := time.Now()
	n, err := conn.WriteTo(bindRequest, udpAddr)
	if err != nil {
		errorString := fmt.Sprintf("error writing to STUN connection: %v", err.Error())
		stunResponse.ErrorString = &errorString
		return
	}
	if n != len(bindRequest) {
		errorString := fmt.Sprintf("did not finish writing to STUN connection")
		stunResponse.ErrorString = &errorString
		return
	}

	// Set a read deadline for reading on the conn in this test and remove that deadline at
	// the end of this test.
	if retErr = conn.SetReadDeadline(time.Now().Add(readTimeout)); retErr != nil {
		return
	}
	defer func() {
		retErr = conn.SetReadDeadline(time.Time{})
	}()

	// Receive response from connection.
	rawResponse := make([]byte, 2000 /* arbitrarily large */)
	if _, _, err = conn.ReadFrom(rawResponse); err != nil {
		errorString := fmt.Sprintf("error reading from STUN connection: %v", err.Error())
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
			errorString := fmt.Sprintf("Transaction ID mismatch (expected %s, got %s)",
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

// Tests NAT over UDP against STUN servers.
func testUDP(ctx context.Context, logger logging.Logger) error {
	// Listen on arbitrary UDP port.
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		logger.Warn("Failed to listen over UDP on a port; UDP traffic may be blocked")
		return err
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
	for _, stunServerURLToTest := range stunServerURLsToTestUDP {
		if ctx.Err() != nil {
			logger.Info("Machine shutdown detected; stopping UDP network tests")
			return nil
		}

		stunResponse, err := sendUDPBindRequest(
			bindRequestRaw,
			stunServerURLToTest,
			conn,
			bindRequest.TransactionID,
		)
		if err != nil {
			logger.Warnf("error running UDP network test against %v: %v", stunServerURLToTest,
				err.Error())
			continue
		}
		stunResponses = append(stunResponses, stunResponse)
	}

	logSTUNResults(logger, stunResponses, sourceAddress, "udp")
	return nil
}

// Tests NAT over TCP against STUN servers.
func testTCP(ctx context.Context, logger logging.Logger) error {
	// Create a dialer with a consistent port (randomly chosen) from
	// which to dial over tcp.
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP("0.0.0.0"),
			Port: 23654, /* arbitrary but consistent across dials */
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

	var conn net.Conn
	defer func() {
		if conn != nil {
			conn.Close() //nolint:gosec,errcheck
		}
	}()
	var stunResponses []*STUNResponse
	var sourceAddress string
	for i, stunServerURLToTest := range stunServerURLsToTestTCP {
		if conn != nil {
			// Close any connection from previous iteration of for loop, as we will reuse the
			// same port. We must sleep for a moment after the `Close` call to avoid a "bind:
			// address already in use," as there is, presumably a small delay until the
			// underlying sock is closed.
			conn.Close() //nolint:gosec,errcheck
			conn = nil
			time.Sleep(100 * time.Millisecond)
		}

		if ctx.Err() != nil {
			logger.Info("Machine shutdown detected; stopping TCP network tests")
			return nil
		}

		logger := logger.WithFields("stun_server_url", stunServerURLToTest)

		// Unlike with UDP, TCP needs a new `conn` for every STUN server test (all
		// derived from the same dialer that uses the same local address).
		conn, err = dialer.DialContext(ctx, "tcp", stunServerURLToTest)
		if err != nil {
			logger.Errorw("Error dialing STUN server via tcp", "error", err)
			continue
		}

		if i == 0 {
			// Honor first TCP connection's local addr for now.
			sourceAddress = conn.LocalAddr().String()
		}

		// Set a deadline for this interaction of 5 seconds in the future.
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			logger.Error("Error setting read deadline on TCP connection")
			continue
		}

		stunResponse := &STUNResponse{}
		stunResponses = append(stunResponses, stunResponse)

		tcpAddr, err := net.ResolveTCPAddr("tcp", stunServerURLToTest)
		if err != nil {
			logger.Errorw("Error resolving URL to a TCP address", "error", err)
			continue
		}
		stunServerAddr := tcpAddr.String()
		stunResponse.STUNServerAddr = &stunServerAddr

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

			bindResponseAddrString := bindResponseAddr.String()
			bindTimeMS := time.Since(bindStart).Milliseconds()
			stunResponse.BindResponseAddr = &bindResponseAddrString
			stunResponse.TimeToBindResponseMS = &bindTimeMS
		case stun.ClassErrorResponse, stun.ClassIndication, stun.ClassRequest:
			logger.Errorw("Unexpected STUN response received", "response_type", c)
		}
	}

	logSTUNResults(logger, stunResponses, sourceAddress, "TCP")
	return nil
}
