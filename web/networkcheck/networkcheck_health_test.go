package networkcheck

import (
	"testing"

	"go.viam.com/test"
)

func ptr[T any](v T) *T { return &v }

func TestVerdict(t *testing.T) {
	healthy := HealthSnapshot{
		DNS:  DNSSummary{Status: FamilyOK},
		UDP:  STUNSummary{Status: FamilyOK},
		TCP:  STUNSummary{Status: FamilyOK},
		Loss: PacketLossSummary{InternetStatus: FamilyOK, LocalNetworkStatus: FamilyOK},
	}

	t.Run("all healthy is good", func(t *testing.T) {
		test.That(t, healthy.Verdict(), test.ShouldEqual, VerdictGood)
	})

	t.Run("internet down is down", func(t *testing.T) {
		s := healthy
		s.Loss.InternetStatus = FamilyDown
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictDown)
	})

	t.Run("dns down is down", func(t *testing.T) {
		s := healthy
		s.DNS.Status = FamilyDown
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictDown)
	})

	// The machine still reaches app over TCP when STUN fails; only peer-to-peer
	// media is affected. Reporting "down" here would tell a user their working
	// machine is offline.
	t.Run("udp down is only degraded", func(t *testing.T) {
		s := healthy
		s.UDP.Status = FamilyDown
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictDegraded)
	})

	t.Run("dns degraded is degraded", func(t *testing.T) {
		s := healthy
		s.DNS.Status = FamilyDegraded
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictDegraded)
	})

	// A machine with no CAP_NET_RAW cannot ping at all, so both packet loss
	// probes error. DNS and STUN need no raw sockets and still report, so the
	// verdict stays honest rather than falsely down.
	t.Run("packet loss unknown does not force down", func(t *testing.T) {
		s := healthy
		s.Loss.InternetStatus = FamilyUnknown
		s.Loss.LocalNetworkStatus = FamilyUnknown
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictGood)
	})

	t.Run("all unknown is good", func(t *testing.T) {
		s := HealthSnapshot{
			DNS:  DNSSummary{Status: FamilyUnknown},
			UDP:  STUNSummary{Status: FamilyUnknown},
			TCP:  STUNSummary{Status: FamilyUnknown},
			Loss: PacketLossSummary{InternetStatus: FamilyUnknown, LocalNetworkStatus: FamilyUnknown},
		}
		test.That(t, s.Verdict(), test.ShouldEqual, VerdictGood)
	})
}

func TestNATType(t *testing.T) {
	t.Run("no successful responses is unknown", func(t *testing.T) {
		s := HealthSnapshot{UDP: STUNSummary{SuccessCount: 0, Total: 7}}
		test.That(t, s.NATType(), test.ShouldEqual, "unknown")
	})

	t.Run("hard nat", func(t *testing.T) {
		s := HealthSnapshot{UDP: STUNSummary{SuccessCount: 7, Total: 7, HardNAT: true}}
		test.That(t, s.NATType(), test.ShouldEqual, "hard")
	})

	t.Run("endpoint independent", func(t *testing.T) {
		s := HealthSnapshot{UDP: STUNSummary{SuccessCount: 7, Total: 7}}
		test.That(t, s.NATType(), test.ShouldEqual, "endpoint-independent")
	})

	// Without any successful response there is no mapped address to compare, so
	// unknown wins over a HardNAT flag that cannot have been set meaningfully.
	t.Run("no responses wins over hard nat flag", func(t *testing.T) {
		s := HealthSnapshot{UDP: STUNSummary{SuccessCount: 0, Total: 7, HardNAT: true}}
		test.That(t, s.NATType(), test.ShouldEqual, "unknown")
	})
}

