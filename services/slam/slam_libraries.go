// Package slam implements simultaneous localization and mapping
// This is an Experimental package
package slam

// TODO 05/12/2022: Both type and constants will be deprecated when data ingestion via GRPC is available
// only being used now for linter issues.
type (
	// Library type used for slam.
	Library uint8
	// SubAlgo holds the modes a slam model can use.
	SubAlgo string
)

const (
	// Dense is a Library type.
	Dense = Library(iota)
	// Sparse is a Library type.
	Sparse
	// Mono is a subAlgo a slam model can use.
	Mono = SubAlgo("mono")
	// Rgbd is a subAlgo a slam model can use.
	Rgbd = SubAlgo("rgbd")
	// Dim2d is a subAlgo a slam model can use.
	Dim2d = SubAlgo("2d")
	// Dim3d is a subAlgo  model can use.
	Dim3d = SubAlgo("3d")
)

// SLAMLibraries contains a map of available slam libraries.
var SLAMLibraries = map[string]LibraryMetadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = LibraryMetadata{
	AlgoName:       "cartographer",
	AlgoType:       Dense,
	SubAlgo:        map[string]SubAlgo{"2d": Dim2d},
	BinaryLocation: "carto_grpc_server",
}

var orbslamv3Metadata = LibraryMetadata{
	AlgoName:       "orbslamv3",
	AlgoType:       Sparse,
	SubAlgo:        map[string]SubAlgo{"mono": Mono, "rgbd": Rgbd},
	BinaryLocation: "orb_grpc_server",
}

// LibraryMetadata contains all pertinent information for defining a SLAM library/algorithm including the
// Sparse/Dense definition.
type LibraryMetadata struct {
	AlgoName       string
	AlgoType       Library
	SubAlgo        map[string]SubAlgo
	BinaryLocation string
}
