package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestUpdateModelsAction(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)

	dir := filepath.Dir(filename)
	binaryPath := testutils.BuildTempModule(t, "./module/testmodule")
	metaPath := dir + "/../module/testmodule/test_meta.json"
	expectedMetaPath := dir + "/../module/testmodule/expected_meta.json"

	// create a temporary file where we can write the module's metadata
	metaFile, err := os.OpenFile(metaPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, metaFile.Close(), test.ShouldBeNil)
		test.That(t, os.Remove(metaPath), test.ShouldBeNil)
	}()

	_, err = metaFile.WriteString("{}")
	test.That(t, err, test.ShouldBeNil)

	flags := map[string]any{"binary": binaryPath, "module": metaPath}
	cCtx, _, _, errOut := setup(&inject.AppServiceClient{}, nil, nil, flags, "")
	test.That(t, UpdateModelsAction(cCtx, parseStructFromCtx[updateModelsArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)

	// verify that models added to meta.json are equivalent to those defined in expected_meta.json
	metaModels, err := loadManifest(metaPath)
	test.That(t, err, test.ShouldBeNil)

	expectedMetaModels, err := loadManifest(expectedMetaPath)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sameModels(metaModels.Models, expectedMetaModels.Models), test.ShouldBeTrue)
}

func TestValidateModelAPI(t *testing.T) {
	err := validateModelAPI("rdk:component:x")
	test.That(t, err, test.ShouldBeNil)
	err = validateModelAPI("rdk:service:x")
	test.That(t, err, test.ShouldBeNil)
	err = validateModelAPI("rdk:unknown:x")
	test.That(t, err, test.ShouldHaveSameTypeAs, unknownRdkAPITypeError{})
	err = validateModelAPI("other:unknown:x")
	test.That(t, err, test.ShouldHaveSameTypeAs, unknownRdkAPITypeError{})
	err = validateModelAPI("rdk:component")
	test.That(t, err, test.ShouldNotBeNil)
	err = validateModelAPI("other:component:$x")
	test.That(t, err, test.ShouldNotBeNil)
	err = validateModelAPI("other:component:x_")
	test.That(t, err, test.ShouldBeNil)
}

func TestGetMarkdownContent(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)

	dir := filepath.Dir(filename)

	tests := []struct {
		name             string
		filepath         string
		shouldContain    []string
		shouldNotContain []string
		shouldErrorWith  string
		totalChars       int
	}{
		{
			name:     "content types",
			filepath: filepath.Join(dir, "test_data/markdown_content_types.md"),
			shouldContain: []string{
				"This is a simple text paragraph",
				"def hello():",
				"print(\"Hello world!\")",
				"Item 1",
				"Item 2",
				"Item 3",
				"Header 1",
				"Cell 1",
			},
			totalChars: 337,
		},
		{
			name:     "no sections",
			filepath: filepath.Join(dir, "test_data/markdown_no_sections.md"),
			shouldContain: []string{
				"This is a simple markdown file with no sections",
				"It just contains plain text content",
				"Here's a second paragraph",
			},
			totalChars: 170,
		},
		{
			name:     "nested sections with anchor",
			filepath: filepath.Join(dir, "test_data/markdown_four_nested_sections.md#second-level"),
			shouldContain: []string{
				"Some content in second level",
				"Nested content in third level",
				"This is the deepest nested section",
			},
			shouldNotContain: []string{
				"## Second Level",
				"Content in another second level section",
				"Some special characters content",
			},
			totalChars: 167,
		},
		{
			name:     "special characters header",
			filepath: filepath.Join(dir, "test_data/markdown_content_types.md#section-with-pecial-chars"),
			shouldContain: []string{
				"Some special characters content.",
			},
			totalChars: 32,
		},
		{
			name:          "no content",
			filepath:      filepath.Join(dir, "test_data/markdown_no_content.md"),
			shouldContain: []string{},
			totalChars:    0,
		},
		{
			name:            "non-existent file",
			filepath:        "non_existent_file.md",
			shouldContain:   []string{},
			shouldErrorWith: "failed to read markdown file at non_existent_file.md: open non_existent_file.md: no such file or directory",
			totalChars:      0,
		},
		{
			name:            "non-existent anchor",
			filepath:        filepath.Join(dir, "test_data/markdown_content_types.md#non-existent-anchor"),
			shouldContain:   []string{},
			shouldErrorWith: "section #non-existent-anchor not found in",
			totalChars:      0,
		},
		{
			name:     "duplicate headers",
			filepath: filepath.Join(dir, "test_data/markdown_duplicate_headers.md#duplicate-section"),
			shouldContain: []string{
				"This is the first duplicate section.",
			},
			shouldNotContain: []string{
				"This is the second duplicate section.",
				"This is the third duplicate section.",
			},
			totalChars: 37,
		},
		{
			name:     "duplicate headers with different anchor",
			filepath: filepath.Join(dir, "test_data/markdown_duplicate_headers.md#duplicate-section-1"),
			shouldContain: []string{
				"This is the second duplicate section.",
			},
			shouldNotContain: []string{
				"This is the first duplicate section.",
				"This is the third duplicate section.",
			},
			totalChars: 38,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := getMarkdownContent(tt.filepath)
			if tt.shouldErrorWith != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.shouldErrorWith)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}

			// Test that all required strings are present
			for _, str := range tt.shouldContain {
				test.That(t, content, test.ShouldContainSubstring, str)
			}

			// Test that all strings that should not be present are not present
			for _, str := range tt.shouldNotContain {
				test.That(t, content, test.ShouldNotContainSubstring, str)
			}

			// Test total character count
			test.That(t, len(content), test.ShouldEqual, tt.totalChars)
		})
	}
}

func TestGenerateAnchor(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "basic header",
			header:   "## Simple Header",
			expected: "simple-header",
		},
		{
			name:     "header with special characters",
			header:   "### Header with $pecial Ch@rs!",
			expected: "header-with-pecial-chrs",
		},
		{
			name:     "header with multiple spaces",
			header:   "##   Multiple   Spaces  ",
			expected: "multiple-spaces",
		},
		{
			name:     "header with consecutive hyphens",
			header:   "## Header--with---hyphens",
			expected: "header-with-hyphens",
		},
		{
			name:     "header with mixed case",
			header:   "## UPPER lower MiXeD",
			expected: "upper-lower-mixed",
		},
		{
			name:     "header with numbers",
			header:   "## Header 123 Numbers",
			expected: "header-123-numbers",
		},
		{
			name:     "header with leading/trailing hyphens",
			header:   "## -Header with hyphens-",
			expected: "header-with-hyphens",
		},
		{
			name:     "header with only special characters",
			header:   "## @#$%^&*",
			expected: "",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateAnchor(tt.header)
			test.That(t, result, test.ShouldEqual, tt.expected)
		})
	}
}
