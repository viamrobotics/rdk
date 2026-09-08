package videosource

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"go.viam.com/test"
)

func encodeTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for x := 0; x < 32; x++ {
		for y := 0; y < 32; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	test.That(t, jpeg.Encode(&buf, img, nil), test.ShouldBeNil)
	return buf.Bytes()
}

func TestMJPEGFrameComplete(t *testing.T) {
	full := encodeTestJPEG(t)
	test.That(t, mjpegFrameComplete(full), test.ShouldBeTrue)

	t.Run("truncated mid scan", func(t *testing.T) {
		cut := full[:len(full)*2/3]
		test.That(t, mjpegFrameComplete(cut), test.ShouldBeFalse)
		// Confirm this is the failure mode the check exists to prevent.
		_, err := jpeg.Decode(bytes.NewReader(cut))
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("missing final EOI byte", func(t *testing.T) {
		test.That(t, mjpegFrameComplete(full[:len(full)-1]), test.ShouldBeFalse)
	})

	t.Run("zero padding after EOI", func(t *testing.T) {
		padded := append(append([]byte{}, full...), make([]byte, 64)...)
		test.That(t, mjpegFrameComplete(padded), test.ShouldBeTrue)
	})

	t.Run("non zero trailing garbage", func(t *testing.T) {
		garbage := append(append([]byte{}, full...), 0x12, 0x34)
		test.That(t, mjpegFrameComplete(garbage), test.ShouldBeFalse)
	})

	t.Run("missing SOI", func(t *testing.T) {
		test.That(t, mjpegFrameComplete(full[2:]), test.ShouldBeFalse)
	})

	t.Run("too short", func(t *testing.T) {
		test.That(t, mjpegFrameComplete(nil), test.ShouldBeFalse)
		test.That(t, mjpegFrameComplete([]byte{}), test.ShouldBeFalse)
		test.That(t, mjpegFrameComplete([]byte{0xFF, 0xD8, 0xFF}), test.ShouldBeFalse)
		test.That(t, mjpegFrameComplete([]byte{0, 0, 0, 0}), test.ShouldBeFalse)
	})

	t.Run("minimal SOI EOI", func(t *testing.T) {
		test.That(t, mjpegFrameComplete([]byte{0xFF, 0xD8, 0xFF, 0xD9}), test.ShouldBeTrue)
	})
}
