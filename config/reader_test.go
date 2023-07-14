package config

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

func TestStoreToCache(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger)

	test.That(t, err, test.ShouldBeNil)

	cloud := &Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
		AppAddress:       "https://app.viam.dev:443",
	}
	cfg.Cloud = cloud

	// store our config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)

	// store the updated config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg3, test.ShouldResemble, cfg)
}

func TestCacheInvalidation(t *testing.T) {
	id := uuid.New().String()
	// store invalid config in cache
	cachePath := getCloudCacheFilePath(id)
	err := os.WriteFile(cachePath, []byte("invalid-json"), 0o750)
	test.That(t, err, test.ShouldBeNil)

	// read from cache, should return parse error and remove file
	_, err = GenerateConfigFromFile(id)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot parse the cached config as json")

	// read from cache again and file should not exist
	_, err = GenerateConfigFromFile(id)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
}

func TestShouldCheckForCert(t *testing.T) {
	cloud1 := Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
		LocationSecrets: []LocationSecret{
			{ID: "id1", Secret: "secret1"},
			{ID: "id2", Secret: "secret2"},
		},
	}
	cloud2 := cloud1
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeFalse)

	cloud2.TLSCertificate = "abc"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeFalse)

	cloud2 = cloud1
	cloud2.LocationSecret = "something else"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)

	cloud2 = cloud1
	cloud2.LocationSecrets = []LocationSecret{
		{ID: "id1", Secret: "secret1"},
		{ID: "id2", Secret: "secret3"},
	}
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)
}

func TestProcessConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	unprocessedConfig := Config{
		ConfigFilePath: "path",
	}

	cfg, err := processConfig(&unprocessedConfig, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *cfg, test.ShouldResemble, unprocessedConfig)
}

