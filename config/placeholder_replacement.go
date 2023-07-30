package config

import (
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/rdk/utils"
)

// Example strings that satisfy this regexp:
// ${packages.my-COOL-ml-model/__89}
// ${packages.modules.intel:CameraThatRocks}
var packagePlaceholderRegexp = regexp.MustCompile(`\$\{(?P<fulltriple>packages(\.(?P<type>ml_models|modules))?\.(?P<name>[\w:\/-]+))\}`)

func ContainsPlaceholder(s string) bool {
	return packagePlaceholderRegexp.Match([]byte(s))
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
			var zero T
			return zero, err
		}
		attributes = newAttrsTyped
	}
	return attributes, nil
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
	// TODO(pre-merge) if we wish to re-add Env var replacement, here would be a good place for that
	patchedStr := packagePlaceholderRegexp.ReplaceAllFunc([]byte(s), func(b []byte) []byte {
		matches := packagePlaceholderRegexp.FindStringSubmatch(string(b))
		if matches == nil {
			allErrors = multierr.Append(allErrors, errors.Errorf("failed to find substring matches for placeholder %q", string(b)))
			return b
		}
		// The only way that "name" or "type" is not present is if the regexp is out of sync with this function
		// if that occurs, then the tests will fail
		packageType := matches[packagePlaceholderRegexp.SubexpIndex("type")]
		packageName := matches[packagePlaceholderRegexp.SubexpIndex("name")]
		packageConfig, isPresent := v.packages[packageName]
		if !isPresent {
			allErrors = multierr.Append(allErrors, errors.Errorf("failed to find a package named %q for placeholder %q", packageName, string(b)))
			return b
		}
		// TODO(pre-merge) clean up this logic a bit if we decide to keep the module/modules difference
		if packageType != "" && packageType != string(packageConfig.Type)+"s" {
			allErrors = multierr.Append(allErrors,
				errors.Errorf("placeholder %q is of the wrong type. It should be %q", string(b), string(packageConfig.Type)+"s"))
			return b
		}
		return []byte(packageConfig.LocalDataDirectory())
	})
	return string(patchedStr), allErrors
}
