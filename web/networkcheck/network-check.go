// Package networkcheck implements logic for collecting network statistics
package networkcheck

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/stun"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"go.viam.com/rdk/logging"
)

const udp4Network = "udp4"

// RunNetworkChecks characterizes the network through a series of DNS, UDP STUN, TCP STUN,
// and packet loss network checks. Can and should be run asynchronously with server startup
// to avoid blocking. Specifying continueRunningTests as true will run DNS and packet loss
// checks every 5 minutes in goroutines non-verbosely after this function completes until
// context error.
func RunNetworkChecks(ctx context.Context, rdkLogger logging.Logger, continueRunningTests bool) {
	logger := rdkLogger.Sublogger("network-checks")
	if testing.Testing() {
		logger.Debug("Skipping network checks in a testing environment")
		return
	}

	logger.Info("Starting network checks")

	dnsSublogger := logger.Sublogger("dns")
	TestDNS(ctx, dnsSublogger, true /* verbose to log successes */)

	if err := testUDP(ctx, logger.Sublogger("udp")); err != nil {
		logger.Errorw("Error running udp network tests", "error", err)
	}

	if err := testTCP(ctx, logger.Sublogger("tcp")); err != nil {
		logger.Errorw("Error running tcp network tests", "error", err)
	}

	packetLossSublogger := logger.Sublogger("packet-loss")
	TestPacketLoss(ctx, packetLossSublogger, true /* verbose to log successes */)

	if continueRunningTests {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Minute):
					TestDNS(ctx, dnsSublogger, false /* non-verbose to only log failures */)
					TestPacketLoss(ctx, packetLossSublogger, false /* non-verbose to only log failures */)
				}
			}
		}()
	}
}

const (
	// All blocking I/O for all network checks gets 5 seconds before being considered a timeout.
	timeout = 5 * time.Second

	// Number of ICMP echo probes sent per host during packet loss tests.
	packetLossProbeCount = 10

	// Timeout per ICMP echo probe.
	packetLossProbeTimeout = time.Second

	// Hard cap on total TestPacketLoss runtime (2 targets × 10 probes × 1 s/probe + 5 s buffer).
	packetLossTestTimeout = 25 * time.Second

	// External IP used as a proxy for ISP connectivity. Cloudflare's anycast DNS is
	// positioned at ISP edge PoPs and responds reliably to ICMP.
	ispProbeTarget = "1.1.1.1"
)

// getDefaultGateway returns the IP address of the default gateway (router).
func getDefaultGateway(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return getDefaultGatewayLinux()
	case "darwin":
		return getDefaultGatewayDarwin(ctx)
	case "windows":
		return getDefaultGatewayWindows(ctx)
	default:
		return "", fmt.Errorf("unsupported OS for gateway detection: %s", runtime.GOOS)
	}
}

// getDefaultGatewayLinux parses /proc/net/route to find the default gateway.
func getDefaultGatewayLinux() (string, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", fmt.Errorf("reading /proc/net/route: %w", err)
	}
	return parseDefaultGatewayLinux(string(data))
}

// parseDefaultGatewayLinux extracts the default gateway IP from /proc/net/route content.
// When multiple default routes exist (e.g. VPN + physical NIC), the one with the lowest
// metric is the kernel's preferred route.
func parseDefaultGatewayLinux(data string) (string, error) {
	bestGW := ""
	bestMetric := -1
	for _, line := range strings.Split(data, "\n")[1:] {
		fields := strings.Fields(line)
		// Need at least 7 fields: Iface Destination Gateway Flags RefCnt Use Metric
		if len(fields) < 7 || fields[1] != "00000000" {
			continue
		}
		gwBytes, err := hex.DecodeString(fields[2])
		if err != nil || len(gwBytes) != 4 {
			continue
		}
		metric, err := strconv.Atoi(fields[6])
		if err != nil {
			continue
		}
		if bestMetric < 0 || metric < bestMetric {
			bestMetric = metric
			// Stored in little-endian byte order.
			bestGW = fmt.Sprintf("%d.%d.%d.%d", gwBytes[3], gwBytes[2], gwBytes[1], gwBytes[0])
		}
	}
	if bestGW == "" {
		return "", fmt.Errorf("default gateway not found in /proc/net/route")
	}
	return bestGW, nil
}

// getDefaultGatewayDarwin uses the route(8) command to find the default gateway on macOS.
func getDefaultGatewayDarwin(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("running route command: %w", err)
	}
	return parseDefaultGatewayDarwin(string(out))
}

// parseDefaultGatewayDarwin extracts the default gateway IP from `route -n get default` output.
func parseDefaultGatewayDarwin(data string) (string, error) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("gateway not found in route output")
}

// getDefaultGatewayWindows uses `route PRINT 0.0.0.0` to find the default gateway on Windows.
// The output contains rows like:
//
//	0.0.0.0   0.0.0.0   192.168.1.1   192.168.1.5   25
//
// where the fields are: destination, netmask, gateway, interface, metric.
func getDefaultGatewayWindows(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "route", "PRINT", "0.0.0.0").Output()
	if err != nil {
		return "", fmt.Errorf("running route PRINT: %w", err)
	}
	return parseDefaultGatewayWindows(string(out))
}

