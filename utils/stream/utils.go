package stream

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"image"
	"io/ioutil"
	"time"
	"unsafe"

	"github.com/echolabsinc/robotcore/utils/log"
	"github.com/echolabsinc/robotcore/vision"

	"fyne.io/fyne"
)

/*
// https://stackoverflow.com/questions/9465815/rgb-to-yuv420-algorithm-efficiency
void rgba2yuv(void *destination, void *source, int width, int height, int stride) {
	const int image_size = width * height;
	unsigned char *rgba = source;
  unsigned char *dst_y = destination;
  unsigned char *dst_u = destination + image_size;
  unsigned char *dst_v = destination + image_size + image_size/4;
	// Y plane
	for( int y=0; y<height; ++y ) {
    for( int x=0; x<width; ++x ) {
      const int i = y*(width+stride) + x;
			*dst_y++ = ( ( 66*rgba[4*i] + 129*rgba[4*i+1] + 25*rgba[4*i+2] ) >> 8 ) + 16;
		}
  }
  // U plane
  for( int y=0; y<height; y+=2 ) {
    for( int x=0; x<width; x+=2 ) {
      const int i = y*(width+stride) + x;
			*dst_u++ = ( ( -38*rgba[4*i] + -74*rgba[4*i+1] + 112*rgba[4*i+2] ) >> 8 ) + 128;
		}
  }
  // V plane
  for( int y=0; y<height; y+=2 ) {
    for( int x=0; x<width; x+=2 ) {
      const int i = y*(width+stride) + x;
			*dst_v++ = ( ( 112*rgba[4*i] + -94*rgba[4*i+1] + -18*rgba[4*i+2] ) >> 8 ) + 128;
		}
  }
}
*/
import "C"

func streamFunc(once func(), f func() image.Image, remoteView RemoteView, captureInternal time.Duration) {
	if once != nil {
		once()
	}
	<-remoteView.Ready()
	for {
		time.Sleep(captureInternal)
		remoteView.InputFrames() <- f()
	}
}

func StreamFunc(f func() image.Image, remoteView RemoteView, captureInternal time.Duration) {
	streamFunc(nil, f, remoteView, captureInternal)
}

func StreamWindow(window fyne.Window, remoteView RemoteView, captureInternal time.Duration) {
	streamFunc(
		func() { time.Sleep(2 * time.Second) },
		func() image.Image { return window.Canvas().Capture() },
		remoteView,
		captureInternal,
	)
}

func StreamMatSource(src vision.MatSource, remoteView RemoteView, captureInternal time.Duration, logger log.Logger) {
	streamFunc(
		nil,
		func() image.Image {
			now := time.Now()
			mat, _, err := src.NextColorDepthPair()
			if err != nil {
				panic(err) // TODO(err): don't panic... bones, sinking like stones
			}
			defer mat.Close()
			if remoteView.Debug() {
				logger.Debugw("NextColorDepthPair", "elapsed", time.Since(now))
			}
			img, err := mat.ToImage()
			if err != nil {
				panic(err) // TODO(err): don't panic
			}
			return img
		},
		remoteView,
		captureInternal,
	)
}

// RgbaToYuv convert to yuv from rgba
// TODO: rewrite code maybe
func RgbaToYuv(rgba *image.RGBA) []byte {
	w := rgba.Rect.Max.X
	h := rgba.Rect.Max.Y
	size := int(float32(w*h) * 1.5)
	stride := rgba.Stride - w*4
	yuv := make([]byte, size)
	// now := time.Now()
	C.rgba2yuv(unsafe.Pointer(&yuv[0]), unsafe.Pointer(&rgba.Pix[0]), C.int(w), C.int(h), C.int(stride))
	// fmt.Println("conversion took", time.Since(now))
	return yuv
}

// Allows compressing offer/answer to bypass terminal input limits.
const compress = false

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	if compress {
		b = zip(b)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if compress {
		b = unzip(b)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

func zip(in []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(in)
	if err != nil {
		panic(err)
	}
	err = gz.Flush()
	if err != nil {
		panic(err)
	}
	err = gz.Close()
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

func unzip(in []byte) []byte {
	var b bytes.Buffer
	_, err := b.Write(in)
	if err != nil {
		panic(err)
	}
	r, err := gzip.NewReader(&b)
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return res
}
