// Package avlog is libav's logging facilities
// See https://ffmpeg.org/doxygen/3.0/log_8h.html
package avlog

//#cgo CFLAGS: -I${SRCDIR}/../include
//#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../Linux-aarch64/lib -lavformat -lavcodec -lavutil
//#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../Linux-x86_64/lib -lavformat -lavcodec -lavutil
//#cgo linux,arm LDFLAGS: -L${SRCDIR}/../Linux-armv7l/lib -lavformat -lavcodec -lavutil
//#include <libavutil/log.h>
import "C"

const (
	// LogQuiet Print no output.
	LogQuiet = (iota * 8) - 8

	// LogPanic Something went really wrong and we will crash now.
	LogPanic

	// LogFatal Something went wrong and recovery is not possible.
	//  For example, no header was found for a format which depends
	//  on headers or an illegal combination of parameters is used.
	LogFatal

	// LogError Something went wrong and cannot losslessly be recovered.
	// However, not all future data is affected.
	LogError

	// LogWarning Something somehow does not look correct. This may or may not
	// lead to problems. An example would be the use of '-vstrict -2'.
	LogWarning

	// LogInfo Standard information.
	LogInfo

	// LogVerbose Detailed information.
	LogVerbose

	// LogDebug Stuff which is only useful for libav* developers.
	LogDebug

	// LogTrace Extremely verbose debugging, useful for libav* development.
	LogTrace
)

// SetLevel sets the log level
func SetLevel(level int) {
	C.av_log_set_level(C.int(level))
}

// GetLevel returns the current log level
func GetLevel() int {
	return int(C.av_log_get_level())
}
