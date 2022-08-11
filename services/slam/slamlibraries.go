// Package slam implements simultaneous localization and mapping
package slam

// TODO 05/12/2022: Both type and constants will be deprecated when data ingestion via GRPC is available
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

// SLAMLibraries contains a map of available slam libraries.
var SLAMLibraries = map[string]LibraryMetadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = LibraryMetadata{
	AlgoName:       "cartographer",
	AlgoType:       dense,
	SlamMode:       map[string]mode{"2d": dim2d},
	BinaryLocation: "",
}

var orbslamv3Metadata = LibraryMetadata{
	AlgoName:       "orbslamv3",
	AlgoType:       sparse,
	SlamMode:       map[string]mode{"mono": mono, "rgbd": rgbd},
	BinaryLocation: "orb_grpc_server",
}

// LibraryMetadata contains all pertinent information for defining a SLAM library/algorithm including the
// sparse/dense definition.
type LibraryMetadata struct {
	AlgoName       string
	AlgoType       slamLibrary
	SlamMode       map[string]mode
	BinaryLocation string
}