func TestSummarizePacketLoss(t *testing.T) {
	router := func(sent, received int) *PacketLossResult {
		return &PacketLossResult{
			Target: "10.0.0.1", Description: gatewayResultDescription,
			Sent: sent, Received: received, AvgRTTMS: ptr(int64(3)),
		}
	}
	isp := func(sent, received int) *PacketLossResult {
		return &PacketLossResult{
			Target: ispProbeTarget, Description: "ISP (" + ispProbeTarget + ")",
			Sent: sent, Received: received, AvgRTTMS: ptr(int64(13)),
		}
	}
	errored := func(description string) *PacketLossResult {
		return &PacketLossResult{Description: description, ErrorString: ptr("no raw socket")}
	}

	t.Run("both healthy", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 10), isp(10, 10)})
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyOK)
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyOK)
		test.That(t, s.RouterIgnoresPing, test.ShouldBeFalse)
		test.That(t, *s.RouterLossPct, test.ShouldEqual, 0.0)
		test.That(t, *s.ISPLossPct, test.ShouldEqual, 0.0)
		test.That(t, *s.RouterRTTMS, test.ShouldEqual, int64(3))
		test.That(t, *s.ISPRTTMS, test.ShouldEqual, int64(13))
	})

	// Traffic to the ISP target routes through the gateway, so an ISP reply
	// proves the gateway forwards fine even when it ignores pings itself.
	t.Run("router drops ping but isp replies is healthy", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 0), isp(10, 10)})
		test.That(t, s.RouterIgnoresPing, test.ShouldBeTrue)
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyOK)
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyOK)
	})

	t.Run("isp full loss is down", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 10), isp(10, 0)})
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyDown)
		test.That(t, *s.ISPLossPct, test.ShouldEqual, 100.0)
	})

	t.Run("isp partial loss is degraded", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 10), isp(10, 7)})
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyDegraded)
		test.That(t, *s.ISPLossPct, test.ShouldEqual, 30.0)
	})

	t.Run("router partial loss is degraded", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 7), isp(10, 10)})
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyDegraded)
		test.That(t, s.RouterIgnoresPing, test.ShouldBeFalse)
	})

	// "The probe could not run" is not "every packet was lost". Without this
	// distinction a machine lacking CAP_NET_RAW reports its internet as down.
	t.Run("errored isp probe is unknown not down", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{router(10, 10), errored("ISP (1.1.1.1)")})
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyOK)
	})

	t.Run("errored router probe is unknown", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{errored(gatewayResultDescription), isp(10, 10)})
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.RouterIgnoresPing, test.ShouldBeFalse)
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyOK)
	})

	t.Run("both probes errored", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{
			errored(gatewayResultDescription), errored("ISP (1.1.1.1)"),
		})
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.RouterIgnoresPing, test.ShouldBeFalse)
	})

	t.Run("no gateway discovered leaves router fields nil", func(t *testing.T) {
		s := summarizePacketLoss([]*PacketLossResult{isp(10, 10)})
		test.That(t, s.RouterLossPct, test.ShouldBeNil)
		test.That(t, s.RouterRTTMS, test.ShouldBeNil)
		test.That(t, s.LocalNetworkStatus, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.InternetStatus, test.ShouldEqual, FamilyOK)
	})
}

func TestSummarizeDNS(t *testing.T) {
	conn := func(errString *string) *DNSResult {
		return &DNSResult{TestType: ConnectionDNSTestType, DNSServer: ptr("1.1.1.1:53"), ErrorString: errString}
	}
	resolve := func(hostname string, ms int64, errString *string) *DNSResult {
		return &DNSResult{
			TestType: ResolutionDNSTestType, Hostname: ptr(hostname),
			ResolutionTimeMS: ptr(ms), ErrorString: errString,
		}
	}

	t.Run("all healthy", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{conn(nil), conn(nil), resolve("viam.com", 20, nil)})
		test.That(t, s.Status, test.ShouldEqual, FamilyOK)
		test.That(t, s.ConnectionsOK, test.ShouldEqual, 2)
		test.That(t, s.ConnectionsTotal, test.ShouldEqual, 2)
		test.That(t, s.ResolutionsOK, test.ShouldEqual, 1)
		test.That(t, *s.MaxResolutionMS, test.ShouldEqual, int64(20))
		test.That(t, s.SlowHostnames, test.ShouldBeEmpty)
	})

	// Connection tests to public resolvers fail on networks that force their own
	// resolver, while system resolution still works. Resolution is the signal
	// that matters, so a connection failure alone must not read as down.
	t.Run("connection failures with working resolution are degraded", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{conn(ptr("refused")), conn(nil), resolve("viam.com", 20, nil)})
		test.That(t, s.Status, test.ShouldEqual, FamilyDegraded)
		test.That(t, s.ConnectionsOK, test.ShouldEqual, 1)
	})

	t.Run("no resolutions succeeded is down", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{conn(nil), resolve("viam.com", 0, ptr("timeout"))})
		test.That(t, s.Status, test.ShouldEqual, FamilyDown)
		test.That(t, s.ResolutionsOK, test.ShouldEqual, 0)
		test.That(t, s.ResolutionsTotal, test.ShouldEqual, 1)
	})

	t.Run("slow resolution is degraded and recorded", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{
			resolve("fast.com", 20, nil),
			resolve("slow.com", slowResolutionThresholdMS+1, nil),
		})
		test.That(t, s.Status, test.ShouldEqual, FamilyDegraded)
		test.That(t, s.SlowHostnames, test.ShouldResemble, []string{"slow.com"})
		test.That(t, *s.MaxResolutionMS, test.ShouldEqual, int64(slowResolutionThresholdMS+1))
	})

	t.Run("threshold is exclusive", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{resolve("edge.com", slowResolutionThresholdMS, nil)})
		test.That(t, s.Status, test.ShouldEqual, FamilyOK)
		test.That(t, s.SlowHostnames, test.ShouldBeEmpty)
	})

	// Timing is only used for the slow check. A resolution that succeeded
	// without a recorded duration still counts as a success.
	t.Run("successful resolution with no timing still counts", func(t *testing.T) {
		s := summarizeDNS([]*DNSResult{{TestType: ResolutionDNSTestType, Hostname: ptr("viam.com")}})
		test.That(t, s.Status, test.ShouldEqual, FamilyOK)
		test.That(t, s.ResolutionsOK, test.ShouldEqual, 1)
		test.That(t, s.MaxResolutionMS, test.ShouldBeNil)
	})

	t.Run("no tests is unknown", func(t *testing.T) {
		s := summarizeDNS(nil)
		test.That(t, s.Status, test.ShouldEqual, FamilyUnknown)
	})
}

