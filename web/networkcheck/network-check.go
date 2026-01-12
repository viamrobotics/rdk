// Package networkcheck implements logic for collecting network statistics
package networkcheck

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/stun"

	"go.viam.com/rdk/logging"
)

// RunNetworkChecks characterizes the network through a series of DNS, UDP STUN, and TCP
// STUN network checks. Can and should be run asynchronously with server startup to avoid
// blocking. Specifying continueRunningTestDNS as true will run DNS network checks every 5
// minutes in a goroutine non-verbosely after this function completes until context error.
func RunNetworkChecks(ctx context.Context, rdkLogger logging.Logger, continueRunningTestDNS bool) {
	logger := rdkLogger.Sublogger("network-checks")
	if testing.Testing() {
		logger.Debug("Skipping network checks in a testing environment")
		return
	}

	logger.Info("Starting network checks")

	dnsSublogger := logger.Sublogger("dns")
	TestDNS(ctx, dnsSublogger, true /* verbose to log successes */)
	if continueRunningTestDNS {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Minute):
					TestDNS(ctx, dnsSublogger, false /* non-verbose to only log failures */)
				}
			}
		}()
	}

	if err := testUDP(ctx, logger.Sublogger("udp")); err != nil {
		logger.Errorw("Error running udp network tests", "error", err)
	}

	if err := testTCP(ctx, logger.Sublogger("tcp")); err != nil {
		logger.Errorw("Error running tcp network tests", "error", err)
	}
}

// All blocking I/O for all network checks gets 5 seconds before being considered a timeout.
const timeout = 5 * time.Second

var (
	systemdResolvedAddress = "127.0.0.53:53"
	serverIPSToTestDNS     = []string{
		"1.1.1.1:53",        // Cloudflare DNS
		"8.8.8.8:53",        // Google DNS
		"208.67.222.222:53", // OpenDNS
	}
	hostnamesToResolveDNS = []string{
		"cloudflare.com",
		"google.com",
		"github.com",
		"viam.com",
	}

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

func init() {
	// Append the systemd-resolved IP address to the list of IPs to test DNS connectivity
	// against when the operating system is Linux-based. MacOS and Windows do not have
	// systemd nor systemd-resolved. Some Linux distros will not be running it either, but
	// we will find that out during testing.
	if runtime.GOOS == "linux" {
		serverIPSToTestDNS = append(serverIPSToTestDNS, systemdResolvedAddress)
	}
}

// Tests connectivity to a specific DNS server.
func testDNSServerConnectivity(ctx context.Context, dnsServer string) *DNSResult {
	dnsResult := &DNSResult{
		TestType:  ConnectionDNSTestType,
		DNSServer: &dnsServer,
	}

	// TODO(benji): Fall back to TCP in the event of a UDP timeout. That's what Golang's
	// default resolver does.
	start := time.Now()
	conn, err := net.DialTimeout("udp", dnsServer, timeout)
	if err != nil {
		errorString := fmt.Sprintf("failed to connect to DNS server: %v", err)
		dnsResult.ErrorString = &errorString
		return dnsResult
	}
	connectTime := time.Since(start).Milliseconds()
	dnsResult.ConnectTimeMS = &connectTime

	// `net.Conn`s do not function with contexts (only deadlines). If passed-in context
	// expires (machine is likely shutting down), _or_ tests finish, close the underlying
	// `net.PacketConn` asynchronously to stop ongoing network checks.
	testDNSServerConnectivityDone := make(chan struct{})
	defer close(testDNSServerConnectivityDone)
	go func() {
		select {
		case <-ctx.Done():
		case <-testDNSServerConnectivityDone:
		}
		conn.Close() //nolint:errcheck
	}()

	// Send a simple DNS query for "google.com"'s A record.
	dnsQuery := []byte{
		0x12, 0x34, // Transaction ID
		0x01, 0x00, // Flags: standard query
		0x00, 0x01, // Questions: 1
		0x00, 0x00, // Answer RRs: 0
		0x00, 0x00, // Authority RRs: 0
		0x00, 0x00, // Additional RRs: 0
		0x06, 'g', 'o', 'o', 'g', 'l', 'e', // length + "google"
		0x03, 'c', 'o', 'm', // length + "com"
		0x00,       // null terminator
		0x00, 0x01, // Type: A
		0x00, 0x01, // Class: IN
	}

	queryStart := time.Now()
	_, err = conn.Write(dnsQuery)
	if err != nil {
		errorString := fmt.Sprintf("failed to send DNS query: %v", err)
		dnsResult.ErrorString = &errorString
		return dnsResult
	}

	err = conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		errorString := fmt.Sprintf("failed to set read deadline: %v", err)
		dnsResult.ErrorString = &errorString
		return dnsResult
	}

	response := make([]byte, 512 /* standard DNS response size */)
	n, err := conn.Read(response)
	if err != nil {
		errorString := fmt.Sprintf("failed to read DNS response: %v", err)
		dnsResult.ErrorString = &errorString
		return dnsResult
	}
	queryTime := time.Since(queryStart).Milliseconds()
	dnsResult.QueryTimeMS = &queryTime

	if n < 12 /* minimum DNS header size */ {
		errorString := "DNS response too short"
		dnsResult.ErrorString = &errorString
		return dnsResult
	}
	if response[0] != 0x12 || response[1] != 0x34 {
		errorString := "DNS response transaction ID mismatch"
		dnsResult.ErrorString = &errorString
		return dnsResult
	}
	if response[2]&0x80 == 0 {
		errorString := "received query instead of response"
		dnsResult.ErrorString = &errorString
		return dnsResult
	}

	responseSize := int64(n)
	dnsResult.ResponseSize = &responseSize

	return dnsResult
}

