package data

import "go.viam.com/rdk/utils"

// FormatBytesI64 formats an int64 representing bytes
// as an easily human parsable string.
//
// Deprecated: use utils.FormatBytesI64. This wrapper is retained to avoid
// breaking external callers of the data package.
func FormatBytesI64(b int64) string {
	return utils.FormatBytesI64(b)
}

// FormatBytesU64 formats an uint64 representing bytes
// as an easily human parsable string.
//
// Deprecated: use utils.FormatBytes. This wrapper is retained to avoid
// breaking external callers of the data package.
func FormatBytesU64(b uint64) string {
	return utils.FormatBytes(b)
}
