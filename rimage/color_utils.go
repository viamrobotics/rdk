package rimage

import (
	"html/template"
	"image/color"
	"io/ioutil"
	"strings"

	"github.com/lucasb-eyer/go-colorful"
)

func ConvertToNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// -----

type TheColorModel struct {
}

func (cm *TheColorModel) Convert(c color.Color) color.Color {
	return NewColorFromColor(c)
}

// --------

type ColorDiff struct {
	Left  Color
	Right Color
	Diff  float64
}

type ColorDiffs []ColorDiff

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
	err = tt.Execute(&w, x)
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
			d := c.Distance(c2)
			diffs = append(diffs, ColorDiff{c, c2, d})
		}
	}
	return diffs
}
