package config

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/utils"
)

// This placeholder regex matches on all strings that satisfy our criteria for a placeholder
// Example string satisfying the regex:
// ${hello}.
var placeholderRegexp = regexp.MustCompile(`\$\{(?P<placeholder_key>[^\}]*)\}`)

// packagePlaceholderRegexp matches on all valid ways of specifying one of our package placeholders
// Example strings satisfying the regex:
// packages.my-COOL-ml-model/__89
// packages.modules.intel:CameraThatRocks
// packages.FutureP4ckge_Ty-pe.name.
var packagePlaceholderRegexp = regexp.MustCompile(`^packages(\.(?P<type>[^\.]+))?\.(?P<name>[\w:/-]+)$`)

// ContainsPlaceholder returns true if the passed string contains a placeholder.
func ContainsPlaceholder(s string) bool {
	return placeholderRegexp.MatchString(s)
}

// ReplacePlaceholders traverses parts of the config to replace placeholders with their resolved values.
func (c *Config) ReplacePlaceholders() error {
	var allErrs, err error
	visitor := NewPlaceholderReplacementVisitor(c)

	for i, service := range c.Services {
		// this nil check may seem superfluous, however, the walking & casting will transform a
		// utils.AttributeMap(nil) into a utils.AttributeMap{} which causes config diffs
		if service.Attributes == nil {
			continue
		}
		c.Services[i].Attributes, err = walkTypedAttributes(visitor, service.Attributes)
		allErrs = multierr.Append(allErrs, err)
	}

	for i, component := range c.Components {
		if component.Attributes == nil {
			continue
		}
		c.Components[i].Attributes, err = walkTypedAttributes(visitor, component.Attributes)
		allErrs = multierr.Append(allErrs, err)
	}

	for i, module := range c.Modules {
		c.Modules[i].ExePath, err = visitor.replacePlaceholders(module.ExePath)
		allErrs = multierr.Append(allErrs, err)
	}

	return multierr.Append(visitor.AllErrors, allErrs)
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
	return attributes, errors.New("placeholder replacement tried to walk an unwalkable type")
}

// PlaceholderReplacementVisitor is a visitor that replaces strings containing placeholder values with their desired values.
type PlaceholderReplacementVisitor struct {
	// Map of packageName -> packageConfig
	packages map[string]PackageConfig
	// Accumulation of all that occurred during traversal
	AllErrors error
}

// NewPlaceholderReplacementVisitor creates a new PlaceholderReplacementVisitor.
func NewPlaceholderReplacementVisitor(cfg *Config) *PlaceholderReplacementVisitor {
	// Create the map of packages that will be used for replacement
	packages := map[string]PackageConfig{}
	for _, config := range cfg.Packages {
		packages[config.Name] = config
	}

	return &PlaceholderReplacementVisitor{
		packages:  packages,
		AllErrors: nil,
	}
}

// Visit implements config.Visitor.
// Importantly, this function will never error. Instead, all errors are accumulated on the PlaceholderReplacementVisitor.AllErrors object.
//
// Returning an error causes the walker to prematurely stop traversing the tree. This is undesirable because it means a single invalid
// placeholder causes otherwise valid placeholders will appear invalid & unreplaced to the user (there is also no guaranteed order an
// attribute map is traversed, so if there is a single invalid placeholder, the set of other placeholders that fail to be resolved would
// be non-deterministic).
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
	v.AllErrors = multierr.Append(v.AllErrors, err)

	// If the input was a pointer, return a pointer.
	if t.Kind() == reflect.Ptr {
		return &withReplacedRefs, nil
	}
	return withReplacedRefs, nil
}

// replacePlaceholders tries to replace all placeholders in a given string using a two step process:
// First, match anything that could be a placeholder (ex: ${hello})
// Second, attempt to match any of those placeholder keys (ex: "hello") with any rules we have for transformation
//
// This is done so that misspellings like ${package.module.name} wont be silently ignored and
// so that it is easy to add additional placeholder types in the future (like environment variables).
func (v *PlaceholderReplacementVisitor) replacePlaceholders(s string) (string, error) {
	var allErrors error
	// First, match all possible placeholders (ex: ${hello})
	patchedStr := placeholderRegexp.ReplaceAllFunc([]byte(s), func(placeholder []byte) []byte {
		matches := placeholderRegexp.FindSubmatch(placeholder)
		if matches == nil {
			allErrors = multierr.Append(allErrors, errors.Errorf("failed to find substring matches for placeholder %q", string(placeholder)))
			return placeholder
		}
		placeholderKey := matches[placeholderRegexp.SubexpIndex("placeholder_key")]

		// Now, match against every way we know of doing placeholder replacement
		if packagePlaceholderRegexp.Match(placeholderKey) {
			replaced, err := v.replacePackagePlaceholder(string(placeholderKey))
			if err != nil {
				allErrors = multierr.Append(allErrors, err)
				return placeholder
			}
			return []byte(replaced)
		}

		allErrors = multierr.Append(allErrors, errors.Errorf("invalid placeholder %q", string(placeholder)))
		return placeholder
	})

	return string(patchedStr), allErrors
}

func (v *PlaceholderReplacementVisitor) replacePackagePlaceholder(toReplace string) (string, error) {
	matches := packagePlaceholderRegexp.FindStringSubmatch(toReplace)
	if matches == nil {
		return toReplace, errors.Errorf("failed to find substring matches for placeholder %q", toReplace)
	}
	packageType := matches[packagePlaceholderRegexp.SubexpIndex("type")]
	packageName := matches[packagePlaceholderRegexp.SubexpIndex("name")]

	if packageType == "" {
		// for backwards compatibility
		packageType = string(PackageTypeMlModel)
	}
	packageConfig, isPresent := v.packages[packageName]
	if !isPresent {
		return toReplace, errors.Errorf("failed to find a package named %q for placeholder %q",
			packageName, toReplace)
	}
	if packageType != string(packageConfig.Type) {
		expectedPlaceholder := fmt.Sprintf("packages.%s.%s", string(packageConfig.Type), packageName)
		return toReplace,
			errors.Errorf("placeholder %q is looking for a package of type %q but a package of type %q was found. Try %q",
				toReplace, packageType, string(packageConfig.Type), expectedPlaceholder)
	}
	return packageConfig.LocalDataDirectory(viamPackagesDir), nil
}
