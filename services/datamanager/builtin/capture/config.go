// Package capture implements datacapture for the builtin datamanger
package capture

// Config is the capture config.
type Config struct {
	CaptureDisabled             bool
	CaptureDir                  string
	Tags                        []string
	MaximumCaptureFileSizeBytes int64
}
