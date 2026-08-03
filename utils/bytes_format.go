package utils

import "fmt"

// Decimal (1000-based) units, so the KB/MB/GB/TB labels below are accurate
// (1 KB == 1000 bytes, not 1024).
const (
	kb = 1000
	mb = 1000 * kb
	gb = 1000 * mb
	tb = 1000 * gb
)

// FormatBytes renders a byte count as a human-friendly string in decimal units (KB/MB/GB/TB).
func FormatBytes(b uint64) string {
	// The comparisons use >= so an exact power (e.g. 1 MB) renders in its own unit
	// instead of falling through to "1000.00 KB".
	switch {
	case b >= tb:
		return fmt.Sprintf("%.2f TB", float64(b)/tb)
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.2f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

// FormatBytesI64 renders a signed byte count as a human-friendly string in decimal units
// (KB/MB/GB/TB), preserving the sign for negative values.
func FormatBytesI64(b int64) string {
	if b < 0 {
		return "-" + FormatBytes(uint64(-b))
	}
	return FormatBytes(uint64(b))
}
