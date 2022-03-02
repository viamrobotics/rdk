package vision

// Parameters3D specifies the necessary parameters for 3D object finding.
type Parameters3D struct {
	MinPtsInPlane      int
	MinPtsInSegment    int
	ClusteringRadiusMm float64
}
