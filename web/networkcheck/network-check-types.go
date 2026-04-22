package networkcheck

import (
	"fmt"
	"strings"

	"go.viam.com/rdk/logging"
)

// gatewayResultDescription is the Description value set on a PacketLossResult for the router probe.
const gatewayResultDescription = "router"

// PacketLossResult holds the results of a packet loss probe to a specific host.
type PacketLossResult struct {
	// Target is the IP address being probed.
	Target string

	// Description describes the role of the target (e.g., "router", "ISP").
	Description string

	// Sent is the number of ICMP echo probes sent.
	Sent int

	// Received is the number of ICMP echo replies received.
	Received int

	// AvgRTTMS is the average round-trip time in milliseconds across received replies.
	// Nil if no replies were received.
	AvgRTTMS *int64

	// ErrorString is set if the test could not be initialized or completed.
	ErrorString *string
}

// LossPercent returns the percentage of probes that were lost.
func (r *PacketLossResult) LossPercent() float64 {
	if r.Sent == 0 {
		return 100.0
	}
	return float64(r.Sent-r.Received) / float64(r.Sent) * 100.0
}

func stringifyPacketLossResults(results []*PacketLossResult) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, r := range results {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, "{target: %s, description: %s, sent: %d, received: %d, loss_pct: %.0f%%",
			r.Target, r.Description, r.Sent, r.Received, r.LossPercent())
		if r.AvgRTTMS != nil {
			fmt.Fprintf(&sb, ", avg_rtt_ms: %d", *r.AvgRTTMS)
		}
		if r.ErrorString != nil {
			fmt.Fprintf(&sb, ", error: %s", *r.ErrorString)
		}
		sb.WriteString("}")
	}
	sb.WriteString("]")
	return sb.String()
}

func logPacketLossResults(logger logging.Logger, results []*PacketLossResult, verbose bool) {
	var anyLoss bool
	for _, r := range results {
		if r.ErrorString != nil || r.LossPercent() > 0 {
			anyLoss = true
			break
		}
	}

	// If the router has 100% packet loss but the ISP target is reachable, note that the
	// gateway is still routing traffic correctly — many routers drop ICMP ping by default.
	var routerFullLoss, ispReachable, ispHighLoss, ispFullLoss bool
	for _, r := range results {
		if r.Description == gatewayResultDescription && r.LossPercent() == 100 && r.ErrorString == nil {
			routerFullLoss = true
		}
		if r.Description != gatewayResultDescription {
			if r.LossPercent() == 0 {
				ispReachable = true
			}
			if r.LossPercent() > 50 && r.LossPercent() < 100 {
				ispHighLoss = true
			}
			if r.LossPercent() == 100 || r.ErrorString != nil {
				ispFullLoss = true
			}
		}
	}

	msg := "packet loss tests complete"
	keysAndValues := []any{"packet_loss_tests", stringifyPacketLossResults(results)}
	if routerFullLoss && ispReachable {
		keysAndValues = append(keysAndValues,
			"note", "gateway is not responding to ICMP ping, but internet connectivity appears normal; many routers block ping by default",
		)
	}
	if ispHighLoss {
		keysAndValues = append(keysAndValues,
			"note", "ISP target (1.1.1.1) has high packet loss; internet connectivity may be spotty",
		)
	}
	if ispFullLoss {
		keysAndValues = append(keysAndValues,
			"note", "ISP target (1.1.1.1) is unreachable; internet connectivity may be down",
		)
	}
	if anyLoss {
		logger.Warnw(msg, keysAndValues...)
	} else if verbose {
		logger.Infow(msg, keysAndValues...)
	}
}

