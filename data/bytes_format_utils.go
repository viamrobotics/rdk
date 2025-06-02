package data

import "fmt"

const (
	_ = 1 << (10 * iota)
	kib
	mib
	gib
	tib
)

// FormatBytesI64 formats an int64 representing bytes
// as an easily human parsable string.
func FormatBytesI64(b int64) string {
	switch {
	case b > tib:
		return fmt.Sprintf("%.2f TB", float64(b)/tib)
	case b > gib:
		return fmt.Sprintf("%.2f GB", float64(b)/gib)
	case b > mib:
		return fmt.Sprintf("%.2f MB", float64(b)/mib)
	case b > kib:
		return fmt.Sprintf("%.2f KB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

// FormatBytesU64 formats an uint64 representing bytes
// as an easily human parsable string.
func FormatBytesU64(b uint64) string {
	switch {
	case b > tib:
		return fmt.Sprintf("%.2f TB", float64(b)/tib)
	case b > gib:
		return fmt.Sprintf("%.2f GB", float64(b)/gib)
	case b > mib:
		return fmt.Sprintf("%.2f MB", float64(b)/mib)
	case b > kib:
		return fmt.Sprintf("%.2f KB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}
