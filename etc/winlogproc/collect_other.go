//go:build !windows

package winlogproc

import (
	"errors"
	"time"
)

// CollectOpts is a no-op type on non-Windows so callers compile but
// can't actually invoke Collect. See collect.go for the real definition.
type CollectOpts struct {
	ETLDir         string
	SessionName    string
	EventlogSource string
	OutDir         string
	After, Before  time.Time
}

// Collect is Windows-only because it shells to tracerpt, logman, and
// Get-EventLog. On other platforms it returns an error so callers can
// still build, and the cross-platform Eventlog/Trace processors stay
// usable for dumps transferred from a Windows host.
func Collect(opts CollectOpts) (string, error) {
	return "", errors.New("winlogproc.Collect is only supported on Windows")
}

// DefaultETLDir returns an empty string on non-Windows. See
// collect.go for the real implementation.
func DefaultETLDir() string { return "" }
