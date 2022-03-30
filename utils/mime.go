package utils

const (
	// MimeTypeViamBest is a hint that we should send whatever format is best for the type of image.
	MimeTypeViamBest = "image/viambest"

	// MimeTypeRawIWD is a row rimage.ImageWithDepth.
	MimeTypeRawIWD = "image/raw-iwd"

	// MimeTypeRawRGBA is for go's internal image.RGBA.
	MimeTypeRawRGBA = "image/raw-rgba"

	// MimeTypeRawDepth is a raw rimage.DepthMap.
	MimeTypeRawDepth = "image/raw-depth"

	// MimeTypeBoth is for the the .both file format we use, see rimage/both.go.
	MimeTypeBoth = "image/both"

	// MimeTypeJPEG is regular jpgs.
	MimeTypeJPEG = "image/jpeg"

	// MimeTypePNG is regular pngs.
	MimeTypePNG = "image/png"

	// MimeTypePCD is for .pcd pountcloud files.
	MimeTypePCD = "pointcloud/pcd"

	// MimeTypeQOI is for .qoi "Quite OK Image" for lossless, fast encoding/decoding.
	MimeTypeQOI = "image/qoi"
)