// parseDefaultGatewayWindows extracts the default gateway IP from `route PRINT 0.0.0.0` output.
// When multiple default routes exist, the one with the lowest metric is the preferred route.
func parseDefaultGatewayWindows(data string) (string, error) {
	bestGW := ""
	bestMetric := -1
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		// Need all 5 fields: destination, netmask, gateway, interface, metric.
		if len(fields) < 5 || fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" {
			continue
		}
		metric, err := strconv.Atoi(fields[4])
		if err != nil {
			continue
		}
		if bestMetric < 0 || metric < bestMetric {
			bestMetric = metric
			bestGW = fields[2]
		}
	}
	if bestGW == "" {
		return "", fmt.Errorf("default gateway not found in route PRINT output")
	}
	return bestGW, nil
}

// openICMPConn opens an ICMP packet connection. Tries privileged raw ICMP first
// ("ip4:icmp"), then falls back to unprivileged ping sockets ("udp4") which work
// on Linux when net.ipv4.ping_group_range is configured permissively.
// Returns the connection and the network string used.
func openICMPConn() (*icmp.PacketConn, string, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err == nil {
		return conn, "ip4:icmp", nil
	}

	conn, err = icmp.ListenPacket(udp4Network, "")
	if err == nil {
		return conn, udp4Network, nil
	}
	return nil, "", fmt.Errorf("failed to open ICMP socket (requires root or CAP_NET_RAW): %w", err)
}

// probePacketLoss sends count ICMP echo requests to target and returns a PacketLossResult.
// A separate connection per call prevents cross-test reply contamination.
func probePacketLoss(ctx context.Context, target string, count int) *PacketLossResult {
	result := &PacketLossResult{Target: target}

	targetIP, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		errStr := fmt.Sprintf("failed to resolve %q: %v", target, err)
		result.ErrorString = &errStr
		return result
	}

	conn, network, err := openICMPConn()
	if err != nil {
		errStr := err.Error()
		result.ErrorString = &errStr
		return result
	}

	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { conn.Close() }) } //nolint:errcheck
	defer closeConn()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		closeConn()
	}()

	isUnprivileged := network == udp4Network
	id := os.Getpid() & 0xffff
	var sent, received int
	var totalRTTMS int64

	for seq := 0; seq < count; seq++ {
		if ctx.Err() != nil {
			break
		}

		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("viam-rdk")},
		}
		wb, err := msg.Marshal(nil)
		if err != nil {
			continue
		}

		var dst net.Addr
		if isUnprivileged {
			dst = &net.UDPAddr{IP: targetIP.IP}
		} else {
			dst = &net.IPAddr{IP: targetIP.IP}
		}

		start := time.Now()
		if _, err := conn.WriteTo(wb, dst); err != nil {
			continue
		}
		sent++

		if err := conn.SetReadDeadline(time.Now().Add(packetLossProbeTimeout)); err != nil {
			continue
		}

		rb := make([]byte, 1500)
		for {
			n, _, err := conn.ReadFrom(rb)
			if err != nil {
				break // timeout or closed; probe counts as lost
			}

			msgBytes := rb[:n]
			if !isUnprivileged && n > 20 {
				// Raw socket includes the IP header; strip it.
				ihl := int(rb[0]&0x0f) * 4
				if ihl < n {
					msgBytes = rb[ihl:n]
				}
			}

			rm, err := icmp.ParseMessage(1 /* IPPROTO_ICMP */, msgBytes)
			if err != nil {
				continue
			}
			if rm.Type != ipv4.ICMPTypeEchoReply {
				continue
			}
			echo, ok := rm.Body.(*icmp.Echo)
			if !ok || echo.Seq != seq {
				continue
			}
			// For raw sockets we also match on ID; for unprivileged ("udp4") the
			// kernel rewrites the identifier so we skip that check.
			if !isUnprivileged && echo.ID != id {
				continue
			}
			received++
			totalRTTMS += time.Since(start).Milliseconds()
			break
		}
	}

	result.Sent = sent
	result.Received = received
	if received > 0 {
		avg := totalRTTMS / int64(received)
		result.AvgRTTMS = &avg
	}
	return result
}

// TestPacketLoss measures packet loss to the default gateway (router) and to
// ispProbeTarget as an indicator of ISP/WAN connectivity. If verbose is true,
// successful results are logged; otherwise only failures are logged.
func TestPacketLoss(ctx context.Context, logger logging.Logger, verbose bool) {
	ctx, cancel := context.WithTimeout(ctx, packetLossTestTimeout)
	defer cancel()

	type target struct{ ip, desc string }

	var targets []target
	gatewayIP, err := getDefaultGateway(ctx)
	if err != nil {
		logger.Warnw("Could not determine default gateway; skipping router packet loss test", "error", err)
	} else {
		targets = append(targets, target{gatewayIP, gatewayResultDescription})
	}
	targets = append(targets, target{ispProbeTarget, "ISP (" + ispProbeTarget + ")"})

	var results []*PacketLossResult
	for _, t := range targets {
		if ctx.Err() != nil {
			logger.Info("Shutdown detected; stopping packet loss tests")
			return
		}
		result := probePacketLoss(ctx, t.ip, packetLossProbeCount)
		result.Description = t.desc
		results = append(results, result)
	}

	logPacketLossResults(logger, results, verbose)
}

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
