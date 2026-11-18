// Package worksheet provides an interactive CLI game for learning spatialmath transformations.
package worksheet

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"go.viam.com/rdk/spatialmath"
)

// Question represents a single exercise in the worksheet.
type Question struct {
	// Setup is the Go code snippet shown to the user.
	Setup string
	// Answer is the formatted result string.
	Answer string
	// Explanation describes why the answer is what it is.
	Explanation string
	// InputPoses are the named poses to visualize before the answer is revealed.
	// Keys should match variable names in Setup (e.g. "a", "b").
	InputPoses map[string]spatialmath.Pose
	// ResultPose is the result to visualize after the answer is revealed.
	ResultPose spatialmath.Pose
}

// Level represents a cohesive set of exercises.
type Level struct {
	Number      int
	Title       string
	Description string
	Questions   []Question
}

// print helpers — this is a CLI tool, stdout output is intentional.

//nolint:forbidigo
func printLine(a ...any) { fmt.Println(a...) }

//nolint:forbidigo
func printFmt(format string, a ...any) { fmt.Printf(format, a...) }

//nolint:forbidigo
func printPrompt(s string) { fmt.Print(s) }

func waitForEnter(reader *bufio.Reader) {
	//nolint:errcheck
	reader.ReadString('\n')
}

// inputPoseLegend builds a human-readable legend mapping colors to variables.
func inputPoseLegend(poses map[string]spatialmath.Pose) string {
	if len(poses) == 0 {
		return ""
	}
	names := make([]string, 0, len(poses))
	for name := range poses {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names)+1)
	parts = append(parts, "white = origin")
	for i, name := range names {
		color := PoseColorByIndex(i)
		parts = append(parts, color+" = "+name)
	}
	return strings.Join(parts, ", ")
}

// RunLevel runs a single level interactively.
func RunLevel(reader *bufio.Reader, level Level, totalLevels int) {
	printFmt("\n=== Level %d: %s (%d/%d) ===\n",
		level.Number, level.Title, level.Number, totalLevels)
	printLine(level.Description)
	printLine()

	for i, q := range level.Questions {
		printFmt("--- Question %d of %d ---\n\n",
			i+1, len(level.Questions))

		if len(q.InputPoses) > 0 {
			DrawInputPoses(q.InputPoses)
			legend := inputPoseLegend(q.InputPoses)
			printFmt("  3D view: %s\n", legend)
			printLine("  Each pose is a 10x20x30 box so you can see orientation.")
			printLine()
		}

		printLine(q.Setup)
		printLine()
		printLine("What is the result?")
		printLine()
		printPrompt("Press Enter when you've thought about it...")
		waitForEnter(reader)

		printLine()
		printFmt("  Answer:\n    %s\n", q.Answer)

		if q.ResultPose != nil {
			DrawResult(q.ResultPose)
			printLine()
			printLine("  3D view: red = result (added to the scene)")
		}

		if q.Explanation != "" {
			printLine()
			printFmt("  %s\n", q.Explanation)
		}

		printLine()
		if i < len(level.Questions)-1 {
			printPrompt("Press Enter for next question...")
		} else {
			printPrompt("Press Enter to finish this level...")
		}
		waitForEnter(reader)
		printLine()
	}

	ClearVisualization()
	printFmt("=== Level %d Complete! ===\n\n", level.Number)
}

// RunAllLevels runs all levels sequentially.
func RunAllLevels(reader *bufio.Reader, levels []Level) {
	printLine("=== Spatialmath Worksheet Game ===")
	printFmt("Learn spatial transformations through %d levels.\n",
		len(levels))
	printLine()
	printLine("3D Visualization (requires motion-tools running):")
	printLine("  - Poses drawn as 10x20x30 mm boxes (asymmetric)")
	printLine("  - white  = origin (reference)")
	printLine("  - blue   = 1st input pose")
	printLine("  - green  = 2nd input pose")
	printLine("  - yellow = 3rd input pose")
	printLine("  - red    = result (after reveal)")
	printLine("  Variable names shown in each question's legend.")
	printLine()
	printPrompt("Press Enter to begin...")
	waitForEnter(reader)

	for _, level := range levels {
		RunLevel(reader, level, len(levels))
		if level.Number < len(levels) {
			printPrompt("Press Enter to continue to next level...")
			waitForEnter(reader)
		}
	}

	printLine("=== All levels complete! ===")
}
