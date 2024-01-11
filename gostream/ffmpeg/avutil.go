//go:build cgo && !android

package ffmpeg

//#include <libswresample/swresample.h>
//#include <libavutil/error.h>
//#include <libavutil/dict.h>
//#include <stdlib.h>
// static const char *error2string(int code) { return av_err2str(code); }
import "C"

import (
	"reflect"
	"unsafe"

	"github.com/pkg/errors"
)

func ptr(buf []byte) *C.uint8_t {
	h := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	return (*C.uint8_t)(unsafe.Pointer(h.Data))
}

const (
	// EAGAIN Resource temporarily unavailable
	EAGAIN = -11
	// EOF End of file
	EOF = int(C.AVERROR_EOF)
	// Success no errors
	Success = 0
)

// ErrorFromCode returns an error from the given code
func ErrorFromCode(code int) error {
	if code >= 0 {
		return nil
	}

	return errors.New(C.GoString(C.error2string(C.int(code))))
}

const (
	// AvmediaTypeUnknown AVMEDIA_TYPE_UNKNOWN
	AvmediaTypeUnknown = iota - 1 // Usually treated as AVMEDIA_TYPE_DATA
	// AvmediaTypeVideo AVMEDIA_TYPE_VIDEO
	AvmediaTypeVideo
	// AvmediaTypeAudio AVMEDIA_TYPE_AUDIO
	AvmediaTypeAudio
	// AvmediaTypeData AVMEDIA_TYPE_DATA
	AvmediaTypeData // Opaque data information usually continuous
	// AvmediaTypeSubtitle AVMEDIA_TYPE_SUBTITLE
	AvmediaTypeSubtitle
	// AvmediaTypeAttachment AVMEDIA_TYPE_ATTACHMENT
	AvmediaTypeAttachment // Opaque data information usually sparse
	// AvmediaTypeNb AVMEDIA_TYPE_NB
	AvmediaTypeNb
)
