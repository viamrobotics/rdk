package mpserver

import (
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestBuildPlanTree(t *testing.T) {
	// Ordered the way filepath.WalkDir yields them: lexical, depth-first.
	tree := buildPlanTree("mplans", []string{
		"mplans/abc-123/a.json",
		"mplans/abc-123/b.json",
		"mplans/abc-123/nested/deep.json",
		"mplans/top.json",
		"mplans/zzz/c.json",
	})

	test.That(t, tree.Name, test.ShouldEqual, "mplans")
	test.That(t, tree.Path, test.ShouldEqual, "mplans")
	test.That(t, tree.TotalFiles, test.ShouldEqual, 5)

	test.That(t, len(tree.Files), test.ShouldEqual, 1)
	test.That(t, tree.Files[0].Name, test.ShouldEqual, "top.json")
	test.That(t, tree.Files[0].Path, test.ShouldEqual, "mplans/top.json")

	test.That(t, len(tree.Subdirs), test.ShouldEqual, 2)
	abc, zzz := tree.Subdirs[0], tree.Subdirs[1]

	test.That(t, abc.Name, test.ShouldEqual, "abc-123")
	test.That(t, abc.Path, test.ShouldEqual, "mplans/abc-123")
	// Counts are recursive, so abc-123 includes the file under nested/.
	test.That(t, abc.TotalFiles, test.ShouldEqual, 3)
	test.That(t, len(abc.Files), test.ShouldEqual, 2)
	test.That(t, abc.Files[0].Name, test.ShouldEqual, "a.json")
	test.That(t, abc.Files[1].Name, test.ShouldEqual, "b.json")

	test.That(t, len(abc.Subdirs), test.ShouldEqual, 1)
	nested := abc.Subdirs[0]
	test.That(t, nested.Path, test.ShouldEqual, "mplans/abc-123/nested")
	test.That(t, nested.TotalFiles, test.ShouldEqual, 1)
	test.That(t, nested.Files[0].Path, test.ShouldEqual, "mplans/abc-123/nested/deep.json")

	test.That(t, zzz.Name, test.ShouldEqual, "zzz")
	test.That(t, zzz.TotalFiles, test.ShouldEqual, 1)
	test.That(t, len(zzz.Subdirs), test.ShouldEqual, 0)
}

func TestBuildPlanTreeEmpty(t *testing.T) {
	tree := buildPlanTree("mplans", nil)
	test.That(t, tree.TotalFiles, test.ShouldEqual, 0)
	test.That(t, len(tree.Subdirs), test.ShouldEqual, 0)
	test.That(t, len(tree.Files), test.ShouldEqual, 0)
}

func TestIndexTmplFoldsSubdirectories(t *testing.T) {
	tree := buildPlanTree("mplans", []string{
		"mplans/abc-123/a.json",
		"mplans/abc-123/nested/deep.json",
		"mplans/top.json",
	})

	var sb strings.Builder
	test.That(t, indexTmpl.Execute(&sb, tree), test.ShouldBeNil)
	page := sb.String()

	// Both tags close right after data-path, i.e. neither carries `open`, so every
	// directory starts folded; the page's script is what re-opens the ones the
	// browser remembered.
	test.That(t, strings.Count(page, "<details class=\"dir\""), test.ShouldEqual, 2)
	test.That(t, strings.Contains(page, "<details class=\"dir\" data-path=\"mplans/abc-123\">"), test.ShouldBeTrue)
	test.That(t, strings.Contains(page, "<details class=\"dir\" data-path=\"mplans/abc-123/nested\">"), test.ShouldBeTrue)

	// Files at the scan root stay outside any <details> so they are visible
	// without expanding anything.
	test.That(t, strings.Index(page, "top.json"), test.ShouldBeGreaterThan, strings.LastIndex(page, "</details>"))

	test.That(t, strings.Contains(page, "/detail?file=mplans%2fabc-123%2fnested%2fdeep.json"), test.ShouldBeTrue)
}
