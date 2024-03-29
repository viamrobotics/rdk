package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestPlaceholderReplacement(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	viamPackagesDir := filepath.Join(homeDir, ".viam", "packages")
	t.Run("package placeholder replacement", func(t *testing.T) {
		cfg := &Config{
			Components: []resource.Config{
				{
					Name: "m",
					Attributes: utils.AttributeMap{
						"should_equal_1":     "${packages.coolpkg}",
						"should_equal_2":     "${packages.ml_model.coolpkg}",
						"mid_string_replace": "Hello ${packages.coolpkg} Friends!",
						"module_replace":     "${packages.module.coolmod}",
						"multi_replace":      "${packages.coolpkg} ${packages.module.coolmod}",
					},
				},
			},
			Services: []resource.Config{
				{
					Name: "m",
					Attributes: utils.AttributeMap{
						"apply_to_services_too": "${packages.coolpkg}",
					},
				},
			},
			Modules: []Module{
				{
					ExePath: "${packages.module.coolmod}/bin",
				},
			},
			Packages: []PackageConfig{
				{
					Name:    "coolpkg",
					Package: "orgid/pkg",
					Type:    "ml_model",
					Version: "0.4.0",
				},
				{
					Name:    "coolmod",
					Package: "orgid/mod",
					Type:    "module",
					Version: "0.5.0",
				},
			},
		}
		err := cfg.ReplacePlaceholders()
		test.That(t, err, test.ShouldBeNil)
		dirForMlModel := cfg.Packages[0].LocalDataDirectory(viamPackagesDir)
		dirForModule := cfg.Packages[1].LocalDataDirectory(viamPackagesDir)
		// components
		attrMap := cfg.Components[0].Attributes
		test.That(t, attrMap["should_equal_1"], test.ShouldResemble, attrMap["should_equal_2"])
		test.That(t, attrMap["should_equal_1"], test.ShouldResemble, dirForMlModel)
		test.That(t, attrMap["mid_string_replace"], test.ShouldResemble, fmt.Sprintf("Hello %s Friends!", dirForMlModel))
		test.That(t, attrMap["module_replace"], test.ShouldResemble, dirForModule)
		test.That(t, attrMap["multi_replace"], test.ShouldResemble, fmt.Sprintf("%s %s", dirForMlModel, dirForModule))
		// services
		attrMap = cfg.Services[0].Attributes
		test.That(t, attrMap["apply_to_services_too"], test.ShouldResemble, dirForMlModel)
		// module
		exePath := cfg.Modules[0].ExePath
		test.That(t, exePath, test.ShouldResemble, fmt.Sprintf("%s/bin", dirForModule))
	})
	t.Run("package placeholder typos", func(t *testing.T) {
		// Unknown type of placeholder
		cfg := &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${invalidplaceholder}",
					},
				},
			},
		}
		err := cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "invalidplaceholder")
		// Test that the attribute is left unchanged if replacement failed
		test.That(t, cfg.Components[0].Attributes["a"], test.ShouldResemble, "${invalidplaceholder}")

		// Empy placeholder
		cfg = &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${}",
					},
				},
			},
		}
		err = cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "invalid placeholder")
		// Test that the attribute is left unchanged if replacement failed
		test.That(t, cfg.Components[0].Attributes["a"], test.ShouldResemble, "${}")

		// Package placeholder with no equivalent pkg
		cfg = &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${packages.ml_model.chicken}",
					},
				},
			},
		}
		err = cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "package named \"chicken\"")
		// Test that the attribute is left unchanged if replacement failed
		test.That(t, cfg.Components[0].Attributes["a"], test.ShouldResemble, "${packages.ml_model.chicken}")

		// Package placeholder with wrong type
		cfg = &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${packages.ml_model.chicken}",
					},
				},
			},
			Packages: []PackageConfig{
				{
					Name:    "chicken",
					Package: "orgid/pkg",
					Type:    "module",
					Version: "0.4.0",
				},
			},
		}
		err = cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "looking for a package of type \"ml_model\"")

		// Half successful string replacement
		cfg = &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${packages.module.chicken}/${invalidplaceholder}",
					},
				},
			},
			Packages: []PackageConfig{
				{
					Name:    "chicken",
					Package: "orgid/pkg",
					Type:    "module",
					Version: "0.4.0",
				},
			},
		}
		err = cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "invalidplaceholder")
		test.That(t, fmt.Sprint(err), test.ShouldNotContainSubstring, "chicken")

		test.That(t, cfg.Components[0].Attributes["a"], test.ShouldResemble,
			fmt.Sprintf("%s/${invalidplaceholder}", cfg.Packages[0].LocalDataDirectory(viamPackagesDir)))
	})
	t.Run("environment variable placeholder replacement", func(t *testing.T) {
		// test success
		cfg := &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${environment.HOME}",
					},
				},
			},
			Modules: []Module{
				{
					Environment: map[string]string{
						"PATH": "${environment.PATH}",
					},
				},
			},
		}
		err := cfg.ReplacePlaceholders()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfg.Components[0].Attributes["a"], test.ShouldEqual, os.Getenv("HOME"))
		test.That(t, cfg.Modules[0].Environment["PATH"], test.ShouldEqual, os.Getenv("PATH"))

		// test failure
		cfg = &Config{
			Components: []resource.Config{
				{
					Attributes: utils.AttributeMap{
						"a": "${environment.VIAM_UNDEFINED_TEST_VAR}",
					},
				},
			},
		}
		err = cfg.ReplacePlaceholders()
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "VIAM_UNDEFINED_TEST_VAR")
	})
	t.Run("packageNames", func(t *testing.T) {
		test.That(t, packageNames("${packages.module.first}/hello", PackageTypeModule), test.ShouldResemble, []string{"first"})
		test.That(t, packageNames("hello", PackageTypeModule), test.ShouldResemble, []string{})
		test.That(t, packageNames("", PackageTypeModule), test.ShouldResemble, []string{})
		test.That(t, packageNames("${packages.module.first}/hello", PackageTypeMlModel), test.ShouldResemble, []string{})
		test.That(t, packageNames("${packages.ml_model.first}/hello", PackageTypeMlModel), test.ShouldResemble, []string{"first"})
		test.That(t, packageNames("${packages.first}/hello", PackageTypeMlModel), test.ShouldResemble, []string{"first"})
	})
	t.Run("ModulesForPackage", func(t *testing.T) {
		module := Module{ExePath: "${packages.module.first}/module"}
		modules := ModulesForPackage("first", []Module{
			module,
			{ExePath: "${packages.module.second}/mainbinary"},
			{ExePath: "/abs/path/mainbinary"},
		})
		test.That(t, modules, test.ShouldResemble, []Module{module})
	})
}
