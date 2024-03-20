// Package unit contains data types that represent encoded video data units
// https://github.com/bluenviron/mediamtx/blob/main/internal/unit/h264.go & related package
package unit

import (
	"time"

	"github.com/pion/rtp"
)

// Unit is the elementary data unit routed across the server.
type Unit interface {
	// returns RTP packets contained into the unit.
	GetRTPPackets() []*rtp.Packet

	// returns the NTP timestamp of the unit.
	GetNTP() time.Time

	// returns the PTS of the unit.
	GetPTS() time.Duration
}

// H264 is a H264 data unit.
type H264 struct {
	Base
	AU [][]byte
}

// Base contains fields shared across all units.
type Base struct {
	RTPPackets []*rtp.Packet
	NTP        time.Time
	PTS        time.Duration
}

// GetRTPPackets implements Unit.
func (u *Base) GetRTPPackets() []*rtp.Packet {
	return u.RTPPackets
}

// GetNTP implements Unit.
func (u *Base) GetNTP() time.Time {
	return u.NTP
}

// GetPTS implements Unit.
func (u *Base) GetPTS() time.Duration {
	return u.PTS
}
