package main

import (
	"flag"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

var (
	pathImg   = flag.String("path", artifact.MustPath("rimage/image_2021-07-16-16-10-41.png"), "path of image to detect chessboard")
	pathEdges = flag.String("pathEdges", artifact.MustPath("rimage/edges.png"), "path of edges image in which to detect chessboard")
	conf      = flag.String("conf", "conf.json", "path of configuration for chessboard detection algorithm")
)

func TestRunChessBoardDetection(t *testing.T) {
	res := RunChessBoardDetection(*pathImg, *pathEdges, *conf)
	test.That(t, res, test.ShouldEqual, 0)
}
