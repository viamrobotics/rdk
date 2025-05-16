// Package tunnel contains helpers for a traffic tunneling implementation
package tunnel

import (
	"fmt"
	"sync"
	"time"
)

// Statistics tracks metrics about a tunnel connection.
type Statistics struct {
	mu sync.Mutex

	// Packet counts
	packetsSent     int64
	packetsReceived int64

	// Byte counts
	bytesSent     int64
	bytesReceived int64

	// RTT tracking
	rttSamples    []time.Duration
	rttTotal      time.Duration
	packetTimings map[int64]time.Time // Maps packet IDs to send times

	// Time tracking
	startTime time.Time
	endTime   time.Time

	// Packet ID counter for RTT measurement
	currentPacketID int64
}

// NewStatistics creates a new Statistics instance for tracking tunnel metrics.
func NewStatistics() *Statistics {
	return &Statistics{
		packetTimings: make(map[int64]time.Time),
		startTime:     time.Now(),
	}
}

// TrackPacketSent records metrics for a sent packet.
func (s *Statistics) TrackPacketSent(bytes int) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.packetsSent++
	s.bytesSent += int64(bytes)

	// Track send time for RTT calculation
	packetID := s.currentPacketID
	s.packetTimings[packetID] = time.Now()
	s.currentPacketID++

	return packetID
}

// TrackPacketReceived records metrics for a received packet.
func (s *Statistics) TrackPacketReceived(bytes int, packetID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.packetsReceived++
	s.bytesReceived += int64(bytes)

	// Calculate RTT if we have timing data for this packet
	if sentTime, ok := s.packetTimings[packetID]; ok {
		rtt := time.Since(sentTime)
		s.rttSamples = append(s.rttSamples, rtt)
		s.rttTotal += rtt
		delete(s.packetTimings, packetID) // Clean up timing data
	}
}

// Close finalizes the statistics and prepares for reporting.
func (s *Statistics) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.endTime = time.Now()
}

// Report returns a formatted summary of the tunnel statistics.
func (s *Statistics) Report() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := s.endTime.Sub(s.startTime)
	if s.endTime.IsZero() {
		duration = time.Since(s.startTime)
	}

	var avgRTT time.Duration
	if len(s.rttSamples) > 0 {
		avgRTT = s.rttTotal / time.Duration(len(s.rttSamples))
	}

	var avgPacketSizeSent, avgPacketSizeRecv int64
	if s.packetsSent > 0 {
		avgPacketSizeSent = s.bytesSent / s.packetsSent
	}
	if s.packetsReceived > 0 {
		avgPacketSizeRecv = s.bytesReceived / s.packetsReceived
	}

	return fmt.Sprintf(`Tunnel Statistics:
  Duration:           %v
  Packets Sent:       %d
  Packets Received:   %d
  Total Bytes Sent:   %d
  Total Bytes Received: %d
  Avg Packet Size Sent: %d bytes
  Avg Packet Size Received: %d bytes
  Average RTT:        %v
  Measured RTT Samples: %d`,
		duration,
		s.packetsSent,
		s.packetsReceived,
		s.bytesSent,
		s.bytesReceived,
		avgPacketSizeSent,
		avgPacketSizeRecv,
		avgRTT,
		len(s.rttSamples))
}

// GetStatistics returns the raw statistics values.
func (s *Statistics) GetStatistics() (packetsSent, packetsReceived, bytesSent, bytesReceived int64, avgRTT time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var avgRTTValue time.Duration
	if len(s.rttSamples) > 0 {
		avgRTTValue = s.rttTotal / time.Duration(len(s.rttSamples))
	}

	return s.packetsSent, s.packetsReceived, s.bytesSent, s.bytesReceived, avgRTTValue
}