type (
	// DNSTestType is an enumeration of test types.
	DNSTestType int

	// STUNResponse represents a response from a STUN server.
	STUNResponse struct {
		// STUNServerURL is the URL of the STUN server.
		STUNServerURL string

		// TCPSourceAddress is the source address for the bind request if this was a TCP test.
		// If it was a UDP test, it will be the same UDP source address for all UDP tests, and
		// that value will be passed to `logSTUNResults`.
		TCPSourceAddress *string

		// STUNServerAddr is the resolved address of the STUN server.
		STUNServerAddr *string

		// BindResponseAddr is our address as reported by the STUN server.
		BindResponseAddr *string

		// Time taken to send bind request, receive bind response, and extract address. A vague
		// measurement of RTT to the STUN server.
		TimeToBindResponseMS *int64

		// Any error received during STUN interactions.
		ErrorString *string
	}

	// DNSResult represents the result of a DNS resolution test.
	DNSResult struct {
		// TestType indicates the type of DNS test.
		TestType DNSTestType

		// Any error encountered during the test.
		ErrorString *string

		/* Fields populated in Connection tests below */

		// DNS server being tested.
		DNSServer *string

		// Time taken to connect to DNS server.
		ConnectTimeMS *int64

		// Time taken to send query and receive response.
		QueryTimeMS *int64

		// Size of DNS response in bytes.
		ResponseSize *int64

		/* Fields populated in Resolution tests below */

		// Hostname being resolved.
		Hostname *string

		// Resolved IP addresses (comma-separated).
		ResolvedIPs *string

		// Time taken to resolve the hostname.
		ResolutionTimeMS *int64
	}
)

const (
	// ConnectionDNSTestType is a DNS connection test.
	ConnectionDNSTestType DNSTestType = iota
	// ResolutionDNSTestType is a DNS resolution test.
	ResolutionDNSTestType
)

// String stringifies a DNS test type.
func (dtt DNSTestType) String() string {
	switch dtt {
	case ConnectionDNSTestType:
		return "connection"
	case ResolutionDNSTestType:
		return "resolution"
	default:
		return "unknown"
	}
}

func stringifyDNSResults(dnsResults []*DNSResult) string {
	ret := "["

	for i, dr := range dnsResults {
		comma := ","
		if i == 0 {
			comma = ""
		}

		ret += fmt.Sprintf("%v{test_type: %s", comma, dr.TestType)
		if dr.ErrorString != nil {
			ret += fmt.Sprintf(", error_string: %v", *dr.ErrorString)
		}

		// Connection fields.
		if dr.DNSServer != nil {
			ret += fmt.Sprintf(", dns_server: %v", *dr.DNSServer)
		}
		if dr.ConnectTimeMS != nil {
			ret += fmt.Sprintf(", connect_time_ms: %d", *dr.ConnectTimeMS)
		}
		if dr.QueryTimeMS != nil {
			ret += fmt.Sprintf(", query_time_ms: %d", *dr.QueryTimeMS)
		}
		if dr.ResponseSize != nil {
			ret += fmt.Sprintf(", response_size: %d", *dr.ResponseSize)
		}

		// Resolution fields.
		if dr.Hostname != nil {
			ret += fmt.Sprintf(", hostname: %v", *dr.Hostname)
		}
		if dr.ResolutionTimeMS != nil {
			ret += fmt.Sprintf(", resolution_time_ms: %d", *dr.ResolutionTimeMS)
		}
		if dr.ResolvedIPs != nil {
			ret += fmt.Sprintf(", resolved_ips: %v", *dr.ResolvedIPs)
		}

		ret += "}"
	}

	return ret + "]"
}

// Logs DNS test results.
func logDNSResults(
	logger logging.Logger,
	dnsResults []*DNSResult,
	resolvConfContents string,
	systemdResolvedConfContents string,
	verbose bool,
) {
	var successfulConnectionTests, totalConnectionTests int
	var successfulResolutionTests, totalResolutionTests int
	var slowResolutions []string

	for _, dr := range dnsResults {
		switch dr.TestType {
		case ConnectionDNSTestType:
			totalConnectionTests++
			if dr.ErrorString == nil {
				successfulConnectionTests++
			}
		case ResolutionDNSTestType:
			totalResolutionTests++
			if dr.ErrorString == nil {
				successfulResolutionTests++
				// Flag slow DNS resolutions (>1s).
				if dr.ResolutionTimeMS != nil && *dr.ResolutionTimeMS > 1000 {
					if dr.Hostname != nil /* should be non-nil */ {
						slowResolutions = append(slowResolutions, *dr.Hostname)
					}
				}
			}
		default:
			logger.Warnf("Unknown DNS test type; cannot handle %s", dr.TestType)
		}
	}

	systemMsg := fmt.Sprintf(
		"%d/%d dns connection and %d/%d dns resolution tests succeeded",
		successfulConnectionTests,
		totalConnectionTests,
		successfulResolutionTests,
		totalResolutionTests,
	)
	keysAndValues := []any{"dns_tests", stringifyDNSResults(dnsResults)}

	if successfulConnectionTests < totalConnectionTests ||
		successfulResolutionTests < totalResolutionTests {
		logger.Warnw(systemMsg, keysAndValues...)
		// Only log `/etc/resolv.conf` and `/etc/systemd/resolved.conf` contents in the event
		// of a DNS test failure.
		if resolvConfContents != "" {
			logger.Infof("/etc/resolv.conf contents: %s", resolvConfContents)
		}
		if systemdResolvedConfContents != "" {
			logger.Infof("/etc/systemd/resolved.conf contents: %s", systemdResolvedConfContents)
		}
	} else if verbose {
		logger.Infow(systemMsg, keysAndValues...)
	}

	// Warn about slow DNS resolutions
	if len(slowResolutions) > 0 {
		logger.Warnw(
			"Slow DNS resolutions detected (>1000ms)",
			"slow_hostnames", strings.Join(slowResolutions, ", "),
		)
	}
}