func TestPlaceholderReplacement(t *testing.T) {
	t.Run("Generating package placeholder", func(t *testing.T) {
		config := PackageConfig{
			Name: "name",
		}
		test.That(t, getPackagePlaceholder(config), test.ShouldEqual, "packages.name")

		config.Type = PackageTypeMlModel
		test.That(t, getPackagePlaceholder(config), test.ShouldEqual, "packages.ml_models.name")

		config.Type = PackageTypeModule
		test.That(t, getPackagePlaceholder(config), test.ShouldEqual, "packages.modules.name")
	})

	t.Run("MatchingPackageRegex", func(t *testing.T) {
		placeholder := "\n\n ${packages.ml_model.my_model} \n my name is"
		strings := placeholderRegexp.FindStringSubmatch(placeholder)
		test.That(t, len(strings), test.ShouldEqual, 2)
		test.That(t, strings[1], test.ShouldEqual, "packages.ml_model.my_model")

		placeholder = "\n\n ${packages.modules.my_module} bleh bleh"
		strings = placeholderRegexp.FindStringSubmatch(placeholder)
		test.That(t, len(strings), test.ShouldEqual, 2)
		test.That(t, strings[1], test.ShouldEqual, "packages.modules.my_module")

		placeholder = "\n\n ${packages.my_ml_model}/testing bleh bleh"
		strings = placeholderRegexp.FindStringSubmatch(placeholder)
		test.That(t, len(strings), test.ShouldEqual, 2)
		test.That(t, strings[1], test.ShouldEqual, "packages.my_ml_model")

		// invalid ones

		// no starting with packages should not match
		placeholder = "\n\n ${HOME.viam} my random text"
		strings = placeholderRegexp.FindStringSubmatch(placeholder)
		test.That(t, len(strings), test.ShouldEqual, 0)

		// one with a random type placeholder
		placeholder = "\n\n ${packages.random.random-name} bleh bleh"
		strings = placeholderRegexp.FindStringSubmatch(placeholder)
		test.That(t, len(strings), test.ShouldEqual, 0)
	})

	t.Run("Generate Expected Filepath", func(t *testing.T) {
		config := PackageConfig{
			Name:    "name",
			Package: "org/name",
			Version: "latest",
		}

		// can't test the full path because it depends on the root of OS
		// not sure how that works in CI

		// for backwards compatibility will leave this for now
		expectedPath := path.Join(viamDotDir, "packages", "name")
		test.That(t, generateFilePath(config), test.ShouldEqual, expectedPath)

		config.Type = PackageTypeMlModel
		expectedPath = path.Join(viamDotDir, "packages", "ml_models", ".data", "org-name-latest")
		test.That(t, generateFilePath(config), test.ShouldEqual, expectedPath)

		config.Type = PackageTypeModule
		expectedPath = path.Join(viamDotDir, "packages", "modules", ".data", "org-name-latest")
		test.That(t, generateFilePath(config), test.ShouldEqual, expectedPath)
	})

	t.Run("Get Packages from file", func(t *testing.T) {
		fakeConfig := string(`{
			"services": [
				  {
					"type": "mlmodel",
					"model": "tflite_cpu",
					"attributes": {
					  "label_path": "${packages.ml-test}/effdetlabels.txt",
					  "num_threads": 1,
					  "model_path": "${packages.ml-test}/effdet0.tflite"
					},
					"name": "face-detection"
				  }
			],
			"packages": [
				  {
					"name": "ml-test",
					"version": "latest",
					"package": "org/ml-test"
				  }
			],
			"components": []
		}`)

		file := []byte(fakeConfig)

		packages, err := getPackagesFromFile(file)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, packages, test.ShouldHaveLength, 1)
		p := packages[0]

		package1 := PackageConfig{
			Name:    "ml-test",
			Package: "org/ml-test",
			Version: "latest",
		}

		test.That(t, p, test.ShouldResemble, package1)

		fakeConfig = string(`{
			"services": [
				  {
					"type": "mlmodel",
					"model": "tflite_cpu",
					"attributes": {
					  "label_path": "${packages.ml-test}/effdetlabels.txt",
					  "num_threads": 1,
					  "model_path": "${packages.ml-test}/effdet0.tflite"
					},
					"name": "face-detection"
				  }
			],
			"packages": [
				  {
					"name": "ml-test",
					"version": "latest",
					"package": "org/ml-test", 
					"type": "ml_model"
				  }, 
				  {
					"name": "great-module",
					"version": "latest",
					"package": "org/great-module",
					"type": "module"

				  }
			],
			"components": []
		}`)

		package1.Type = PackageTypeMlModel
		file = []byte(fakeConfig)

		packages, err = getPackagesFromFile(file)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, packages, test.ShouldHaveLength, 2)

		package2 := packages[1]
		test.That(t, packages[0], test.ShouldResemble, package1)
		test.That(t, packages[1], test.ShouldResemble, package2)

		// test with no packages

		fakeConfig = string(`{
			"services": [
				  {
					"type": "mlmodel",
					"model": "tflite_cpu",
					"attributes": {
					  "label_path": "${packages.ml-test}/effdetlabels.txt",
					  "num_threads": 1,
					  "model_path": "${packages.ml-test}/effdet0.tflite"
					},
					"name": "face-detection"
				  }
			],
			"components": []
		}`)

		file = []byte(fakeConfig)

		packages, err = getPackagesFromFile(file)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, packages, test.ShouldHaveLength, 0)
	})

	t.Run("Map placeholders to paths", func(t *testing.T) {
		packages := []PackageConfig{
			{
				Name:    "ml-test",
				Type:    PackageTypeMlModel,
				Package: "org/ml-test",
				Version: "1",
			},
			{
				Name:    "great-module",
				Type:    PackageTypeModule,
				Package: "org/great-module",
				Version: "2",
			},
			{
				Name:    "old-package",
				Package: "org/old-package",
				Version: "3",
			},
		}

		actualMap := mapPlaceholderToRealPaths(packages)
		expectedMap := make(map[string]string, 3)
		expectedMap["packages.old_package"] = path.Clean(
			path.Join(viamDotDir, "packages", "old_package"))
		expectedMap["packages.ml_models.ml_test"] = path.Clean(
			path.Join(viamDotDir, "packages", "ml_models", ".data", "org-ml-test-1"))
		expectedMap["packages.modules.great_module"] = path.Clean(
			path.Join(viamDotDir, "packages", "modules", ".data", "org-modules-great-module-2"))

		test.That(t, actualMap, test.ShouldResemble, actualMap)
	})

	t.Run("ReplaceFilePlaceholders", func(t *testing.T) {
		filepath := "./data/robot.json"
		config, err := GenerateConfigFromFile(filepath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, config.Modules, test.ShouldHaveLength, 1)
		test.That(t, config.Packages, test.ShouldHaveLength, 3)
		test.That(t, config.Services, test.ShouldHaveLength, 2)

		module := config.Modules[0]
		moduleExecPath := module.ExePath
		expectedExecPAth := path.Clean(path.Join(viamDotDir, "packages", "modules", ".data", "org-my-module-latest", "exec.sh"))
		test.That(t, moduleExecPath, test.ShouldEqual, expectedExecPAth)

		service := config.Services[0]
		test.That(t, service.Attributes.Has("label_path"), test.ShouldBeTrue)
		labelPath := service.Attributes.String("label_path")
		test.That(t, labelPath, test.ShouldEqual,
			path.Clean(path.Join(viamDotDir, "packages", "ml_models", ".data", "org-my-ml-package-3", "effdetlabels.txt")))

		test.That(t, service.Attributes.Has("model_path"), test.ShouldBeTrue)
		modelPath := service.Attributes.String("model_path")
		test.That(t, modelPath, test.ShouldEqual,
			path.Clean(path.Join(viamDotDir, "packages", "ml_models", ".data", "org-my-ml-package-3", "effdet0.tflite")))

		service = config.Services[1]
		test.That(t, service.Attributes.Has("label_path"), test.ShouldBeTrue)
		test.That(t, service.Attributes.Has("model_path"), test.ShouldBeTrue)

		labelPath = service.Attributes.String("label_path")
		modelPath = service.Attributes.String("model_path")

		test.That(t, labelPath, test.ShouldEqual, path.Clean(path.Join(viamDotDir, "packages", "cool-package", "coollabels.txt")))
		test.That(t, modelPath, test.ShouldEqual, path.Clean(path.Join(viamDotDir, "packages", "cool-package", "cool.tflite")))
	})

	t.Run("replace placeholders in a config", func(t *testing.T) {
		attributes1 := make(rutils.AttributeMap)
		attributes1["label_path"] = "${packages.old-package}/effdetlabels.txt"
		attributes1["model_path"] = "${packages.old-package}/effdet0.tflite"
		attributes1["num_threads"] = 1

		attributes2 := make(rutils.AttributeMap)
		attributes2["label_path"] = "${packages.ml_models.ml-test}/effdetlabels.txt"
		attributes2["model_path"] = "${packages.ml_models.ml-test}/effdet0.tflite"
		attributes2["num_threads"] = 1

		conf := &Config{
			Packages: []PackageConfig{
				{
					Name:    "ml-test",
					Type:    PackageTypeMlModel,
					Package: "org/ml-test",
					Version: "1",
				},
				{
					Name:    "my-great-module",
					Type:    PackageTypeModule,
					Package: "org/my-great-module",
					Version: "2",
				},
				{
					Name:    "old-package",
					Package: "org/old-package",
					Version: "3",
				},
			},
			Services: []resource.Config{
				{
					Name:       "fake1",
					API:        resource.NewAPI("rdk", "new", "myapi"),
					Model:      resource.DefaultModelFamily.WithModel("tflite_cpu"),
					Attributes: attributes1,
				},
				{
					Name:       "fake1",
					API:        resource.NewAPI("rdk", "new", "myapi"),
					Model:      resource.DefaultModelFamily.WithModel("tflite_cpu"),
					Attributes: attributes2,
				},
			},
			Modules: []Module{
				{
					Name:    "my-great-module",
					ExePath: "${packages.modules.my-great-module}/exec.sh",
				},
			},
		}

		viamDotDir := filepath.Join(os.Getenv("HOME"), ".viam")

		err := replacePlaceholdersInCloudConfig(conf)
		test.That(t, err, test.ShouldBeNil)

		module := conf.Modules[0]
		moduleExecPath := module.ExePath
		expectedExecPAth := path.Clean(path.Join(viamDotDir, "packages", "modules", ".data", "org-my-great-module-2", "exec.sh"))
		test.That(t, moduleExecPath, test.ShouldEqual, expectedExecPAth)

		service := conf.Services[0]
		test.That(t, service.Attributes.Has("label_path"), test.ShouldBeTrue)
		test.That(t, service.Attributes.Has("model_path"), test.ShouldBeTrue)

		labelPath := service.Attributes.String("label_path")
		modelPath := service.Attributes.String("model_path")

		test.That(t, labelPath, test.ShouldEqual, path.Clean(path.Join(viamDotDir, "packages", "old-package", "effdetlabels.txt")))
		test.That(t, modelPath, test.ShouldEqual, path.Clean(path.Join(viamDotDir, "packages", "old-package", "effdet0.tflite")))

		service = conf.Services[1]
		test.That(t, service.Attributes.Has("label_path"), test.ShouldBeTrue)
		labelPath = service.Attributes.String("label_path")
		test.That(t, labelPath, test.ShouldEqual,
			path.Clean(path.Join(viamDotDir, "packages", "ml_models", ".data", "org-ml-test-1", "effdetlabels.txt")))

		test.That(t, service.Attributes.Has("model_path"), test.ShouldBeTrue)
		modelPath = service.Attributes.String("model_path")
		test.That(t, modelPath, test.ShouldEqual,
			path.Clean(path.Join(viamDotDir, "packages", "ml_models", ".data", "org-ml-test-1", "effdet0.tflite")))
	})
}
