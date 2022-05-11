// Package slam implements simultaneous localization and mapping
package slam

type (
	slamLibrary uint8
	fileType    uint8
)

const (
	cartographer = slamLibrary(iota)
	orbslamv3
	mono = fileType(iota)
	rgbd
	twod
)

var slamLibraries = map[string]metadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	AlgoType:       cartographer,
	SlamMode:       map[string]fileType{"2d": twod},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	AlgoType:       orbslamv3,
	SlamMode:       map[string]fileType{"mono": mono, "rgbd": rgbd},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	AlgoType       slamLibrary
	SlamMode       map[string]fileType
	BinaryLocation string
}
