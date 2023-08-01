package config

import (
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/rdk/utils"
)

// This placeholder regex matches on all strings that satisfy our criteria for a placeholder
// Example string satisfying the regex:
// ${hello}
var placeholderRegexp = regexp.MustCompile(`\$\{(?P<placeholder_key>[^\}]*)\}`)

// packagePlaceholderRegexp matches on all valid ways of specifying one of our package placeholders
// Example strings satisfying the regex:
// packages.my-COOL-ml-model/__89
// packages.modules.intel:CameraThatRocks
var packagePlaceholderRegexp = regexp.MustCompile(`packages(\.(?P<type>ml_model|module))?\.(?P<name>[\w:\/-]+)`)

func ContainsPlaceholder(s string) bool {
	return placeholderRegexp.MatchString(s)
}

func (cfg *Config) ReplacePlaceholders() error {
	var allErrs, err error
	visitor := NewPlaceholderReplacementVisitor(cfg)

	for i, s := range cfg.Services {
		cfg.Services[i].Attributes, err = walkTypedAttributes(visitor, s.Attributes)
		allErrs = multierr.Append(allErrs, err)
	}

	for i, c := range cfg.Components {
		cfg.Components[i].Attributes, err = walkTypedAttributes(visitor, c.Attributes)
		allErrs = multierr.Append(allErrs, err)
	}

	for i, c := range cfg.Modules {
		cfg.Modules[i].ExePath, err = visitor.replacePlaceholders(c.ExePath)
		allErrs = multierr.Append(allErrs, err)
	}

	return allErrs
}

func walkTypedAttributes[T any](visitor *PlaceholderReplacementVisitor, attributes T) (T, error) {
	var asIfc interface{} = attributes
	if walker, ok := asIfc.(utils.Walker); ok {
		newAttrs, err := walker.Walk(visitor)
		if err != nil {
			return attributes, err
		}
		newAttrsTyped, err := utils.AssertType[T](newAttrs)
		if err != nil {
			return attributes, err
		}
		return newAttrsTyped, nil
	}
	return attributes, errors.Errorf("passed attribute cannot be walked")
}

// PlaceholderReplacementVisitor is a visitor that replaces strings containing placeholder values with their desired values
type PlaceholderReplacementVisitor struct {
	// Map of packageName -> packageConfig
	packages map[string]PackageConfig
}

// NewPlaceholderReplacementVisitor creates a new PlaceholderReplacementVisitor.
func NewPlaceholderReplacementVisitor(cfg *Config) *PlaceholderReplacementVisitor {
	// Create the list of packages that will be used for replacement
	packages := map[string]PackageConfig{}
	// based on the given packages, generate the real filepath
	for _, config := range cfg.Packages {
		packages[config.Name] = config
	}

	return &PlaceholderReplacementVisitor{
		packages,
	}
}

// Visit implements config.Visitor.
func (v *PlaceholderReplacementVisitor) Visit(data interface{}) (interface{}, error) {
	t := reflect.TypeOf(data)

	var s string
	switch {
	case t.Kind() == reflect.String:
		s = data.(string)
	case t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.String:
		s = *data.(*string)
	default:
		return data, nil
	}

	withReplacedRefs, err := v.replacePlaceholders(s)
	if err != nil {
		return data, err
	}

	// If the input was a pointer, return a pointer.
	if t.Kind() == reflect.Ptr {
		return &withReplacedRefs, nil
	}
	return withReplacedRefs, nil
}

// replacePlaceholders replaces a string with a package path if its a valid package placeholder.
func (v *PlaceholderReplacementVisitor) replacePlaceholders(s string) (string, error) {
	var allErrors error
	// first match all possible placeholders (ex: ${hello})
	patchedStr := placeholderRegexp.ReplaceAllFunc([]byte(s), func(b []byte) []byte {
		matches := placeholderRegexp.FindSubmatch(b)
		if matches == nil {
			allErrors = multierr.Append(allErrors, errors.Errorf("failed to find substring matches for placeholder %q", string(b)))
			return b
		}
		// placeholder key is the inside of the placeholder ex "hello" for ${hello}
		placeholderKey := matches[placeholderRegexp.SubexpIndex("placeholder_key")]

		if packagePlaceholderRegexp.Match(placeholderKey) {
			replaced, err := v.replacePackagePlaceholder(placeholderKey)
			allErrors = multierr.Append(allErrors, err)
			return replaced
		}

		allErrors = multierr.Append(allErrors, errors.Errorf("invalid placeholder %q", string(b)))
		return b
	})

	return string(patchedStr), allErrors

}

func (v *PlaceholderReplacementVisitor) replacePackagePlaceholder(b []byte) ([]byte, error) {
	matches := packagePlaceholderRegexp.FindStringSubmatch(string(b))
	if matches == nil {
		return b, errors.Errorf("failed to find substring matches for placeholder %q", string(b))
	}
	packageType := matches[packagePlaceholderRegexp.SubexpIndex("type")]
	packageName := matches[packagePlaceholderRegexp.SubexpIndex("name")]

	packageConfig, isPresent := v.packages[packageName]
	if !isPresent {
		return b, errors.Errorf("failed to find a package named %q for placeholder %q",
			packageName, string(b))
	}
	if packageType != "" && packageType != string(packageConfig.Type) {
		return b,
			errors.Errorf("placeholder %q is of the wrong type. It should be a %q", string(b), string(packageConfig.Type))
	}
	return []byte(packageConfig.LocalDataDirectory()), nil

}
