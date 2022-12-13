package main

import (
	"go.viam.com/rdk/rimage"
)

func main() {
	// I want to make a real vnd.viam.dep and save it
	width := 20
	height := 10
	dm := rimage.NewEmptyDepthMap(width, height)
	for w := 0; w < width; w++ {
		for h := 0; h < height; h++ {
			dm.Set(w, h, rimage.Depth(w*h))
		}
	}

	rimage.WriteRawDepthMapToFile(dm, "fakeDM.vnd.viam.dep")
}
