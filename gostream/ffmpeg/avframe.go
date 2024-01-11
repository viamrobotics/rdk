//go:build cgo && !android

package ffmpeg

//#include <libavutil/frame.h>
//#include <libavutil/imgutils.h>
import "C"

import (
	"image"
	"unsafe"

	"github.com/pkg/errors"
)

// Frame an AVFrame
type Frame struct {
	p  *C.struct_AVFrame
	pp **C.struct_AVFrame
}

// FrameAlloc Allocate an AVFrame and set its fields to default values. The resulting
// struct must be freed using av_frame_free().
//
// @return An AVFrame filled with default values or NULL on failure.
//
// @note this only allocates the AVFrame itself, not the data buffers. Those
// must be allocated through other means, e.g. with av_frame_get_buffer() or
// manually.
func FrameAlloc() (Frame, error) {
	p := C.av_frame_alloc()
	if p == nil {
		return Frame{}, errors.New("cannot allocate frame")
	}

	return Frame{
		p:  p,
		pp: &p,
	}, nil
}

// SetFrame sets the given frame from the given width (w), height (h), and pixel format (pixFmt)
func SetFrame(f Frame, w, h, pixFmt int) error {
	f.p.width = C.int(w)
	f.p.height = C.int(h)
	f.p.format = C.int(pixFmt)
	if ret := C.av_frame_get_buffer(f.p, 0 /*alignment*/); ret < 0 {
		return errors.Errorf("error allocating avframe buffer: return value %d", int(ret))
	}
	return nil
}

// FrameMakeWritable Ensure that the frame data is writable, avoiding data copy if possible.
//
// Do nothing if the frame is writable, allocate new buffers and copy the data
// if it is not. Non-refcounted frames behave as non-writable, i.e. a copy
// is always made.
//
// @return 0 on success, a negative AVERROR on error.
//
// @see av_frame_is_writable(), av_buffer_is_writable(),
// av_buffer_make_writable()
func FrameMakeWritable(f Frame) int {
	return int(C.av_frame_make_writable(f.p))
}

// Unref Unreference all the buffers referenced by frame and reset the frame fields.
func (f *Frame) Unref() {
	C.av_frame_unref(f.p)
}

// Free the frame
func (f *Frame) Free() {
	C.av_frame_free(f.pp)
}

// Stride returns the stride
func (f *Frame) Stride() int {
	return int(f.p.linesize[0])
}

// Width returns the width
func (f *Frame) Width() int {
	return int(f.p.width)
}

// Height returns the height
func (f *Frame) Height() int {
	return int(f.p.height)
}

// PixFmt returns the pixel format
func (f *Frame) PixFmt() int {
	return int(f.p.format)
}

// SetFrameFromImgMacroAlign sets the frame from the given image.YCbCr adding
// line padding to the image to ensure that the data is aligned to the given boundary.
// For example see alignment requirements for the Raspberry Pi GPU codec:
// https://github.com/raspberrypi/linux/blob/rpi-6.1.y/drivers/staging/vc04_services/bcm2835-codec/bcm2835-v4l2-codec.c#L174
func (f *Frame) SetFrameFromImgMacroAlign(img *image.YCbCr, boundary int) {
	// Calculating padded strides
	// Rounding up to next multiple of boundary value
	paddedYStride := ((img.YStride + boundary - 1) / boundary) * boundary
	// UV half the Y stride for 4:2:0
	paddedCbCrStride := paddedYStride / 2

	// Allocate new buffers with padding
	// These will be freed by the GC
	paddedY := make([]byte, paddedYStride*img.Rect.Dy())
	paddedCb := make([]byte, paddedCbCrStride*img.Rect.Dy()/2)
	paddedCr := make([]byte, paddedCbCrStride*img.Rect.Dy()/2)

	// Copy data from img to padded buffers line by line
	for i := 0; i < img.Rect.Dy(); i++ {
		copy(paddedY[i*paddedYStride:(i+1)*paddedYStride], img.Y[i*img.YStride:])
	}
	for i := 0; i < img.Rect.Dy()/2; i++ {
		copy(paddedCb[i*paddedCbCrStride:(i+1)*paddedCbCrStride], img.Cb[i*img.CStride:])
		copy(paddedCr[i*paddedCbCrStride:(i+1)*paddedCbCrStride], img.Cr[i*img.CStride:])
	}

	// Update AVFrame data pointers and linesize
	// Casting from go slice to C array without changing memory
	f.p.data[0] = (*C.uchar)(unsafe.Pointer(&paddedY[0]))
	f.p.data[1] = (*C.uchar)(unsafe.Pointer(&paddedCb[0]))
	f.p.data[2] = (*C.uchar)(unsafe.Pointer(&paddedCr[0]))
	f.p.linesize[0] = C.int(paddedYStride)
	f.p.linesize[1] = C.int(paddedCbCrStride)
	f.p.linesize[2] = C.int(paddedCbCrStride)
}

// SetFrameFromImg sets the frame from the given image.YCbCr
func (f *Frame) SetFrameFromImg(img *image.YCbCr) {
	f.p.data[0] = ptr(img.Y)
	f.p.data[1] = ptr(img.Cb)
	f.p.data[2] = ptr(img.Cr)

	w := C.int(img.Bounds().Dx())
	f.p.linesize[0] = w
	f.p.linesize[1] = w / 2
	f.p.linesize[2] = w / 2
}

// SetFramePTS sets the presentation time stamp (PTS)
func (f *Frame) SetFramePTS(pts int64) {
	f.p.pts = C.int64_t(pts)
}
