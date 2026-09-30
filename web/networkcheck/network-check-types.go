package networkcheck

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"go.viam.com/rdk/logging"
)

// gatewayResultDescription is the Description value set on a PacketLossResult for the router probe.
const gatewayResultDescription = "router"

// ispHighLossPctThreshold is the packet loss percentage above which internet
// connectivity is described as spotty rather than merely lossy.
const ispHighLossPctThreshold = 50

// slowResolutionThresholdMS is the point above which a successful DNS
// resolution is counted as degraded.
const slowResolutionThresholdMS = 1000

// FamilyStatus is the health of a single family of network checks:
// DNS, UDP STUN, TCP STUN, or packet loss
type FamilyStatus string

// FamilyStatus values.
const (
	FamilyOK       FamilyStatus = "ok"
	FamilyDegraded FamilyStatus = "degraded"
	FamilyDown     FamilyStatus = "down"
	FamilyUnknown  FamilyStatus = "unknown"
)

// Verdict is the machine-wide rollup of every FamilyStatus. It has no "unknown"
// member; an unmeasured family does not contribute to the verdict.
type Verdict string

// Verdict values.
const (
	VerdictGood     Verdict = "good"
	VerdictDegraded Verdict = "degraded"
	VerdictDown     Verdict = "down"
)

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

// DNSSummary condenses a TestDNS run. The detail logger emits Results; the
// health line reads only the condensed fields.
type DNSSummary struct {
	Status                          FamilyStatus
	ConnectionsOK, ConnectionsTotal int
	ResolutionsOK, ResolutionsTotal int
	MaxResolutionMS                 *int64
	// Hostnames that resolved slower than slowResolutionThresholdMS, slowest first.
	SlowHostnames []string
	Results       []*DNSResult
}

// STUNSummary condenses a testUDP or testTCP run. HardNAT is only meaningful
// for UDP. The detail logger emits Results.
type STUNSummary struct {
	Status              FamilyStatus
	SuccessCount, Total int
	HardNAT             bool
	// Number of STUN responses carrying mapped address
	//  Comparing two is the minimum needed to classify the mapping
	MappedAddrSamples int
	Results           []*STUNResponse
}

// PacketLossSummary condenses a TestPacketLoss run. Pointer fields are nil when
// that target was not probed at all, e.g. no default gateway could be found.
// The detail logger emits Results.
type PacketLossSummary struct {
	InternetStatus, LocalNetworkStatus FamilyStatus
	RouterLossPct, ISPLossPct          *float64
	RouterRTTMS, ISPRTTMS              *int64
	RouterIgnoresPing                  bool
	Results                            []*PacketLossResult
}

// HealthSnapshot is one complete pass of every network check family.
type HealthSnapshot struct {
	DNS  DNSSummary
	UDP  STUNSummary
	TCP  STUNSummary
	Loss PacketLossSummary
}

// Verdict rolls the family statuses into the machine-wide answer. STUN failures
// are degraded, never down: the machine still reaches app over TCP, only
// peer-to-peer media is affected.
func (s HealthSnapshot) Verdict() Verdict {
	if s.Loss.InternetStatus == FamilyDown || s.DNS.Status == FamilyDown {
		return VerdictDown
	}
	for _, f := range []FamilyStatus{
		s.Loss.InternetStatus, s.Loss.LocalNetworkStatus,
		s.DNS.Status, s.UDP.Status, s.TCP.Status,
	} {
		if f == FamilyDegraded || f == FamilyDown {
			return VerdictDegraded
		}
	}
	return VerdictGood
}

// NATType reports the NAT behavior observed over UDP STUN.
func (s HealthSnapshot) NATType() string {
	switch {
	case s.UDP.SuccessCount == 0:
		return "unknown"
	case s.UDP.HardNAT:
		return "hard"
	case s.UDP.MappedAddrSamples < 2:
		return "unknown"
	default:
		return "endpoint-independent"
	}
}

type slowResolution struct {
	hostname string
	ms       int64
}

