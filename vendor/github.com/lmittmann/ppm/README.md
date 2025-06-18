# Package ppm [![PkgGoDev](https://pkg.go.dev/badge/github.com/lmittmann/ppm)](https://pkg.go.dev/github.com/lmittmann/ppm) [![Go Report Card](https://goreportcard.com/badge/github.com/lmittmann/ppm)](https://goreportcard.com/report/github.com/lmittmann/ppm)


```
import "github.com/lmittmann/ppm"
```
Package ppm implements a Portable Pixel Map (PPM) image decoder and encoder. The supported image color model is [color.RGBAModel](https://pkg.go.dev/image/color#RGBAModel).

The PPM specification is at http://netpbm.sourceforge.net/doc/ppm.html.


## func [Decode](reader.go#L28)
<pre>
func Decode(r <a href="https://pkg.go.dev/io">io</a>.<a href="https://pkg.go.dev/io#Reader">Reader</a>) (<a href="https://pkg.go.dev/image">image</a>.<a href="https://pkg.go.dev/image#Image">Image</a>, <a href="https://pkg.go.dev/builtin#error">error</a>)
</pre>
Decode reads a PPM image from Reader r and returns it as an image.Image.


## func [DecodeConfig](reader.go#L39)
<pre>
func DecodeConfig(r <a href="https://pkg.go.dev/io">io</a>.<a href="https://pkg.go.dev/io#Reader">Reader</a>) (<a href="https://pkg.go.dev/image">image</a>.<a href="https://pkg.go.dev/image#Config">Config</a>, <a href="https://pkg.go.dev/builtin#error">error</a>)
</pre>
DecodeConfig returns the color model and dimensions of a PPM image without decoding the entire image.


## func [Encode](writer.go#L15)
<pre>
func Encode(w <a href="https://pkg.go.dev/io">io</a>.<a href="https://pkg.go.dev/io#Writer">Writer</a>, img <a href="https://pkg.go.dev/image">image</a>.<a href="https://pkg.go.dev/image#Image">Image</a>) <a href="https://pkg.go.dev/builtin#error">error</a>
</pre>
Encode writes the Image img to Writer w in PPM format.
