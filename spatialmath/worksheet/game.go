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
	// Keys should match variable names in Setup (e.g. "a", "b", "rot", "endEffector").
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

// inputPoseLegend builds a human-readable legend of which color represents which variable.
func inputPoseLegend(poses map[string]spatialmath.Pose) string {
	if len(poses) == 0 {
		return ""
	}
	// Sort keys for deterministic output.
	names := make([]string, 0, len(poses))
	for name := range poses {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names)+1)
	parts = append(parts, "white = origin")
	for _, name := range names {
		color := PoseColor(name)
		parts = append(parts, fmt.Sprintf("%s = %s", color, name))
	}
	return strings.Join(parts, ", ")
}

// RunLevel runs a single level interactively.
func RunLevel(reader *bufio.Reader, level Level, totalLevels int) {
	fmt.Printf("\n=== Level %d: %s (%d/%d) ===\n", level.Number, level.Title, level.Number, totalLevels)
	fmt.Println(level.Description)
	fmt.Println()

	for i, q := range level.Questions {
		fmt.Printf("--- Question %d of %d ---\n\n", i+1, len(level.Questions))

		// Visualize inputs and print legend
		if len(q.InputPoses) > 0 {
			DrawInputPoses(q.InputPoses)
			legend := inputPoseLegend(q.InputPoses)
			fmt.Printf("  3D view: %s\n", legend)
			fmt.Println("  Each pose is shown as a 10x20x30 box so you can see its orientation.")
			fmt.Println()
		}

		// Show the code
		fmt.Println(q.Setup)
		fmt.Println()
		fmt.Println("What is the result?")
		fmt.Println()
		fmt.Print("Press Enter when you've thought about it...")
		reader.ReadString('\n')

		// Reveal answer
		fmt.Println()
		fmt.Printf("  Answer:\n    %s\n", q.Answer)

		// Visualize result
		if q.ResultPose != nil {
			DrawResult(q.ResultPose)
			fmt.Println()
			fmt.Println("  3D view: red = result (added to the scene)")
		}

		// Explanation
		if q.Explanation != "" {
			fmt.Println()
			fmt.Printf("  %s\n", q.Explanation)
		}

		fmt.Println()
		if i < len(level.Questions)-1 {
			fmt.Print("Press Enter for next question...")
		} else {
			fmt.Print("Press Enter to finish this level...")
		}
		reader.ReadString('\n')
		fmt.Println()
	}

	ClearVisualization()
	fmt.Printf("=== Level %d Complete! ===\n\n", level.Number)
}

// RunAllLevels runs all levels sequentially.
func RunAllLevels(reader *bufio.Reader, levels []Level) {
	fmt.Println("=== Spatialmath Worksheet Game ===")
	fmt.Printf("Learn spatial transformations through %d levels of exercises.\n", len(levels))
	fmt.Println()
	fmt.Println("3D Visualization (requires motion-tools running):")
	fmt.Println("  - Each pose is drawn as a 10x20x30 mm box (asymmetric so you can see orientation)")
	fmt.Println("  - white  = origin (always shown for reference)")
	fmt.Println("  - blue   = first input pose (a)")
	fmt.Println("  - green  = second input pose (b)")
	fmt.Println("  - yellow = third input pose (c)")
	fmt.Println("  - red    = result (shown after you reveal the answer)")
	fmt.Println()
	fmt.Print("Press Enter to begin...")
	reader.ReadString('\n')

	for _, level := range levels {
		RunLevel(reader, level, len(levels))
		if level.Number < len(levels) {
			fmt.Print("Press Enter to continue to next level...")
			reader.ReadString('\n')
		}
	}

	fmt.Println("=== All levels complete! ===")
}