func summarizeDNS(results []*DNSResult) DNSSummary {
	s := DNSSummary{Results: results}
	var slow []slowResolution
	for _, r := range results {
		switch r.TestType {
		case ConnectionDNSTestType:
			s.ConnectionsTotal++
			if r.ErrorString == nil {
				s.ConnectionsOK++
			}
		case ResolutionDNSTestType:
			s.ResolutionsTotal++
			if r.ErrorString != nil {
				continue
			}
			s.ResolutionsOK++
			if r.ResolutionTimeMS == nil {
				continue
			}
			if s.MaxResolutionMS == nil || *r.ResolutionTimeMS > *s.MaxResolutionMS {
				s.MaxResolutionMS = r.ResolutionTimeMS
			}
			if *r.ResolutionTimeMS > slowResolutionThresholdMS && r.Hostname != nil {
				slow = append(slow, slowResolution{*r.Hostname, *r.ResolutionTimeMS})
			}
		}
	}

	// Slowest first, so consumers that report or truncate the list keep the
	// worst offender rather than whichever host happened to be probed first.
	slices.SortStableFunc(slow, func(a, b slowResolution) int {
		return cmp.Compare(b.ms, a.ms)
	})
	for _, sr := range slow {
		s.SlowHostnames = append(s.SlowHostnames, sr.hostname)
	}

	switch {
	case s.ConnectionsTotal == 0 && s.ResolutionsTotal == 0:
		s.Status = FamilyUnknown
	// Resolution is the signal that matters. Connection tests to public resolvers
	// fail on networks that force their own, while system resolution still works.
	case s.ResolutionsTotal > 0 && s.ResolutionsOK == 0:
		s.Status = FamilyDown
	case s.ResolutionsOK < s.ResolutionsTotal ||
		s.ConnectionsOK < s.ConnectionsTotal ||
		len(s.SlowHostnames) > 0:
		s.Status = FamilyDegraded
	default:
		s.Status = FamilyOK
	}
	return s
}

func summarizeSTUN(responses []*STUNResponse, network string) STUNSummary {
	s := STUNSummary{Total: len(responses), Results: responses}
	var expectedBindResponseAddr string

	for _, r := range responses {
		if r.ErrorString == nil {
			s.SuccessCount++
		}
		if r.BindResponseAddr == nil {
			continue
		}
		s.MappedAddrSamples++
		if expectedBindResponseAddr == "" {
			expectedBindResponseAddr = *r.BindResponseAddr
			continue
		}
		// A changing mapped address between servers means endpoint-dependent
		// mapping ("hard" NAT). Expected over TCP, where each bind uses a new
		// connection, so only UDP instability is meaningful.
		if network == "udp" && expectedBindResponseAddr != *r.BindResponseAddr {
			s.HardNAT = true
		}
	}

	switch {
	case s.Total == 0:
		s.Status = FamilyUnknown
	case s.SuccessCount == 0:
		s.Status = FamilyDown
	case s.SuccessCount < s.Total || s.HardNAT:
		s.Status = FamilyDegraded
	default:
		s.Status = FamilyOK
	}
	return s
}

