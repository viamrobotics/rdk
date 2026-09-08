package videosource

import "bytes"

var (
	jpegSOI = []byte{0xFF, 0xD8}
	jpegEOI = []byte{0xFF, 0xD9}
)

// mjpegFrameComplete reports whether b looks like a whole JPEG image: it starts with the SOI
// marker and ends with the EOI marker.
//
// V4L2 MJPEG buffers from UVC cameras can arrive truncated when USB payloads are lost. The uvcvideo
// driver only length-checks uncompressed formats, so such a frame reaches userspace with a short
// bytesused and no error flag. Decoding it wastes a full JPEG decode and then fails with errors like
// "invalid JPEG format: short Huffman data", so callers use this check to skip the frame instead.
//
// Trailing zero bytes are ignored because some cameras pad frames after EOI.
func mjpegFrameComplete(b []byte) bool {
	b = bytes.TrimRight(b, "\x00")
	if len(b) < len(jpegSOI)+len(jpegEOI) {
		return false
	}
	return bytes.HasPrefix(b, jpegSOI) && bytes.HasSuffix(b, jpegEOI)
}
