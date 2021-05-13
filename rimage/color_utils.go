package rimage

import (
	"html/template"
	"image/color"
	"io/ioutil"
	"sort"
	"strings"
)

func ConvertToNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

type TheColorModel struct {
}

func (cm *TheColorModel) Convert(c color.Color) color.Color {
	return NewColorFromColor(c)
}

type ColorDiff struct {
	Left  Color
	Right Color
	Diff  float64
}

type ColorDiffs struct {
	all        []ColorDiff
	seenCombos map[uint64]bool // this a + b for now, which is wrong, but..
}

func (x *ColorDiffs) Len() int {
	return len(x.all)
}

func (x *ColorDiffs) Less(i, j int) bool {
	return x.all[i].Diff < x.all[j].Diff
}

func (x *ColorDiffs) Swap(i, j int) {
	t := x.all[i]
	x.all[i] = x.all[j]
	x.all[j] = t
}

func (x *ColorDiffs) Sort() {
	sort.Sort(x)
}

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

func (x *ColorDiffs) WriteTo(fn string) error {
	return ioutil.WriteFile(fn, []byte(x.output()), 0640)
}

func ComputeColorDiffs(all []Color) ColorDiffs {
	diffs := ColorDiffs{}
	for i, c := range all {
		for _, c2 := range all[i+1:] {
			diffs.Add(c, c2)
		}
	}
	return diffs
}
