// Package capture implements datacapture for the builtin datamanger
package capture

// Config is the capture config.
type Config struct {
	// CaptureDisabled if set to true disables all data capture collectors
	CaptureDisabled bool
	// CaptureDir defines where data capture should write capture files
	CaptureDir string
	// Tags defines the tags that should be added to capture file metadata
	Tags []string
	// MaximumCaptureFileSizeBytes defines the maximum size that in progress data capture
	// (.prog) files should be allowed to grow to before they are convered into .capture
	// files
	MaximumCaptureFileSizeBytes int64
}
