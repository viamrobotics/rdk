package utils

import (
	"fmt"
	"strings"
)

const (
	// MimeTypeRawRGBA is for go's internal image.NRGBA64. The data has no headers, so you must
	// assume that the bytes can be read directly into the image.NRGBA64 struct.
	MimeTypeRawRGBA = "image/vnd.viam.rgba"

	// MimeTypeRawRGBALazy is a lazy MimeTypeRawRGBA.
	MimeTypeRawRGBALazy = MimeTypeRawRGBA + "+" + MimeTypeSuffixLazy

	// MimeTypeJPEG is regular jpgs.
	MimeTypeJPEG = "image/jpeg"

	// MimeTypePNG is regular pngs.
	MimeTypePNG = "image/png"

	// MimeTypePCD is for .pcd pountcloud files.
	MimeTypePCD = "pointcloud/pcd"

	// MimeTypeQOI is for .qoi "Quite OK Image" for lossless, fast encoding/decoding.
	MimeTypeQOI = "image/qoi"

	// MimeTypeSuffixLazy is used to indicate a lazy loading of data.
	MimeTypeSuffixLazy = "lazy"
)

// WithLazyMIMEType attaches the lazy suffix to a MIME.
func WithLazyMIMEType(mimeType string) string {
	return fmt.Sprintf("%s+%s", mimeType, MimeTypeSuffixLazy)
}

const lazyMIMESuffixCheck = "+" + MimeTypeSuffixLazy

// CheckLazyMIMEType checks the lazy suffix of a MIME.
func CheckLazyMIMEType(mimeType string) (string, bool) {
	if strings.Count(mimeType, lazyMIMESuffixCheck) == 1 && strings.HasSuffix(mimeType, lazyMIMESuffixCheck) {
		return strings.TrimSuffix(mimeType, lazyMIMESuffixCheck), true
	}
	return mimeType, false
}
