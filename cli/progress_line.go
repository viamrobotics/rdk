package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// progressLine renders one line whose only changing part is a running count. On a terminal it
// rewrites the line in place; elsewhere it writes once, when the count is final.
//
// Keep the prefix short. A carriage return only returns to the start of the current visual row, so
// a line long enough to wrap cannot be redrawn.
type progressLine struct {
	w        io.Writer
	prefix   string
	tail     func(count int) string // the changing part, e.g. " (1234 rows)"
	terminal bool
	started  bool
	width    int // widest drawn, in runes
	maxWidth int // terminal columns; 0 when unknown
}

func newProgressLine(w io.Writer, prefix string, tail func(count int) string) *progressLine {
	l := &progressLine{w: w, prefix: prefix, tail: tail, terminal: isTerminalOutput()}
	if !l.terminal {
		return l
	}
	if cols, err := terminalWidth(); err == nil {
		l.maxWidth = cols
	}
	// Writing once is worse than a live count, but better than scribbling over a wrapped line or
	// truncating away the count itself.
	if !l.fitsOneRow(prefix + tail(widestCount)) {
		l.terminal = false
	}
	return l
}

// widestCount is the count assumed when checking the line fits.
const widestCount = 999_999

// fitsOneRow reports whether s stays on a single visual row. An unknown width is treated as
// fitting, since guessing wrong the other way disables the live count for everyone.
func (l *progressLine) fitsOneRow(s string) bool {
	return l.maxWidth <= 0 || utf8.RuneCountInString(s) < l.maxWidth
}

// render concatenates rather than formats: prefix is caller-supplied and may contain a '%'.
func (l *progressLine) render(count int) string {
	return l.prefix + l.tail(count)
}

// start names the work before counting begins, plus a placeholder on a terminal that the first
// count overwrites.
func (l *progressLine) start(placeholder string) {
	l.started = true
	drawn := l.prefix
	// The placeholder is longer than any count, so it can overflow a line the count fits on.
	if l.terminal && l.fitsOneRow(l.prefix+placeholder) {
		drawn += placeholder
	}
	l.width = utf8.RuneCountInString(drawn)
	fmt.Fprint(l.w, drawn) //nolint:errcheck
}

// waiting is start for a caller that cannot commit to the line yet, because the count may end up
// zero. Piped, nothing would ever overwrite it, so it draws nothing.
func (l *progressLine) waiting(placeholder string) {
	if l.terminal {
		l.start(placeholder)
	}
}

func (l *progressLine) update(count int) {
	if l.terminal {
		fmt.Fprint(l.w, "\r"+l.padded(count)) //nolint:errcheck
	}
}

// padded blank-fills to the widest drawn so a shorter render cannot leave an earlier one's tail
// behind; count and tail both come from the caller, so neither is assumed to grow. Runes, not
// display cells, so a double-width glyph costs a stray character.
func (l *progressLine) padded(count int) string {
	drawn := l.render(count)
	if gap := l.width - utf8.RuneCountInString(drawn); gap > 0 {
		drawn += strings.Repeat(" ", gap)
	}
	l.width = utf8.RuneCountInString(drawn)
	return drawn
}

func (l *progressLine) finish(count int) {
	switch {
	case l.terminal:
		fmt.Fprint(l.w, "\r"+l.padded(count)+"\n") //nolint:errcheck
	case l.started:
		// prefix is already on the line.
		fmt.Fprint(l.w, l.tail(count)+"\n") //nolint:errcheck
	default:
		fmt.Fprint(l.w, l.render(count)+"\n") //nolint:errcheck
	}
}

// abandon closes the line so whatever follows starts on its own.
func (l *progressLine) abandon() {
	fmt.Fprintln(l.w) //nolint:errcheck
}

// erase removes the line, for a caller replacing a running total with a fuller breakdown.
func (l *progressLine) erase() {
	if l.terminal {
		fmt.Fprint(l.w, "\r"+strings.Repeat(" ", l.width)+"\r") //nolint:errcheck
	}
}
