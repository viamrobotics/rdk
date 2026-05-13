package networkcheck

import (
	"testing"

	"go.viam.com/test"
)

func TestLossPercent(t *testing.T) {
	t.Run("zero sent returns 100", func(t *testing.T) {
		r := &PacketLossResult{Sent: 0, Received: 0}
		test.That(t, r.LossPercent(), test.ShouldEqual, 100.0)
	})
	t.Run("no loss", func(t *testing.T) {
		r := &PacketLossResult{Sent: 10, Received: 10}
		test.That(t, r.LossPercent(), test.ShouldEqual, 0.0)
	})
	t.Run("full loss", func(t *testing.T) {
		r := &PacketLossResult{Sent: 10, Received: 0}
		test.That(t, r.LossPercent(), test.ShouldEqual, 100.0)
	})
	t.Run("partial loss", func(t *testing.T) {
		r := &PacketLossResult{Sent: 10, Received: 7}
		test.That(t, r.LossPercent(), test.ShouldEqual, 30.0)
	})
}

func TestStringifyPacketLossResults(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		test.That(t, stringifyPacketLossResults(nil), test.ShouldEqual, "[]")
	})

	t.Run("single result", func(t *testing.T) {
		r := &PacketLossResult{Target: "1.2.3.4", Description: "router", Sent: 10, Received: 10}
		got := stringifyPacketLossResults([]*PacketLossResult{r})
		test.That(t, got, test.ShouldEqual,
			"[{target: 1.2.3.4, description: router, sent: 10, received: 10, loss_pct: 0%}]")
	})

	t.Run("with avg rtt", func(t *testing.T) {
		rtt := int64(5)
		r := &PacketLossResult{Target: "1.2.3.4", Description: "router", Sent: 10, Received: 10, AvgRTTMS: &rtt}
		got := stringifyPacketLossResults([]*PacketLossResult{r})
		test.That(t, got, test.ShouldEqual,
			"[{target: 1.2.3.4, description: router, sent: 10, received: 10, loss_pct: 0%, avg_rtt_ms: 5}]")
	})

	t.Run("with error", func(t *testing.T) {
		errStr := "connect failed"
		r := &PacketLossResult{Target: "1.2.3.4", Description: "router", Sent: 0, Received: 0, ErrorString: &errStr}
		got := stringifyPacketLossResults([]*PacketLossResult{r})
		test.That(t, got, test.ShouldContainSubstring, "error: connect failed")
	})

	t.Run("multiple results joined with comma", func(t *testing.T) {
		r1 := &PacketLossResult{Target: "192.168.1.1", Description: "router", Sent: 10, Received: 10}
		r2 := &PacketLossResult{Target: "1.1.1.1", Description: "ISP", Sent: 10, Received: 8}
		got := stringifyPacketLossResults([]*PacketLossResult{r1, r2})
		test.That(t, got, test.ShouldContainSubstring, "192.168.1.1")
		test.That(t, got, test.ShouldContainSubstring, "1.1.1.1")
		// Exactly one comma between the two entries (not at start).
		test.That(t, got[:1], test.ShouldEqual, "[")
		test.That(t, got, test.ShouldContainSubstring, "},")
	})
}

func TestParseDefaultGatewayDarwin(t *testing.T) {
	t.Run("parses gateway from normal output", func(t *testing.T) {
		data := "   route to: default\n" +
			"destination: default\n" +
			"       mask: default\n" +
			"    gateway: 192.168.1.1\n" +
			"  interface: en0\n"
		gw, err := parseDefaultGatewayDarwin(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "192.168.1.1")
	})

	t.Run("returns first gateway when multiple present", func(t *testing.T) {
		data := "    gateway: 10.0.0.1\n" +
			"    gateway: 10.0.0.2\n"
		gw, err := parseDefaultGatewayDarwin(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "10.0.0.1")
	})

	t.Run("no gateway line returns error", func(t *testing.T) {
		data := "   route to: default\n" +
			"destination: default\n" +
			"  interface: en0\n"
		_, err := parseDefaultGatewayDarwin(data)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := parseDefaultGatewayDarwin("")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("gateway line with no value returns error", func(t *testing.T) {
		data := "    gateway:\n"
		_, err := parseDefaultGatewayDarwin(data)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestParseDefaultGatewayWindows(t *testing.T) {
	t.Run("parses gateway from normal output", func(t *testing.T) {
		data := "===========================================================================\n" +
			"Active Routes:\n" +
			"Network Destination        Netmask          Gateway       Interface  Metric\n" +
			"          0.0.0.0          0.0.0.0      192.168.1.1    192.168.1.5      25\n" +
			"        127.0.0.0        255.0.0.0        127.0.0.1      127.0.0.1     331\n"
		gw, err := parseDefaultGatewayWindows(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "192.168.1.1")
	})

	t.Run("picks lowest metric route", func(t *testing.T) {
		// 10.0.0.1 has metric 20, 10.0.0.2 has metric 10 — lower wins even though it appears second.
		data := "          0.0.0.0          0.0.0.0         10.0.0.1        10.0.0.5      20\n" +
			"          0.0.0.0          0.0.0.0         10.0.0.2        10.0.0.5      10\n"
		gw, err := parseDefaultGatewayWindows(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "10.0.0.2")
	})

	t.Run("no default route returns error", func(t *testing.T) {
		data := "Network Destination        Netmask          Gateway       Interface  Metric\n" +
			"        127.0.0.0        255.0.0.0        127.0.0.1      127.0.0.1     331\n"
		_, err := parseDefaultGatewayWindows(data)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := parseDefaultGatewayWindows("")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("destination matches but netmask does not", func(t *testing.T) {
		data := "          0.0.0.0        255.0.0.0         10.0.0.1        10.0.0.5      10\n"
		_, err := parseDefaultGatewayWindows(data)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestParseDefaultGatewayLinux(t *testing.T) {
	// /proc/net/route stores gateway bytes in little-endian hex.
	// FE01A8C0 → bytes [FE,01,A8,C0] → reversed → 192.168.1.254
	validRoute := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\tFE01A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\tFFFFFF00\t0\t0\t0\n"

	t.Run("parses default gateway", func(t *testing.T) {
		gw, err := parseDefaultGatewayLinux(validRoute)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "192.168.1.254")
	})

	t.Run("no default route returns error", func(t *testing.T) {
		data := "Iface\tDestination\tGateway\n" +
			"eth0\t0001A8C0\t00000000\n"
		_, err := parseDefaultGatewayLinux(data)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := parseDefaultGatewayLinux("")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("skips malformed hex gateway", func(t *testing.T) {
		// First default route has bad hex, second has a valid one.
		data := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\n" +
			"eth0\t00000000\tZZZZZZZZ\t0003\t0\t0\t100\n" +
			"eth0\t00000000\t0101A8C0\t0003\t0\t0\t50\n"
		gw, err := parseDefaultGatewayLinux(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "192.168.1.1")
	})

	t.Run("picks lowest metric route", func(t *testing.T) {
		// 192.168.1.254 has metric 200, 192.168.1.1 has metric 50 — lower wins.
		data := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
			"eth0\t00000000\tFE01A8C0\t0003\t0\t0\t200\t00000000\t0\t0\t0\n" +
			"wlan0\t00000000\t0101A8C0\t0003\t0\t0\t50\t00000000\t0\t0\t0\n"
		gw, err := parseDefaultGatewayLinux(data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gw, test.ShouldEqual, "192.168.1.1")
	})
}
