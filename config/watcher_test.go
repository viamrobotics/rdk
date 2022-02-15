package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
)

func TestNewWatcherNoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	watcher, err := NewWatcher(context.Background(), &Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}

	test.That(t, utils.TryClose(context.Background(), watcher), test.ShouldBeNil)
}

func TestNewWatcherFile(t *testing.T) {
	logger := golog.NewTestLogger(t)

	temp, err := ioutil.TempFile("", "*.json")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	watcher, err := NewWatcher(context.Background(), &Config{ConfigFilePath: temp.Name()}, logger)
	test.That(t, err, test.ShouldBeNil)

	var writeWait sync.WaitGroup
	writeConf := func(conf *Config) {
		defer writeWait.Done()
		md, err := json.Marshal(&conf)
		test.That(t, err, test.ShouldBeNil)
		f, err := os.OpenFile(temp.Name(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, f.Close(), test.ShouldBeNil)
		}()
		_, err = f.Write(md)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f.Sync(), test.ShouldBeNil)
		time.Sleep(2 * time.Second) // wait after sync
	}

	confToWrite := Config{
		ConfigFilePath: temp.Name(),
		Components: []Component{
			{
				Name: "hello",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}
	writeWait.Add(1)
	go writeConf(&confToWrite)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	confToWrite = Config{
		ConfigFilePath: temp.Name(),
		Components: []Component{
			{
				Name: "world",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}
	writeWait.Add(1)
	go writeConf(&confToWrite)

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

	confToWrite = Config{
		ConfigFilePath: temp.Name(),
		Components: []Component{
			{
				Name: "woo",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}
	writeWait.Add(1)
	go writeConf(&confToWrite)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	test.That(t, utils.TryClose(context.Background(), watcher), test.ShouldBeNil)
	writeWait.Wait()
}

func TestNewWatcherCloud(t *testing.T) {
	// TODO(https://github.com/viamrobotics/rdk/issues/523): test doesn't work
	t.Skip("Skipping test for now")
	logger := golog.NewTestLogger(t)

	randomPort, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", randomPort))
	test.That(t, err, test.ShouldBeNil)
	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	certsToReturn := Cloud{
		TLSCertificate: "hello",
		TLSPrivateKey:  "world",
	}

	var confToReturn Config
	var confErr bool
	var confMu sync.Mutex
	var certsOnce bool
	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			panic(err)
		}
		if len(r.Form["id"]) == 0 || r.Form["id"][0] != "my_id" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad id"))
			return
		}
		if r.Header.Get("secret") != "my_secret" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("bad secret"))
			return
		}

		if len(r.Form["cert"]) != 0 && !certsOnce {
			certsOnce = true
			md, err := json.Marshal(&certsToReturn)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("error marshaling certs: %s", err)))
				return
			}
			w.Write(md)
			return
		}

		confMu.Lock()
		if confErr {
			confMu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		confErr = true

		md, err := json.Marshal(&confToReturn)
		confMu.Unlock()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("error marshaling conf: %s", err)))
			return
		}
		w.Write(md)
	})
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		httpServer.Serve(listener)
	}()

	cloudConf := &Cloud{
		Path:            fmt.Sprintf("http://%s", listener.Addr().String()),
		ID:              "my_id",
		Secret:          "my_secret",
		FQDN:            "woo",
		LocalFQDN:       "yee",
		RefreshInterval: time.Second,
	}
	confToReturn = Config{
		Cloud: cloudConf,
		Components: []Component{
			{
				Name: "hello",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}

	confToExpect := confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey

	watcher, err := NewWatcher(context.Background(), &Config{Cloud: cloudConf}, logger)
	test.That(t, err, test.ShouldBeNil)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	confToReturn = Config{
		Cloud: cloudConf,
		Components: []Component{
			{
				Name: "world",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}
	confMu.Lock()
	confErr = false

	confToExpect = confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey
	confMu.Unlock()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}

	confToReturn = Config{
		Cloud: cloudConf,
		Components: []Component{
			{
				Name: "woo",
				Attributes: AttributeMap{
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
		Network: NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "localhost:8080"}},
	}
	confMu.Lock()
	confErr = false

	confToExpect = confToReturn
	confToExpect.Cloud.TLSCertificate = certsToReturn.TLSCertificate
	confToExpect.Cloud.TLSPrivateKey = certsToReturn.TLSPrivateKey
	confMu.Unlock()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToExpect)

	test.That(t, utils.TryClose(context.Background(), watcher), test.ShouldBeNil)
	test.That(t, httpServer.Shutdown(context.Background()), test.ShouldBeNil)
	<-serveDone
}
