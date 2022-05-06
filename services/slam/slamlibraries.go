// Package slam implements simultaneous localization and mapping
package slam

// slamType is the type of slam being performed, either sparse or dense.
type slamType uint8

// The types of SLAM avaialble for SLAM library definitions.
const (
	denseSLAM = slamType(iota)
	sparseSLAM
)

var slamLibraries = map[string]metadata{
	"cartographer": cartographerMetadata,
	"orbslamv3":    orbslamv3Metadata,
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	SLAMType:       denseSLAM,
	SlamMode:       map[string]bool{"2d": true, "3d": false},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	SLAMType:       sparseSLAM,
	SlamMode:       map[string]bool{"mono": true, "rgbd": true},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	SLAMType       slamType
	SlamMode       map[string]bool
	BinaryLocation string
}
