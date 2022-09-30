// Package rlog defines common logging utilities.
package rlog

import "github.com/edaniels/golog"

// Logger is the global logger that should be used when a context specific
// one is unavailable.
var Logger = golog.Global()
