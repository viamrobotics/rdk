package server

import (
	"fmt"

	"go.viam.com/rdk/logging"
)

// STUNResponse represents a response from a STUN server.
type STUNResponse struct {
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
