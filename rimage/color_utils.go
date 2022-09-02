package rimage

import (
	"html/template"
	"image/color"
	"os"
	"sort"
	"strings"
)

// ConvertToNRGBA converts a go color to an NRGBA color.
func ConvertToNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// TheColorModel represents our Color type as a model to be
// used for color conversions in color.Color.
type TheColorModel struct{}

// Convert converts the given color into our Color type but
// still returns it as a go color.
func (cm *TheColorModel) Convert(c color.Color) color.Color {
	return NewColorFromColor(c)
}

// ColorDiff TODO.
type ColorDiff struct {
	Left  Color
	Right Color
	Diff  float64
}

// ColorDiffs TODO.
type ColorDiffs struct {
	all        []ColorDiff
	seenCombos map[uint64]bool // this a + b for now, which is wrong, but..
}

// Len returns the number of diffs.
func (x *ColorDiffs) Len() int {
	return len(x.all)
}

// Less returns if one diff is less than another based on its diff value.
func (x *ColorDiffs) Less(i, j int) bool {
	return x.all[i].Diff < x.all[j].Diff
}

// Swap swaps two diffs positionally.
func (x *ColorDiffs) Swap(i, j int) {
	x.all[i], x.all[j] = x.all[j], x.all[i]
}

// Sort sorts the diffs based on satisfying the Sort interface above.
func (x *ColorDiffs) Sort() {
	sort.Sort(x)
}

// AddD TODO.
func (x *ColorDiffs) AddD(a, b Color, d float64) {
	if x.seenCombos == nil {
		x.seenCombos = map[uint64]bool{}
	}

	t := uint64(a) + uint64(b)
	if x.seenCombos[t] {
		return
	}
	x.all = append(x.all, ColorDiff{a, b, d})
	x.seenCombos[t] = true
}

// Add TODO.
func (x *ColorDiffs) Add(a, b Color) {
	d := a.Distance(b)
	x.AddD(a, b, d)
}

func (x *ColorDiffs) output() string {
	t := "<html><body><table>" +
		"{{ range .}}" +
		"<tr>" +
		"<td style='background-color:{{.Left.Hex}}'>{{ .Left.Hex }}&nbsp;</td>" +
		"<td style='background-color:{{.Right.Hex}}'>{{ .Right.Hex }}&nbsp;</td>" +
		"<td>{{ .Diff }}</td>" +
		"</tr>" +
		"{{end}}" +
		"</table></body></html>"

	tt, err := template.New("temp").Parse(t)
	if err != nil {
		panic(err)
	}

	w := strings.Builder{}
	err = tt.Execute(&w, x.all)
	if err != nil {
		panic(err)
	}
	return w.String()
}

// WriteTo writes the diff information out to a file.
func (x *ColorDiffs) WriteTo(fn string) error {
	//nolint:gosec
	return os.WriteFile(fn, []byte(x.output()), 0o640)
}

// ComputeColorDiffs computes the different between the all of the colors given.
func ComputeColorDiffs(all []Color) ColorDiffs {
	diffs := ColorDiffs{}
	for i, c := range all {
		for _, c2 := range all[i+1:] {
			diffs.Add(c, c2)
		}
	}
	return diffs
}
