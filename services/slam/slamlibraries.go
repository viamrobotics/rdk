// Package slam implements simultaneous localization and mapping
package slam

var slamLibraries = map[string]metadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	SlamMode:       map[string]bool{"2d": true, "3d": false},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	SlamMode:       map[string]bool{"mono": true, "rgbd": true},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	SlamMode       map[string]bool
	BinaryLocation string
}
