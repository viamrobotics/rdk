package networkcheck

import (
	"cmp"
	"slices"
	"strings"

	"go.viam.com/rdk/logging"
)

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

// DNSSummary condenses a TestDNS run.
type DNSSummary struct {
	Status                          FamilyStatus
	ConnectionsOK, ConnectionsTotal int
	ResolutionsOK, ResolutionsTotal int
	MaxResolutionMS                 *int64
	// Hostnames that resolved slower than slowResolutionThresholdMS, slowest first.
	SlowHostnames []string
}

// STUNSummary condenses a testUDP or testTCP run. HardNAT is only meaningful
// for UDP.
type STUNSummary struct {
	Status              FamilyStatus
	SuccessCount, Total int
	HardNAT             bool
}

// PacketLossSummary condenses a TestPacketLoss run. Pointer fields are nil when
// that target was not probed at all, e.g. no default gateway could be found.
type PacketLossSummary struct {
	InternetStatus, LocalNetworkStatus FamilyStatus
	RouterLossPct, ISPLossPct          *float64
	RouterRTTMS, ISPRTTMS              *int64
	RouterIgnoresPing                  bool
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
	default:
		return "endpoint-independent"
	}
}

type slowResolution struct {
	hostname string
	ms       int64
}

func summarizeDNS(results []*DNSResult) DNSSummary {
	var s DNSSummary
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
	s := STUNSummary{Total: len(responses)}
	var expectedBindResponseAddr string

	for _, r := range responses {
		if r.ErrorString == nil {
			s.SuccessCount++
		}
		if r.BindResponseAddr == nil {
			continue
		}
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
	var s PacketLossSummary
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
