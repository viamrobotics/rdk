package utils

import (
	"github.com/pkg/errors"
	rutils "go.viam.com/utils"
)

const (
	// MimeTypeRawRGBA is for go's internal image.NRGBA. This uses the custom header as
	// explained in the comments for rimage.DecodeImage and rimage.EncodeImage.
	MimeTypeRawRGBA = "image/vnd.viam.rgba"

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
)

// SupportedMimeTypes is a set of the currently supported MIME types.
var SupportedMimeTypes rutils.StringSet = rutils.NewStringSet(
	MimeTypeRawRGBA,
	MimeTypeJPEG,
	MimeTypePCD,
	MimeTypePNG,
	MimeTypeQOI,
	MimeTypeTabular,
)

// CheckSupportedMimeType checks whether a requested MIME type is
// supported.
func CheckSupportedMimeType(mimeType string) error {
	if _, ok := SupportedMimeTypes[mimeType]; !ok {
		return NewUnrecognizedMimeTypeError(mimeType)
	}
	return nil
}

// NewUnrecognizedMimeTypeError returns an error signifying that a MIME type
// is unrecognized or unsupported.
func NewUnrecognizedMimeTypeError(mimeType string) error {
	return errors.Errorf("Unrecognized mime type: %s", mimeType)
}
