package config_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/config/testutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

func TestNewWatcherNoop(t *testing.T) {
	logger := logging.NewTestLogger(t)
	watcher, err := config.NewWatcher(context.Background(), &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}

	test.That(t, watcher.Close(), test.ShouldBeNil)
}

func TestNewWatcherFile(t *testing.T) {
	logger := logging.NewTestLogger(t)

	temp, err := os.CreateTemp(t.TempDir(), "*.json")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	watcher, err := config.NewWatcher(context.Background(), &config.Config{ConfigFilePath: temp.Name()}, logger)
	test.That(t, err, test.ShouldBeNil)

	writeConf := func(conf *config.Config) {
		md, err := json.Marshal(&conf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(temp.Name(), md, 0o755), test.ShouldBeNil)
		for {
			rd, err := os.ReadFile(temp.Name())
			test.That(t, err, test.ShouldBeNil)
			if bytes.Equal(rd, md) {
				break
			}
			time.Sleep(time.Second)
		}
	}

	confToWrite := config.Config{
		ConfigFilePath: temp.Name(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "hello",
				Model: resource.DefaultModelFamily.WithModel("hello"),
				Attributes: rutils.AttributeMap{
					"world": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}
	writeConf(&confToWrite)
	test.That(t, confToWrite.Ensure(false, logger), test.ShouldBeNil)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	confToWrite = config.Config{
		ConfigFilePath: temp.Name(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "world",
				Model: resource.DefaultModelFamily.WithModel("world"),
				Attributes: rutils.AttributeMap{
					"hello": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "bar",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}
	writeConf(&confToWrite)
	test.That(t, confToWrite.Ensure(false, logger), test.ShouldBeNil)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	go func() {
		f, err := os.OpenFile(temp.Name(), os.O_RDWR|os.O_CREATE, 0o755)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, f.Close(), test.ShouldBeNil)
		}()
		_, err = f.Write([]byte("blahblah"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f.Sync(), test.ShouldBeNil)
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}

	confToWrite = config.Config{
		ConfigFilePath: temp.Name(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "woo",
				Model: resource.DefaultModelFamily.WithModel("woo"),
				Attributes: rutils.AttributeMap{
					"wah": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "wee",
				Name: "mah",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}
	writeConf(&confToWrite)
	test.That(t, confToWrite.Ensure(false, logger), test.ShouldBeNil)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	test.That(t, watcher.Close(), test.ShouldBeNil)
}

func TestNewWatcherCloud(t *testing.T) {
	logger := logging.NewTestLogger(t)

	certsToReturn := config.Cloud{
		TLSCertificate: "hello",
		TLSPrivateKey:  "world",
	}

	deviceID := primitive.NewObjectID().Hex()

	fakeServer, err := testutils.NewFakeCloudServer(context.Background(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, fakeServer.Shutdown(), test.ShouldBeNil)
	}()

	storeConfigInServer := func(cfg config.Config) {
		cloudConfProto, err := config.CloudConfigToProto(cfg.Cloud)
		test.That(t, err, test.ShouldBeNil)

		componentConfProto, err := config.ComponentConfigToProto(&cfg.Components[0])
		test.That(t, err, test.ShouldBeNil)

		proccessConfProto, err := config.ProcessConfigToProto(&cfg.Processes[0])
		test.That(t, err, test.ShouldBeNil)

		networkConfProto, err := config.NetworkConfigToProto(&cfg.Network)
		test.That(t, err, test.ShouldBeNil)

		protoConfig := &pb.RobotConfig{
			Cloud:      cloudConfProto,
			Components: []*pb.ComponentConfig{componentConfProto},
			Processes:  []*pb.ProcessConfig{proccessConfProto},
			Network:    networkConfProto,
		}

		fakeServer.Clear()
		fakeServer.StoreDeviceConfig(deviceID, protoConfig, &pb.CertificateResponse{
			TlsCertificate: certsToReturn.TLSCertificate,
			TlsPrivateKey:  certsToReturn.TLSPrivateKey,
		})
	}

	var confToReturn config.Config
	newCloudConf := func() *config.Cloud {
		return &config.Cloud{
			AppAddress:      fmt.Sprintf("http://%s", fakeServer.Addr().String()),
			ID:              deviceID,
			Secret:          testutils.FakeCredentialPayLoad,
			FQDN:            "woo",
			LocalFQDN:       "yee",
			RefreshInterval: time.Second,
			LocationSecrets: []config.LocationSecret{{ID: "1", Secret: "secret"}},
		}
	}

	confToReturn = config.Config{
		Cloud: newCloudConf(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "hello",
				Model: resource.DefaultModelFamily.WithModel("hello"),
				Attributes: rutils.AttributeMap{
					"world": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}

	storeConfigInServer(confToReturn)

	watcher, err := config.NewWatcher(context.Background(), &config.Config{Cloud: newCloudConf()}, logger)
	test.That(t, err, test.ShouldBeNil)

	confToExpect := confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey
	test.That(t, confToExpect.Ensure(true, logger), test.ShouldBeNil)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	confToReturn = config.Config{
		Cloud: newCloudConf(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "world",
				Model: resource.DefaultModelFamily.WithModel("world"),
				Attributes: rutils.AttributeMap{
					"hello": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "bar",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}

	// update the config with the newer config
	storeConfigInServer(confToReturn)

	confToExpect = confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey
	test.That(t, confToExpect.Ensure(true, logger), test.ShouldBeNil)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	// fake server will start returning 5xx on requests.
	// no new configs should be emitted to channel until the fake server starts returning again
	fakeServer.FailOnConfigAndCerts(true)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}
	fakeServer.FailOnConfigAndCerts(false)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	confToReturn = config.Config{
		Cloud: newCloudConf(),
		Components: []resource.Config{
			{
				API:   arm.API,
				Name:  "woo",
				Model: resource.DefaultModelFamily.WithModel("woo"),
				Attributes: rutils.AttributeMap{
					"wah": 1.0,
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "wee",
				Name: "mah",
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress: "localhost:8080",
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}

	storeConfigInServer(confToReturn)

	confToExpect = confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey
	test.That(t, confToExpect.Ensure(true, logger), test.ShouldBeNil)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	test.That(t, watcher.Close(), test.ShouldBeNil)
}