// Tests DNS resolution using the system's default resolver.
func testDNSResolution(ctx context.Context, hostname string) *DNSResult {
	dnsResult := &DNSResult{
		TestType: ResolutionDNSTestType,
		Hostname: &hostname,
	}

	start := time.Now()
	// Use default resolver's method so we can pass in context.
	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	resolutionTime := time.Since(start).Milliseconds()
	dnsResult.ResolutionTimeMS = &resolutionTime

	if err != nil {
		errorString := fmt.Sprintf("system DNS resolution failed: %v", err.Error())
		dnsResult.ErrorString = &errorString
		return dnsResult
	}

	if len(addrs) == 0 {
		errorString := "no IP addresses returned"
		dnsResult.ErrorString = &errorString
		return dnsResult
	}

	resolvedIPs := strings.Join(addrs, ", ")
	dnsResult.ResolvedIPs = &resolvedIPs

	return dnsResult
}

// Gets the contents of `/etc/resolv.conf` is it exists. Returns an empty string if file
// does not exist.
func getResolvConfContents() string {
	resolvConfContents, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	return string(resolvConfContents)
}

// Gets the contents of `/etc/systemd/resolved.conf` is it exists. Returns an empty string if file
// does not exist.
func getSystemdResolveConfContents() string {
	systemdResolvedConfContents, err := os.ReadFile("/etc/systemd/resolved.conf")
	if err != nil {
		return ""
	}
	return string(systemdResolvedConfContents)
}

// TestDNS tests connectivity to DNS servers and attempts to resolve hostnames with the
// system DNS resolver. Should be run at startup, every 5 minutes afterward, and whenever
// dialing app.viam.com fails. If verbose is true, logs successful and unsuccessful
// results. Logs only unsuccessful results otherwise.
func TestDNS(ctx context.Context, logger logging.Logger, verbose bool) {
	var dnsResults []*DNSResult

	for _, dnsServer := range serverIPSToTestDNS {
		if ctx.Err() != nil {
			logger.Info("Shutdown detected; stopping DNS connectivity tests")
			return
		}

		result := testDNSServerConnectivity(ctx, dnsServer)

		// If the error from the systemd-resolved DNS resolver (at 127.0.0.53:53 in only
		// _some_ Linux distros) test reported "connection refused," do not include the result
		// in the final list. systemd-resolved is likely not installed or not currently
		// running in that case, and that is not indicative of a DNS issue.
		if result.ErrorString != nil && strings.Contains(*result.ErrorString, "connection refused") &&
			dnsServer == systemdResolvedAddress {
			continue
		}

		dnsResults = append(dnsResults, result)
	}

	for _, hostname := range hostnamesToResolveDNS {
		if ctx.Err() != nil {
			logger.Info("Shutdown detected; stopping DNS resolution tests")
			return
		}

		dnsResults = append(dnsResults, testDNSResolution(ctx, hostname))
	}

	logDNSResults(
		logger,
		dnsResults,
		getResolvConfContents(),
		getSystemdResolveConfContents(),
		verbose,
	)
}

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
	if err = conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
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
		conn.Close() //nolint:errcheck
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
			logger.Info("Shutdown detected; stopping UDP network tests")
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
	if err = conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
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
			conn.Close() //nolint:errcheck
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
			logger.Info("Shutdown detected; stopping TCP network tests")
			return nil
		}

		connMu.Lock()
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
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
		conn.Close() //nolint:errcheck
		connMu.Unlock()
	}

	logSTUNResults(logger, stunResponses, "" /* no udpSourceAddress */, "tcp")
	return nil
}
