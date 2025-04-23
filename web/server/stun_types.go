package server

import (
	"time"

	"go.viam.com/rdk/logging"
)

// STUNResponse represents a response from a STUN server.
type STUNResponse struct {
	// STUNServerURL is the URL of the STUN server.
	STUNServerURL string

	// STUNServerAddr is the resolved address of the STUN server.
	STUNServerAddr string

	// BindResponseAddr is our address as reported by the STUN server.
	BindResponseAddr string

	// Time taken to send bind request, receive bind response, and extract address. A vague
	// measurement of RTT to the STUN server.
	TimeToBindResponse time.Duration
}

// NewSTUNResponse Returns a new STUNResponse object with the passed-in URL. Address
// fields will be "unknown" until manually set otherwise.
func NewSTUNResponse(stunServerURL string) *STUNResponse {
	return &STUNResponse{
		STUNServerURL:    stunServerURL,
		STUNServerAddr:   "unknown",
		BindResponseAddr: "unknown",
	}
}

// Logs success, failures, and observations from a set of STUN responses.
func logSTUNResults(logger logging.Logger, stunResponses []*STUNResponse) {
	// Use lastBindResponseAddr to track whether the received address from STUN servers is
	// "unstable." Any changes in port, in particular, between different STUN server's
	// bind responses indicates that we may be behind an endpoint-dependent-mapping NAT
	// device ("hard" NAT.)
	var expectedBindResponseAddr string
	var unstableBindResponseAddr bool

	var successfulStunResponses int
	for _, sr := range stunResponses {
		if sr.BindResponseAddr == "unknown" {
			logger.Warnw("STUN test fail", "stun_url", sr.STUNServerURL, "stun_address", sr.STUNServerAddr)
			continue
		}

		successfulStunResponses++
		logger.Infow("STUN test success", "stun_url", sr.STUNServerURL, "received_address", sr.BindResponseAddr,
			"rtt_ms", sr.TimeToBindResponse.Milliseconds(), "resolved_stun_address", sr.STUNServerAddr)
		if expectedBindResponseAddr == "" {
			// Take first bind response address as "expected," all others should match when
			// behind an endpoint-independent-mapping NAT device.
			expectedBindResponseAddr = sr.BindResponseAddr
		} else if expectedBindResponseAddr != sr.BindResponseAddr {
			unstableBindResponseAddr = true
		}
	}
	logger.Infof("%d/%d STUN tests succeeded", successfulStunResponses, len(stunResponses))

	if unstableBindResponseAddr {
		logger.Warn("STUN tests indicate this machine is behind a 'hard' NAT device; STUN may not work as expected")
	}
}
