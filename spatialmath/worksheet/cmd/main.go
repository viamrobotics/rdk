// Package main provides an interactive CLI game for learning spatialmath transformations.
//
// Run with: go run ./spatialmath/worksheet/cmd
// Jump to a level: go run ./spatialmath/worksheet/cmd --level 3
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"go.viam.com/rdk/spatialmath/worksheet"
)

func main() {
	levelFlag := flag.Int("level", 0, "Jump to a specific level (1-5). 0 runs all levels.")
	flag.Parse()

	levels := worksheet.MakeLevels()
	reader := bufio.NewReader(os.Stdin)

	if *levelFlag < 0 || *levelFlag > len(levels) {
		fmt.Fprintf(os.Stderr, "Invalid level %d. Choose 1-%d, or 0 for all.\n", *levelFlag, len(levels))
		os.Exit(1)
	}

	if *levelFlag > 0 {
		worksheet.RunLevel(reader, levels[*levelFlag-1], len(levels))
	} else {
		worksheet.RunAllLevels(reader, levels)
	}
}
