// Package slam implements simultaneous localization and mapping
package slam

// TODO 05/12/2022: Both type and consts will be deprecated when data ingestion via GRPC is available
// only being used now for linter issues.
type (
	slamLibrary uint8
	mode        string
)

const (
	dense = slamLibrary(iota)
	sparse
	mono  = mode("mono")
	rgbd  = mode("rgbd")
	dim2d = mode("2d")
	dim3d = mode("3d")
)

var slamLibraries = map[string]metadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	AlgoType:       dense,
	SlamMode:       map[string]mode{"2d": dim2d},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	AlgoType:       sparse,
	SlamMode:       map[string]mode{"mono": mono, "rgbd": rgbd},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	AlgoType       slamLibrary
	SlamMode       map[string]mode
	BinaryLocation string
}
