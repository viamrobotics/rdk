package rexec

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"github.com/fsnotify/fsnotify"
)

func TestProcessManagerStart(t *testing.T) {
	t.Run("an empty manager can start", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		defer func() {
			test.That(t, pm.Stop(), test.ShouldBeNil)
		}()
		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)
		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

		t.Run("adding a process after starting starts it", func(t *testing.T) {
			temp, err := ioutil.TempFile("", "*.txt")
			test.That(t, err, test.ShouldBeNil)
			defer os.Remove(temp.Name())

			test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true}), test.ShouldBeNil)

			rd, err := ioutil.ReadFile(temp.Name())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, string(rd), test.ShouldEqual, "hello\n")

			// starting again should do nothing
			test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

			// a canceled ctx should fail immediately for one shot only
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err = pm.AddProcessFromConfig(ctx, ProcessConfig{Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
			err = pm.AddProcessFromConfig(ctx, ProcessConfig{Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}})
			test.That(t, err, test.ShouldBeNil)

			// a "timed" ctx should only have an effect on one shots
			ctx, cancel = context.WithCancel(context.Background())

			temp1, err := ioutil.TempFile("", "*.txt")
			test.That(t, err, test.ShouldBeNil)
			defer os.Remove(temp1.Name())
			temp2, err := ioutil.TempFile("", "*.txt")
			test.That(t, err, test.ShouldBeNil)
			defer os.Remove(temp2.Name())

			watcher, err := fsnotify.NewWatcher()
			test.That(t, err, test.ShouldBeNil)
			defer watcher.Close()
			watcher.Add(temp1.Name())
			watcher.Add(temp2.Name())
			go func() {
				<-watcher.Events
				<-watcher.Events
				cancel()
			}()

			test.That(t, pm.AddProcessFromConfig(ctx, ProcessConfig{Name: "bash", Args: []string{
				"-c", fmt.Sprintf("echo one >> %s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
			}}), test.ShouldBeNil)
			err = pm.AddProcessFromConfig(ctx, ProcessConfig{Name: "bash", Args: []string{
				"-c", fmt.Sprintf("echo two >> %s\nwhile true; do echo hey2; sleep 1; done", temp2.Name()),
			}, OneShot: true})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "killed")
		})
	})

	t.Run("an empty manager starts processes after start", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		defer func() {
			test.That(t, pm.Stop(), test.ShouldBeNil)
		}()

		temp, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp.Name())

		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true}), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo world >> %s`, temp.Name())}, OneShot: true}), test.ShouldBeNil)

		rd, err := ioutil.ReadFile(temp.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldBeEmpty)

		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

		rd, err = ioutil.ReadFile(temp.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(rd), test.ShouldEqual, "hello\nworld\n")
	})

	t.Run("an error starting stops other processes", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		defer func() {
			test.That(t, pm.Stop(), test.ShouldBeNil)
		}()

		fp := &fakeProcess{}
		test.That(t, pm.AddProcess(context.Background(), fp), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", "sleep 1; exit 2",
		}, OneShot: true}), test.ShouldBeNil)

		err := pm.Start(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "exit status 2")

		test.That(t, fp.stopCount, test.ShouldEqual, 1)
	})
}

func TestProcessManagerStop(t *testing.T) {
	t.Run("an empty manager stop does nothing", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		test.That(t, pm.Stop(), test.ShouldBeNil)
		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)
		test.That(t, pm.Stop(), test.ShouldBeNil)
		test.That(t, pm.Start(context.Background()), test.ShouldEqual, errAlreadyStopped)
	})

	t.Run("running processes are stopped", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)

		temp1, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp1.Name())
		temp2, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp2.Name())
		temp3, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp3.Name())

		watcher, err := fsnotify.NewWatcher()
		test.That(t, err, test.ShouldBeNil)
		defer watcher.Close()
		watcher.Add(temp1.Name())
		watcher.Add(temp2.Name())
		watcher.Add(temp3.Name())

		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"exit 0\" SIGINT; echo one >> %s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
		}}), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"exit 0\" SIGINT; echo two >> %s\nwhile true; do echo hey2; sleep 1; done", temp2.Name()),
		}}), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo hey\" SIGINT; echo three >> %s\nwhile true; do echo hey3; sleep 1; done", temp3.Name()),
		}}), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", "echo hello",
		}, OneShot: true}), test.ShouldBeNil)
		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

		<-watcher.Events
		<-watcher.Events
		<-watcher.Events

		test.That(t, pm.Stop(), test.ShouldBeNil)

		rd, err := ioutil.ReadFile(temp1.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(rd), test.ShouldEqual, "one\n")
		rd, err = ioutil.ReadFile(temp2.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(rd), test.ShouldEqual, "two\n")
		rd, err = ioutil.ReadFile(temp3.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(rd), test.ShouldEqual, "three\n")
	})

	t.Run("all processes are stopped even if they error", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

		temp1, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp1.Name())
		temp2, err := ioutil.TempFile("", "*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp2.Name())

		watcher, err := fsnotify.NewWatcher()
		test.That(t, err, test.ShouldBeNil)
		defer watcher.Close()
		watcher.Add(temp1.Name())
		watcher.Add(temp2.Name())

		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo done >> %[1]s;exit 0\" SIGINT; echo one >> %[1]s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
		}}), test.ShouldBeNil)
		fp := &fakeProcess{stopErr: true}
		test.That(t, pm.AddProcess(context.Background(), fp), test.ShouldBeNil)
		test.That(t, pm.AddProcessFromConfig(context.Background(), ProcessConfig{Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo done >> %[1]s;exit 0\" SIGINT; echo two >> %[1]s\nwhile true; do echo hey12 sleep 1; done", temp2.Name()),
		}}), test.ShouldBeNil)

		<-watcher.Events
		<-watcher.Events

		err = pm.Stop()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stop")

		<-watcher.Events
		<-watcher.Events
	})
}
