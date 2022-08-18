package utils

const (
	// MimeTypeRawRGBA is for go's internal image.RGBA. The data has no headers, so you must
	// assume that the bytes can be read directly into the image.RGBA struct.
	MimeTypeRawRGBA = "image/vnd.viam.rgba"

	// MimeTypeJPEG is regular jpgs.
	MimeTypeJPEG = "image/jpeg"

	// MimeTypePNG is regular pngs.
	MimeTypePNG = "image/png"

	// MimeTypePCD is for .pcd pountcloud files.
	MimeTypePCD = "pointcloud/pcd"

	// MimeTypeQOI is for .qoi "Quite OK Image" for lossless, fast encoding/decoding.
	MimeTypeQOI = "image/qoi"
)
