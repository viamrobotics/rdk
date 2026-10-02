package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
)

const (
	// Canonical meta.json `language` values (python | golang | cpp).
	moduleLanguagePython = "python"
	moduleLanguageGolang = "golang"
	moduleLanguageCPP    = "cpp"
)

// NormalizeModuleLanguage maps meta.json / --language values onto python|golang|cpp.
// Empty and unknown inputs return "".
func NormalizeModuleLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case moduleLanguagePython:
		return moduleLanguagePython
	case moduleLanguageGolang, golang: // generate historically stores "go"
		return moduleLanguageGolang
	case moduleLanguageCPP, "c++":
		return moduleLanguageCPP
	default:
		return ""
	}
}

func parseModuleLanguageFlag(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	lang := NormalizeModuleLanguage(raw)
	if lang == "" {
		return "", fmt.Errorf("unsupported --%s %q (want python, golang, or cpp)", moduleFlagLanguage, raw)
	}
	return lang, nil
}

func isPythonSourceHotReload(language string) bool {
	return language == moduleLanguagePython
}

// InferModuleLanguage guesses python|golang|cpp from well-known files in root.
// Returns "" when the tree is ambiguous or empty so callers can prompt or fall
// through to the existing binary-build reload path.
func InferModuleLanguage(root string) string {
	python := fileExists(filepath.Join(root, "pyproject.toml")) ||
		fileExists(filepath.Join(root, "requirements.txt")) ||
		fileExists(filepath.Join(root, "src", "main.py"))
	goLang := fileExists(filepath.Join(root, "go.mod"))
	cppLang := fileExists(filepath.Join(root, "CMakeLists.txt")) ||
		fileExists(filepath.Join(root, "conanfile.py")) ||
		fileExists(filepath.Join(root, "conanfile.txt"))

	n := 0
	var guess string
	if python {
		n++
		guess = moduleLanguagePython
	}
	if goLang {
		n++
		guess = moduleLanguageGolang
	}
	if cppLang {
		n++
		guess = moduleLanguageCPP
	}
	if n != 1 {
		return ""
	}
	return guess
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// resolveModuleLanguage prefers --language, then meta.json, then inference, then
// an interactive prompt. Unknown (empty) is not an error: old modules keep the
// binary-build reload path.
func resolveModuleLanguage(manifest *ModuleManifest, flagLanguage, sourceRoot string) (string, error) {
	flagLang, err := parseModuleLanguageFlag(flagLanguage)
	if err != nil {
		return "", err
	}
	if flagLang != "" {
		return flagLang, nil
	}
	if manifest != nil {
		if lang := NormalizeModuleLanguage(manifest.Language); lang != "" {
			return lang, nil
		}
	}
	if lang := InferModuleLanguage(sourceRoot); lang != "" {
		return lang, nil
	}
	if isInteractive() {
		return promptModuleLanguage()
	}
	return "", nil
}

func promptModuleLanguage() (string, error) {
	var language string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What language is this module written in?").
				Description("Used to choose source copy (Python) vs binary build (Go / C++). You can also set language in meta.json or pass --language.").
				Options(
					huh.NewOption("Python", moduleLanguagePython),
					huh.NewOption("Go", moduleLanguageGolang),
					huh.NewOption("C++", moduleLanguageCPP),
				).
				Value(&language),
		),
	).WithWidth(88)
	if err := form.Run(); err != nil {
		return "", errors.Wrap(err, "language prompt")
	}
	return NormalizeModuleLanguage(language), nil
}
