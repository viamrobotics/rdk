package slam

// TODO 05/12/2022: Both type and constants will be deprecated when data ingestion via GRPC is available
// only being used now for linter issues.
type (
	// Library type used for slam.
	Library uint8
	// Mode holds the modes a slam model can use.
	Mode string
)

const (
	// Dense is a Library type.
	Dense = Library(iota)
	// Sparse is a Library type.
	Sparse
	// Mono is a mode a slam model can use.
	Mono = Mode("mono")
	// Rgbd is a mode a slam model can use.
	Rgbd = Mode("rgbd")
	// Dim2d is a mode a slam model can use.
	Dim2d = Mode("2d")
	// Dim3d is a mode a slam model can use.
	Dim3d = Mode("3d")
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
	SlamMode:       map[string]Mode{"2d": Dim2d},
	BinaryLocation: "carto_grpc_server",
}

var orbslamv3Metadata = LibraryMetadata{
	AlgoName:       "orbslamv3",
	AlgoType:       Sparse,
	SlamMode:       map[string]Mode{"mono": Mono, "rgbd": Rgbd},
	BinaryLocation: "orb_grpc_server",
}

// LibraryMetadata contains all pertinent information for defining a SLAM library/algorithm including the
// Sparse/Dense definition.
type LibraryMetadata struct {
	AlgoName       string
	AlgoType       Library
	SlamMode       map[string]Mode
	BinaryLocation string
}
