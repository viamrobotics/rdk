package server

import (
	"fmt"

	"go.viam.com/rdk/logging"
)

// STUNResponse represents a response from a STUN server.
type STUNResponse struct {
	// STUNServerURL is the URL of the STUN server.
	STUNServerURL string

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
	sourceAddress,
	network string,
) {
	// Use lastBindResponseAddr to track whether the received address from STUN servers is
	// "unstable." Any changes in port, in particular, between different STUN server's bind
	// responses indicates that we may be behind an endpoint-dependent-mapping NAT device
	// ("hard" NAT).
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
	if successfulStunResponses < len(stunResponses) {
		logger.Warnw(
			msg,
			fmt.Sprintf("%v_source_address", network), sourceAddress,
			fmt.Sprintf("%v_tests", network), stringifySTUNResponses(stunResponses),
		)
	} else {
		logger.Infow(
			msg,
			fmt.Sprintf("%v_source_address", network), sourceAddress,
			fmt.Sprintf("%v_tests", network), stringifySTUNResponses(stunResponses),
		)
	}

	if unstableBindResponseAddr {
		logger.Warnf(
			"%v STUN tests indicate this machine is behind a 'hard' NAT device; STUN may not work as expected",
			network,
		)
	}
}