func TestSummarizeSTUN(t *testing.T) {
	ok := func(url, addr string) *STUNResponse {
		return &STUNResponse{STUNServerURL: url, BindResponseAddr: ptr(addr)}
	}
	failed := func(url string) *STUNResponse {
		return &STUNResponse{STUNServerURL: url, ErrorString: ptr("timeout")}
	}

	t.Run("all succeed with stable address", func(t *testing.T) {
		s := summarizeSTUN([]*STUNResponse{
			ok("a:3478", "71.0.0.1:52757"), ok("b:3478", "71.0.0.1:52757"),
		}, "udp")
		test.That(t, s.Status, test.ShouldEqual, FamilyOK)
		test.That(t, s.SuccessCount, test.ShouldEqual, 2)
		test.That(t, s.Total, test.ShouldEqual, 2)
		test.That(t, s.HardNAT, test.ShouldBeFalse)
	})

	t.Run("changing address over udp is hard nat", func(t *testing.T) {
		s := summarizeSTUN([]*STUNResponse{
			ok("a:3478", "71.0.0.1:52757"), ok("b:3478", "71.0.0.1:60001"),
		}, "udp")
		test.That(t, s.HardNAT, test.ShouldBeTrue)
		test.That(t, s.Status, test.ShouldEqual, FamilyDegraded)
	})

	// Each TCP bind uses a new connection, so a changing mapped address is
	// expected there and says nothing about NAT behavior.
	t.Run("changing address over tcp is not hard nat", func(t *testing.T) {
		s := summarizeSTUN([]*STUNResponse{
			ok("a:443", "71.0.0.1:59419"), ok("b:3478", "71.0.0.1:59420"),
		}, "tcp")
		test.That(t, s.HardNAT, test.ShouldBeFalse)
		test.That(t, s.Status, test.ShouldEqual, FamilyOK)
	})

	t.Run("partial success is degraded", func(t *testing.T) {
		s := summarizeSTUN([]*STUNResponse{ok("a:3478", "71.0.0.1:52757"), failed("b:3478")}, "udp")
		test.That(t, s.Status, test.ShouldEqual, FamilyDegraded)
		test.That(t, s.SuccessCount, test.ShouldEqual, 1)
		test.That(t, s.Total, test.ShouldEqual, 2)
	})

	t.Run("all fail is down", func(t *testing.T) {
		s := summarizeSTUN([]*STUNResponse{failed("a:3478"), failed("b:3478")}, "udp")
		test.That(t, s.Status, test.ShouldEqual, FamilyDown)
		test.That(t, s.SuccessCount, test.ShouldEqual, 0)
	})

	t.Run("no responses is unknown", func(t *testing.T) {
		s := summarizeSTUN(nil, "udp")
		test.That(t, s.Status, test.ShouldEqual, FamilyUnknown)
		test.That(t, s.Total, test.ShouldEqual, 0)
	})
}
