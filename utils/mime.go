package utils

import (
	"fmt"
	"strings"
)

const (
	// MimeTypeSuffixLazy is used to indicate a lazy loading of data.
	MimeTypeSuffixLazy = "lazy"

	// MimeTypeRawRGBA is for go's internal image.NRGBA. This uses the custom header as
	// explained in the comments for rimage.DecodeImage and rimage.EncodeImage.
	MimeTypeRawRGBA = "image/vnd.viam.rgba"

	// MimeTypeRawRGBALazy is a lazy MimeTypeRawRGBA.
	MimeTypeRawRGBALazy = MimeTypeRawRGBA + "+" + MimeTypeSuffixLazy

	// MimeTypeRawDepth is for depth images.
	MimeTypeRawDepth = "image/vnd.viam.dep"

	// MimeTypeJPEG is regular jpgs.
	MimeTypeJPEG = "image/jpeg"

	// MimeTypePNG is regular pngs.
	MimeTypePNG = "image/png"

	// MimeTypePCD is for .pcd pountcloud files.
	MimeTypePCD = "pointcloud/pcd"

	// MimeTypeQOI is for .qoi "Quite OK Image" for lossless, fast encoding/decoding.
	MimeTypeQOI = "image/qoi"

	// MimeTypeTabular used to indicate tabular data, this is used mainly for filtering data.
	MimeTypeTabular = "x-application/tabular"

	// MimeTypeDefault used if mimetype cannot be inferred.
	MimeTypeDefault = "application/octet-stream"

	// MimeTypeH264 used to indicate H264 frames.
	MimeTypeH264 = "video/h264"
)

// WithLazyMIMEType attaches the lazy suffix to a MIME.
func WithLazyMIMEType(mimeType string) string {
	if _, has := CheckLazyMIMEType(mimeType); has {
		return mimeType
	}
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
