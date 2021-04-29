package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/utils"
)

func TestNewConfigWatcherNoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	watcher, err := NewConfigWatcher(&Config{}, logger)
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

func TestNewConfigWatcherFile(t *testing.T) {
	logger := golog.NewTestLogger(t)

	temp, err := ioutil.TempFile("", "*.json")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	watcher, err := NewConfigWatcher(&Config{ConfigFilePath: temp.Name()}, logger)
	test.That(t, err, test.ShouldBeNil)

	confToWrite := Config{
		Components: []ComponentConfig{
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
	go func() {
		md, err := json.Marshal(&confToWrite)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ioutil.WriteFile(temp.Name(), md, 0755), test.ShouldBeNil)
	}()

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	confToWrite = Config{
		Components: []ComponentConfig{
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
	go func() {
		md, err := json.Marshal(&confToWrite)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ioutil.WriteFile(temp.Name(), md, 0755), test.ShouldBeNil)
	}()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	go func() {
		test.That(t, ioutil.WriteFile(temp.Name(), []byte("blahblah"), 0755), test.ShouldBeNil)
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case c := <-watcher.Config():
		test.That(t, c, test.ShouldBeNil)
	case <-timer.C:
	}

	confToWrite = Config{
		Components: []ComponentConfig{
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
	go func() {
		md, err := json.Marshal(&confToWrite)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ioutil.WriteFile(temp.Name(), md, 0755), test.ShouldBeNil)
	}()

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToWrite)

	test.That(t, watcher.Close(), test.ShouldBeNil)
}

func TestNewConfigWatcherCloud(t *testing.T) {
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
		if confErr {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		confErr = true
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

	cloudConf := &CloudConfig{
		Path:   fmt.Sprintf("http://%s", listener.Addr().String()),
		ID:     "my_id",
		Secret: "my_secret",
	}
	confToReturn = Config{
		Cloud: cloudConf,
		Components: []ComponentConfig{
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

	watcher, err := NewConfigWatcher(&Config{Cloud: cloudConf}, logger)
	test.That(t, err, test.ShouldBeNil)

	newConf := <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToReturn)

	confToReturn = Config{
		Cloud: cloudConf,
		Components: []ComponentConfig{
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
	confErr = false

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
		Components: []ComponentConfig{
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
	confErr = false

	newConf = <-watcher.Config()
	test.That(t, newConf, test.ShouldResemble, &confToReturn)

	test.That(t, watcher.Close(), test.ShouldBeNil)
	test.That(t, httpServer.Shutdown(context.Background()), test.ShouldBeNil)
	<-serveDone
}
