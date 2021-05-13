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

	"go.viam.com/core/rexec"
	"go.viam.com/core/utils"
)

func TestNewWatcherNoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	watcher, err := NewWatcher(&Config{}, logger)
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
	logger := golog.NewTestLogger(t)

	temp, err := ioutil.TempFile("", "*.json")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	watcher, err := NewWatcher(&Config{ConfigFilePath: temp.Name()}, logger)
	test.That(t, err, test.ShouldBeNil)

	writeConf := func(conf *Config) {
		md, err := json.Marshal(&conf)
		test.That(t, err, test.ShouldBeNil)
		f, err := os.OpenFile(temp.Name(), os.O_RDWR|os.O_CREATE, 0755)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, f.Close(), test.ShouldBeNil)
		}()
		_, err = f.Write(md)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f.Sync(), test.ShouldBeNil)
	}

	confToWrite := Config{
		Components: []Component{
			{
				Name: "hello",
				Attributes: AttributeMap{
					"world": 1.0,
				},
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo",
			},
		},
	}
	go writeConf(&confToWrite)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	confToWrite = Config{
		Components: []Component{
			{
				Name: "world",
				Attributes: AttributeMap{
					"hello": 1.0,
				},
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "2",
				Name: "bar",
			},
		},
	}
	go writeConf(&confToWrite)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	go func() {
		f, err := os.OpenFile(temp.Name(), os.O_RDWR|os.O_CREATE, 0755)
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
		Components: []Component{
			{
				Name: "woo",
				Attributes: AttributeMap{
					"wah": 1.0,
				},
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "wee",
				Name: "mah",
			},
		},
	}
	go writeConf(&confToWrite)

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	test.That(t, watcher.Close(), test.ShouldBeNil)
}

func TestNewWatcherCloud(t *testing.T) {
	logger := golog.NewTestLogger(t)

	randomPort, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", randomPort))
	test.That(t, err, test.ShouldBeNil)
	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	var confToReturn Config
	var confErr bool
	var confErrMu sync.Mutex
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
		confErrMu.Lock()
		if confErr {
			confErrMu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		confErr = true
		confErrMu.Unlock()
		md, err := json.Marshal(&confToReturn)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("error marshaling status: %s", err)))
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
		Processes: []rexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo",
			},
		},
	}

	watcher, err := NewWatcher(&Config{Cloud: cloudConf}, logger)
	test.That(t, err, test.ShouldBeNil)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToReturn)

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
		Processes: []rexec.ProcessConfig{
			{
				ID:   "2",
				Name: "bar",
			},
		},
	}
	confErrMu.Lock()
	confErr = false
	confErrMu.Unlock()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToReturn)

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
		Processes: []rexec.ProcessConfig{
			{
				ID:   "wee",
				Name: "mah",
			},
		},
	}
	confErrMu.Lock()
	confErr = false
	confErrMu.Unlock()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToReturn)

	test.That(t, watcher.Close(), test.ShouldBeNil)
	test.That(t, httpServer.Shutdown(context.Background()), test.ShouldBeNil)
	<-serveDone
}