func summarizePacketLoss(results []*PacketLossResult) PacketLossSummary {
	s := PacketLossSummary{Results: results}
	var routerErrored, ispErrored bool

	for _, r := range results {
		loss := r.LossPercent()
		if r.Description == gatewayResultDescription {
			s.RouterLossPct, s.RouterRTTMS = &loss, r.AvgRTTMS
			routerErrored = r.ErrorString != nil
			continue
		}
		s.ISPLossPct, s.ISPRTTMS = &loss, r.AvgRTTMS
		ispErrored = r.ErrorString != nil
	}

	// A gateway that drops ICMP while the ISP target replies is healthy, not
	// degraded: traffic to the ISP target routes through the gateway, so an ISP reply
	// proves it forwards fine. Many routers block ping by default.
	s.RouterIgnoresPing = s.RouterLossPct != nil && s.ISPLossPct != nil &&
		*s.RouterLossPct == 100 && *s.ISPLossPct == 0 && !routerErrored

	switch {
	case s.ISPLossPct == nil, ispErrored:
		s.InternetStatus = FamilyUnknown
	case *s.ISPLossPct == 100:
		s.InternetStatus = FamilyDown
	case *s.ISPLossPct > 0:
		s.InternetStatus = FamilyDegraded
	default:
		s.InternetStatus = FamilyOK
	}

	switch {
	case s.RouterLossPct == nil, routerErrored:
		s.LocalNetworkStatus = FamilyUnknown
	case s.RouterIgnoresPing, *s.RouterLossPct == 0:
		s.LocalNetworkStatus = FamilyOK
	default:
		s.LocalNetworkStatus = FamilyDegraded
	}
	return s
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

func logPacketLossResults(logger logging.Logger, s PacketLossSummary, verbose bool) {
	msg := "packet loss tests complete"
	keysAndValues := []any{"packet_loss_tests", stringifyPacketLossResults(s.Results)}

	switch {
	case s.RouterIgnoresPing:
		keysAndValues = append(keysAndValues,
			"note", "gateway is not responding to ICMP ping, but internet connectivity appears normal; many routers block ping by default",
		)
	case s.InternetStatus == FamilyDown:
		keysAndValues = append(keysAndValues,
			"note", "ISP target ("+ispProbeTarget+") is unreachable; internet connectivity may be down",
		)
	case s.InternetStatus == FamilyUnknown:
		keysAndValues = append(keysAndValues,
			"note", "ISP target ("+ispProbeTarget+") could not be measured; internet connectivity is unknown",
		)
	case s.ISPLossPct != nil && *s.ISPLossPct > ispHighLossPctThreshold:
		keysAndValues = append(keysAndValues,
			"note", "ISP target ("+ispProbeTarget+") has high packet loss; internet connectivity may be spotty",
		)
	}

	anyLoss := (s.ISPLossPct != nil && *s.ISPLossPct > 0) ||
		(s.RouterLossPct != nil && *s.RouterLossPct > 0)

	if anyLoss {
		logger.Warnw(msg, keysAndValues...)
	} else if verbose {
		logger.Infow(msg, keysAndValues...)
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
	s DNSSummary,
	resolvConfContents string,
	systemdResolvedConfContents string,
	verbose bool,
) {
	systemMsg := fmt.Sprintf(
		"%d/%d dns connection and %d/%d dns resolution tests succeeded",
		s.ConnectionsOK,
		s.ConnectionsTotal,
		s.ResolutionsOK,
		s.ResolutionsTotal,
	)
	keysAndValues := []any{"dns_tests", stringifyDNSResults(s.Results)}

	if s.ConnectionsOK < s.ConnectionsTotal || s.ResolutionsOK < s.ResolutionsTotal {
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

	if len(s.SlowHostnames) > 0 {
		logger.Warnw(
			fmt.Sprintf("Slow DNS resolutions detected (>%dms)", slowResolutionThresholdMS),
			"slow_hostnames", strings.Join(s.SlowHostnames, ", "),
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
	s STUNSummary,
	udpSourceAddress,
	network string,
	verbose bool,
) {
	msg := fmt.Sprintf(
		"%d/%d %v STUN tests succeeded",
		s.SuccessCount,
		s.Total,
		network,
	)
	keysAndValues := []any{fmt.Sprintf("%v_tests", network), stringifySTUNResponses(s.Results)}
	if network == "udp" {
		keysAndValues = append(keysAndValues, "udp_source_address", udpSourceAddress)
	}
	if s.SuccessCount < s.Total {
		logger.Warnw(msg, keysAndValues...)
	} else if verbose {
		logger.Infow(msg, keysAndValues...)
	}

	if s.HardNAT {
		logger.Warn(
			"udp STUN tests indicate this machine is behind a 'hard' NAT device; STUN may not work as expected",
		)
	}
}

// logHealth emits the consolidated periodic verdict line. Always Info: severity
// is carried in the verdict field, since the line is a heartbeat, not an event.
func logHealth(logger logging.Logger, s HealthSnapshot) {
	keysAndValues := []any{
		"verdict", string(s.Verdict()),

		"dns_status", string(s.DNS.Status),
		"dns_connections_ok", s.DNS.ConnectionsOK,
		"dns_connections_total", s.DNS.ConnectionsTotal,
		"dns_resolutions_ok", s.DNS.ResolutionsOK,
		"dns_resolutions_total", s.DNS.ResolutionsTotal,

		"udp_status", string(s.UDP.Status),
		"udp_stun_ok", s.UDP.SuccessCount,
		"udp_stun_total", s.UDP.Total,

		"tcp_status", string(s.TCP.Status),
		"tcp_stun_ok", s.TCP.SuccessCount,
		"tcp_stun_total", s.TCP.Total,

		"nat_type", s.NATType(),

		"internet_status", string(s.Loss.InternetStatus),
		"local_network_status", string(s.Loss.LocalNetworkStatus),
		"router_ignores_ping", s.Loss.RouterIgnoresPing,
	}

	if s.DNS.MaxResolutionMS != nil {
		keysAndValues = append(keysAndValues, "dns_max_resolve_ms", *s.DNS.MaxResolutionMS)
	}
	if len(s.DNS.SlowHostnames) > 0 {
		keysAndValues = append(keysAndValues, "dns_slow_hostnames", strings.Join(s.DNS.SlowHostnames, ","))
	}
	if s.Loss.ISPLossPct != nil {
		keysAndValues = append(keysAndValues, "isp_loss_pct", *s.Loss.ISPLossPct)
	}
	if s.Loss.ISPRTTMS != nil {
		keysAndValues = append(keysAndValues, "isp_rtt_ms", *s.Loss.ISPRTTMS)
	}
	if s.Loss.RouterLossPct != nil {
		keysAndValues = append(keysAndValues, "router_loss_pct", *s.Loss.RouterLossPct)
	}
	if s.Loss.RouterRTTMS != nil {
		keysAndValues = append(keysAndValues, "router_rtt_ms", *s.Loss.RouterRTTMS)
	}

	logger.Infow("network health", keysAndValues...)
}
