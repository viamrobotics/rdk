// Package slam implements simultaneous localization and mapping
package slam

type (
	slamLibrary uint8
	mode        uint8
)

const (
	cartographer = slamLibrary(iota)
	orbslamv3
	mono = mode(iota)
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
	SlamMode:       map[string]mode{"2d": twod},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	AlgoType:       orbslamv3,
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