func stringifySTUNResponses(stunResponses []*STUNResponse) string {
	ret := "["

	for i, sr := range stunResponses {
		comma := ","
		if i == 0 {
			comma = ""
		}

		ret += fmt.Sprintf("%v{stun_server_url: %v", comma, sr.STUNServerURL)
		if sr.TCPSourceAddress != nil {
			ret += fmt.Sprintf(", tcp_source_address: %v", *sr.TCPSourceAddress)
		}
		if sr.STUNServerAddr != nil {
			ret += fmt.Sprintf(", stun_server_addr: %v", *sr.STUNServerAddr)
		}
		if sr.BindResponseAddr != nil {
			ret += fmt.Sprintf(", bind_response_addr: %v", *sr.BindResponseAddr)
		}
		if sr.TimeToBindResponseMS != nil {
			ret += fmt.Sprintf(", time_to_bind_response_ms: %d", *sr.TimeToBindResponseMS)
		}
		if sr.ErrorString != nil {
			ret += fmt.Sprintf(", error_string: %v", *sr.ErrorString)
		}

		ret += "}"
	}

	return ret + "]"
}

// Logs STUN responses and whether the machine appears to be behind a "hard" NAT device.
func logSTUNResults(
	logger logging.Logger,
	stunResponses []*STUNResponse,
	udpSourceAddress,
	network string,
) {
	// Use lastBindResponseAddr to track whether the received address from STUN servers is
	// "unstable." Any changes in port, in particular, between different STUN server's bind
	// responses indicates that we may be behind an endpoint-dependent-mapping NAT device
	// ("hard" NAT). Changes in port are expected for TCP tests, so do not log any warning
	// for them.
	var expectedBindResponseAddr string
	var unstableBindResponseAddr bool

	var successfulStunResponses int
	for _, sr := range stunResponses {
		if sr.ErrorString == nil {
			successfulStunResponses++
		}

		if sr.BindResponseAddr != nil {
			if expectedBindResponseAddr == "" {
				// Take first bind response address as "expected"; all others should match when
				// behind an endpoint-independent-mapping NAT device.
				expectedBindResponseAddr = *sr.BindResponseAddr
			} else if expectedBindResponseAddr != *sr.BindResponseAddr {
				unstableBindResponseAddr = true
			}
		}
	}

	msg := fmt.Sprintf(
		"%d/%d %v STUN tests succeeded",
		successfulStunResponses,
		len(stunResponses),
		network,
	)
	keysAndValues := []any{fmt.Sprintf("%v_tests", network), stringifySTUNResponses(stunResponses)}
	if network == "udp" {
		keysAndValues = append(keysAndValues, "udp_source_address", udpSourceAddress)
	}
	if successfulStunResponses < len(stunResponses) {
		logger.Warnw(msg, keysAndValues...)
	} else {
		logger.Infow(msg, keysAndValues...)
	}

	if unstableBindResponseAddr && network != "tcp" /* do not warn about instability for TCP tests */ {
		logger.Warn(
			"udp STUN tests indicate this machine is behind a 'hard' NAT device; STUN may not work as expected",
		)
	}
}
