package server

import (
	"fmt"
	"strconv"
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
func logSTUNResults(logger logging.Logger, stunResponses []*STUNResponse, network string) {
	// Use lastBindResponseAddr to track whether the received address from STUN servers is
	// "unstable." Any changes in port, in particular, between different STUN server's
	// bind responses indicates that we may be behind an endpoint-dependent-mapping NAT
	// device ("hard" NAT.)
	var expectedBindResponseAddr string
	var unstableBindResponseAddr bool

	var successfulStunResponses int
	var stunLogKeysAndValues []any
	for i, sr := range stunResponses {
		if sr.BindResponseAddr == "unknown" {
			logger.Warnw("STUN test fail", "stun_url", sr.STUNServerURL, "stun_address", sr.STUNServerAddr)
			continue
		}

		successfulStunResponses++
		si := strconv.Itoa(i)
		newKeysAndValues := []any{
			"stun_url" + si, sr.STUNServerURL, "received_address" + si, sr.BindResponseAddr,
			"rtt_ms" + si, sr.TimeToBindResponse.Milliseconds(), "resolved_stun_address" + si, sr.STUNServerAddr,
		}
		stunLogKeysAndValues = append(stunLogKeysAndValues, newKeysAndValues...)

		if expectedBindResponseAddr == "" {
			// Take first bind response address as "expected," all others should match when
			// behind an endpoint-independent-mapping NAT device.
			expectedBindResponseAddr = sr.BindResponseAddr
		} else if expectedBindResponseAddr != sr.BindResponseAddr {
			unstableBindResponseAddr = true
		}
	}
	msg := fmt.Sprintf("%d/%d %v STUN tests succeeded", successfulStunResponses, len(stunResponses), network)
	logger.Infow(msg, stunLogKeysAndValues...)

	if unstableBindResponseAddr {
		logger.Warn("STUN tests indicate this machine is behind a 'hard' NAT device; STUN may not work as expected")
	}
}
