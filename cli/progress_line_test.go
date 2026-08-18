package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestProgressLine(t *testing.T) {
	tail := func(n int) string { return fmt.Sprintf(" (%d %s)", n, pluralize(n, "row")) }
	// The '%' proves prefix never reaches a format function.
	const prefix = "  rate%s Readings: f.ndjson"

	t.Run("redraws in place on a terminal", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: prefix, tail: tail, terminal: true}
		line.start("")
		line.update(100)
		line.finish(250)

		test.That(t, buf.String(), test.ShouldEqual,
			prefix+"\r"+prefix+" (100 rows)"+"\r"+prefix+" (250 rows)\n")
	})

	t.Run("off a terminal writes the line once", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: prefix, tail: tail}
		line.start("")
		line.update(100) // dropped: no cursor to move
		line.finish(250)

		test.That(t, buf.String(), test.ShouldEqual, prefix+" (250 rows)\n")
	})

	t.Run("pads a shorter redraw over a longer one", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{
			w: &buf, terminal: true,
			tail: func(n int) string { return strings.Repeat("x", n) },
		}
		line.update(5)
		line.finish(2)

		test.That(t, buf.String(), test.ShouldEqual, "\rxxxxx"+"\rxx   \n")
	})

	t.Run("the waiting placeholder is overwritten by the first count", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: "  ", tail: tail, terminal: true}
		line.waiting("starting download...")
		line.update(2)

		placeholder, redraw, found := strings.Cut(buf.String(), "\r")
		test.That(t, found, test.ShouldBeTrue)
		test.That(t, placeholder, test.ShouldEqual, "  starting download...")
		test.That(t, strings.TrimRight(redraw, " "), test.ShouldEqual, "   (2 rows)")
		// Blanked out to the placeholder's width, so none of it shows through.
		test.That(t, len(redraw), test.ShouldEqual, len(placeholder))
	})

	t.Run("no waiting placeholder off a terminal", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: "  ", tail: tail}
		line.waiting("starting download...")

		test.That(t, buf.String(), test.ShouldEqual, "")
	})

	// A carriage return only returns to the start of the current visual row, so a line long enough
	// to wrap cannot be redrawn.
	t.Run("detects a line too wide to redraw", func(t *testing.T) {
		narrow := &progressLine{maxWidth: 35}
		test.That(t, narrow.fitsOneRow("  short"), test.ShouldBeTrue)
		test.That(t, narrow.fitsOneRow(strings.Repeat("x", 40)), test.ShouldBeFalse)

		unknown := &progressLine{maxWidth: 0}
		test.That(t, unknown.fitsOneRow(strings.Repeat("x", 200)), test.ShouldBeTrue)
	})

	// The placeholder is longer than any count, so it can overflow a line the count fits on.
	t.Run("drops a placeholder that would wrap", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: "  sensor-1 Readings", tail: tail, terminal: true, maxWidth: 35}
		line.start(": starting download...")

		test.That(t, buf.String(), test.ShouldEqual, "  sensor-1 Readings")
	})

	t.Run("writes a whole line when start was skipped", func(t *testing.T) {
		var buf bytes.Buffer
		line := &progressLine{w: &buf, prefix: prefix, tail: tail}
		line.finish(1)

		test.That(t, buf.String(), test.ShouldEqual, prefix+" (1 row)\n")
	})
}
