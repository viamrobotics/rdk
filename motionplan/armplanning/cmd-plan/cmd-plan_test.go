package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/viam-labs/motion-tools/draw"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// writeFiles creates each named file, empty, under dir, creating parent directories as needed.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		test.That(t, os.MkdirAll(filepath.Dir(path), 0o755), test.ShouldBeNil)
		test.That(t, os.WriteFile(path, nil, 0o600), test.ShouldBeNil)
	}
}

func TestCollectRequestFiles(t *testing.T) {
	t.Run("no arguments", func(t *testing.T) {
		_, err := collectRequestFiles(nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("files are taken in the order given", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "b.json", "a.json")

		files, err := collectRequestFiles([]string{
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "a.json"),
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "a.json"),
		})
	})

	t.Run("a file that is not json is still taken as given", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "capture.txt")

		files, err := collectRequestFiles([]string{filepath.Join(dir, "capture.txt")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{filepath.Join(dir, "capture.txt")})
	})

	t.Run("a missing path errors", func(t *testing.T) {
		_, err := collectRequestFiles([]string{filepath.Join(t.TempDir(), "nope.json")})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("a directory expands alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "c.json", "a.json", "b.json")

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "a.json"),
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "c.json"),
		})
	})

	t.Run("a directory keeps only json entries, case-insensitively", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "a.json", "b.JSON", "notes.txt", "plan.json.bak", "no-extension")

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "a.json"),
			filepath.Join(dir, "b.JSON"),
		})
	})

	t.Run("a directory is not descended into", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "a.json", filepath.Join("nested", "b.json"))

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{filepath.Join(dir, "a.json")})
	})

	t.Run("a directory with no json errors", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "notes.txt")

		_, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, dir)
	})

	t.Run("an empty directory errors", func(t *testing.T) {
		_, err := collectRequestFiles([]string{t.TempDir()})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("files and directories mix, each expanded in place", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "lead.json", filepath.Join("seq", "b.json"), filepath.Join("seq", "a.json"), "tail.json")

		files, err := collectRequestFiles([]string{
			filepath.Join(dir, "lead.json"),
			filepath.Join(dir, "seq"),
			filepath.Join(dir, "tail.json"),
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "lead.json"),
			filepath.Join(dir, "seq", "a.json"),
			filepath.Join(dir, "seq", "b.json"),
			filepath.Join(dir, "tail.json"),
		})
	})
}

// cmd-plan is meant to be runnable as `go run motionplan/armplanning/cmd-plan/cmd-plan.go`, which
// compiles only the named file. A second source file builds fine as a package but breaks that.
func TestSingleSourceFile(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	test.That(t, err, test.ShouldBeNil)
	sources = slices.DeleteFunc(sources, func(name string) bool { return strings.HasSuffix(name, "_test.go") })
	test.That(t, sources, test.ShouldResemble, []string{"cmd-plan.go"})
}

func TestFrameColors(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("test")
	box := func(name string) spatialmath.Geometry {
		t.Helper()
		geom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, name+"_box")
		test.That(t, err, test.ShouldBeNil)
		return geom
	}
	must := func(frame referenceframe.Frame, err error) referenceframe.Frame {
		t.Helper()
		test.That(t, err, test.ShouldBeNil)
		return frame
	}
	add := func(frame, parent referenceframe.Frame) referenceframe.Frame {
		t.Helper()
		test.That(t, fs.AddFrame(frame, parent), test.ShouldBeNil)
		return frame
	}
	slideLimit := referenceframe.Limit{Min: -100, Max: 100}
	zero := spatialmath.NewZeroPose()

	add(must(referenceframe.NewStaticFrameWithGeometry("table", zero, box("table"))), fs.World())
	rail := add(must(referenceframe.NewTranslationalFrameWithGeometry("rail", r3.Vector{X: 1}, slideLimit, box("rail"))), fs.World())
	add(must(referenceframe.NewTranslationalFrameWithGeometry("slide", r3.Vector{Z: 1}, slideLimit, box("slide"))), rail)
	mount := add(must(referenceframe.NewStaticFrame("mount", zero)), rail)
	add(must(referenceframe.NewStaticFrameWithGeometry("gripper", zero, box("gripper"))), mount)
	add(must(referenceframe.NewStaticFrameWithGeometry("camera", zero, box("camera"))), rail)
	add(must(referenceframe.NewStaticFrameWithGeometry("fixture", zero, box("fixture"))), fs.World())
	add(must(referenceframe.NewStaticFrameWithGeometry("bin_origin", zero, box("bin"))), fs.World())
	add(must(referenceframe.NewStaticFrameWithGeometry("plate", zero, box("plate"))), fs.World())
	add(must(referenceframe.NewStaticFrameWithGeometry("right-wall", zero, box("right-wall"))), fs.World())

	constraints := &motionplan.Constraints{CollisionSpecification: []motionplan.CollisionSpecification{{
		Allows: []motionplan.CollisionSpecificationAllowedFrameCollisions{
			{Frame1: "fixture", Frame2: "gripper"},
			{Frame1: "camera", Frame2: "bin"},
			{Frame1: "plate_box", Frame2: "rail"},
			{Frame1: "right-wall", Frame2: "rail"},
		},
	}}}
	allowed := allowedCollisionNames(constraints)

	colors, err := frameColors(fs, referenceframe.NewZeroInputs(fs), allowed)
	test.That(t, err, test.ShouldBeNil)

	// "mount" has no geometry, so it gets no entry; carried frames are colored in name order.
	// Only static frames switch to the allowed-collision color; "fixture" matches by frame name,
	// "bin_origin" through its component name, and "plate" through its geometry label. Walls keep
	// their transparent gray even when allowed to collide.
	see := draw.ColorFromName(allowedCollisionColor).SetAlpha(allowedCollisionAlpha)
	test.That(t, colors, test.ShouldResemble, map[string]draw.Color{
		"right-wall": draw.NewColor(draw.WithRGBA(128, 128, 128, 40)),
		"table":      draw.ColorFromName(staticColor),
		"fixture":    see,
		"bin_origin": see,
		"plate":      see,
		"rail":       draw.ColorFromName(actuatedColor),
		"slide":      draw.ColorFromName(actuatedColor),
		"camera":     draw.ColorFromName(carriedPalette[0]),
		"gripper":    draw.ColorFromName(carriedPalette[1]),
	})

	again, err := frameColors(fs, referenceframe.NewZeroInputs(fs), allowed)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, again, test.ShouldResemble, colors)

	t.Run("obstacles", func(t *testing.T) {
		worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
			referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{box("wall"), box("floor")}),
		}, nil)
		test.That(t, err, test.ShouldBeNil)

		obstacles, err := obstacleColors(worldState, fs, referenceframe.NewZeroInputs(fs), map[string]bool{"floor_box": true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, obstacles, test.ShouldResemble, []draw.Color{draw.ColorFromName(staticColor), see})
	})

	t.Run("nil constraints", func(t *testing.T) {
		test.That(t, allowedCollisionNames(nil), test.ShouldBeEmpty)
	})
}
