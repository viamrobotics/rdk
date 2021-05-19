package rexec

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/fsnotify/fsnotify"
	"go.viam.com/test"

	"go.viam.com/core/utils"
)

func TestProcessManagerProcessIDs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm := NewProcessManager(logger)
	defer func() {
		test.That(t, pm.Stop(), test.ShouldBeNil)
	}()
	test.That(t, pm.ProcessIDs(), test.ShouldBeEmpty)

	fp := &fakeProcess{id: "1"}
	_, err := pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(pm.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1"))

	fp = &fakeProcess{id: "1"}
	_, err = pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(pm.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1"))

	fp = &fakeProcess{id: "2"}
	_, err = pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(pm.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	_, ok := pm.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, utils.NewStringSet(pm.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("2"))
}

func TestProcessManagerProcessByID(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm := NewProcessManager(logger)
	defer func() {
		test.That(t, pm.Stop(), test.ShouldBeNil)
	}()
	_, ok := pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeFalse)

	fp := &fakeProcess{id: "1"}
	_, err := pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)

	proc, ok := pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp)

	_, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeFalse)

	fp1 := &fakeProcess{id: "1"}
	_, err = pm.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)

	proc, ok = pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)

	fp2 := &fakeProcess{id: "2"}
	_, err = pm.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)

	proc, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)

	proc, ok = pm.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)
	proc, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)
}

func TestProcessManagerRemoveProcessByID(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm := NewProcessManager(logger)
	defer func() {
		test.That(t, pm.Stop(), test.ShouldBeNil)
	}()
	_, ok := pm.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeFalse)

	fp1 := &fakeProcess{id: "1"}
	_, err := pm.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)

	_, ok = pm.RemoveProcessByID("2")
	test.That(t, ok, test.ShouldBeFalse)
	proc, ok := pm.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)

	_, ok = pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeFalse)

	_, err = pm.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)

	fp2 := &fakeProcess{id: "2"}
	_, err = pm.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)

	proc, ok = pm.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)

	proc, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)
}

func TestProcessManagerAddProcess(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm := NewProcessManager(logger)
	defer func() {
		test.That(t, pm.Stop(), test.ShouldBeNil)
	}()
	_, ok := pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeFalse)

	fp := &fakeProcess{id: "1"}
	_, err := pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)

	proc, ok := pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp)

	fp2 := &fakeProcess{id: "2"}
	_, err = pm.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)

	newFP := &fakeProcess{id: "1"}
	oldProc, err := pm.AddProcess(context.Background(), newFP, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oldProc, test.ShouldResemble, fp)

	proc, ok = pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, newFP)

	proc, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)

	test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

	fp = &fakeProcess{id: "1", startErr: true}
	newProc, err := pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "start")
	test.That(t, newProc, test.ShouldBeNil)

	proc, ok = pm.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, newFP)

	fp = &fakeProcess{id: "1", startErr: false}
	_, err = pm.AddProcess(context.Background(), fp, true)
	test.That(t, err, test.ShouldBeNil)
}

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

			_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "1", Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true})
			test.That(t, err, test.ShouldBeNil)

			rd, err := ioutil.ReadFile(temp.Name())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, string(rd), test.ShouldEqual, "hello\n")

			// starting again should do nothing
			test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

			// a canceled ctx should fail immediately for one shot only
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err = pm.AddProcessFromConfig(ctx, ProcessConfig{ID: "2", Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
			_, err = pm.AddProcessFromConfig(ctx, ProcessConfig{ID: "3", Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}})
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

			_, err = pm.AddProcessFromConfig(ctx, ProcessConfig{ID: "4", Name: "bash", Args: []string{
				"-c", fmt.Sprintf("echo one >> %s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
			}})
			test.That(t, err, test.ShouldBeNil)
			_, err = pm.AddProcessFromConfig(ctx, ProcessConfig{ID: "5", Name: "bash", Args: []string{
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

		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "1", Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo hello >> %s`, temp.Name())}, OneShot: true})
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "2", Name: "bash", Args: []string{"-c", fmt.Sprintf(`echo world >> %s`, temp.Name())}, OneShot: true})
		test.That(t, err, test.ShouldBeNil)

		rd, err := ioutil.ReadFile(temp.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldBeEmpty)

		test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

		rd, err = ioutil.ReadFile(temp.Name())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(rd), test.ShouldContainSubstring, "hello\n")
		test.That(t, string(rd), test.ShouldContainSubstring, "world\n")
	})

	t.Run("an error starting stops other processes", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		pm := NewProcessManager(logger)
		defer func() {
			test.That(t, pm.Stop(), test.ShouldBeNil)
		}()

		fp := &fakeProcess{id: "1"}
		_, err := pm.AddProcess(context.Background(), fp, true)
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "2", Name: "bash", Args: []string{
			"-c", "sleep 1; exit 2",
		}, OneShot: true})
		test.That(t, err, test.ShouldBeNil)

		err = pm.Start(context.Background())
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

		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "1", Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"exit 0\" SIGINT; echo one >> %s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
		}})
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "2", Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"exit 0\" SIGINT; echo two >> %s\nwhile true; do echo hey2; sleep 1; done", temp2.Name()),
		}})
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "3", Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo hey\" SIGINT; echo three >> %s\nwhile true; do echo hey3; sleep 1; done", temp3.Name()),
		}})
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "4", Name: "bash", Args: []string{
			"-c", "echo hello",
		}, OneShot: true})
		test.That(t, err, test.ShouldBeNil)
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

		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "1", Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo done >> %[1]s;exit 0\" SIGINT; echo one >> %[1]s\nwhile true; do echo hey1; sleep 1; done", temp1.Name()),
		}})
		test.That(t, err, test.ShouldBeNil)
		fp := &fakeProcess{id: "2", stopErr: true}
		_, err = pm.AddProcess(context.Background(), fp, true)
		test.That(t, err, test.ShouldBeNil)
		_, err = pm.AddProcessFromConfig(context.Background(), ProcessConfig{ID: "3", Name: "bash", Args: []string{
			"-c", fmt.Sprintf("trap \"echo done >> %[1]s;exit 0\" SIGINT; echo two >> %[1]s\nwhile true; do echo hey12 sleep 1; done", temp2.Name()),
		}})
		test.That(t, err, test.ShouldBeNil)

		<-watcher.Events
		<-watcher.Events

		err = pm.Stop()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stop")

		<-watcher.Events
		<-watcher.Events
	})
}

func TestProcessManagerClone(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm := NewProcessManager(logger)
	defer func() {
		test.That(t, pm.Stop(), test.ShouldBeNil)
	}()

	test.That(t, pm.Start(context.Background()), test.ShouldBeNil)
	clone1 := pm.Clone()
	test.That(t, clone1.ProcessIDs(), test.ShouldBeEmpty)

	fp1 := &fakeProcess{id: "1"}
	fp2 := &fakeProcess{id: "2"}
	fp3 := &fakeProcess{id: "3"}

	_, err := pm.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm.AddProcess(context.Background(), fp3, true)
	test.That(t, err, test.ShouldBeNil)

	clone2 := pm.Clone()
	test.That(t, utils.NewStringSet(clone2.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2", "3"))
	proc, ok := clone2.RemoveProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)

	proc, ok = pm.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp2)

	_, ok = clone2.ProcessByID("2")
	test.That(t, ok, test.ShouldBeFalse)

	test.That(t, clone2.Stop(), test.ShouldBeNil)
	// can still start since original not stopped
	test.That(t, pm.Start(context.Background()), test.ShouldBeNil)

	test.That(t, fp1.stopCount, test.ShouldEqual, 1)
	test.That(t, pm.Stop(), test.ShouldBeNil)
	test.That(t, fp1.stopCount, test.ShouldEqual, 2)
}

func TestMergeAddProcessManagers(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm1 := NewProcessManager(logger)
	defer func() {
		test.That(t, pm1.Stop(), test.ShouldBeNil)
	}()
	pm2 := NewProcessManager(logger)
	defer func() {
		test.That(t, pm2.Stop(), test.ShouldBeNil)
	}()

	fp1 := &fakeProcess{id: "1"}
	fp2 := &fakeProcess{id: "2"}
	fp3 := &fakeProcess{id: "3"}
	fp4 := &fakeProcess{id: "4"}
	fp5 := &fakeProcess{id: "5"}
	fp6 := &fakeProcess{id: "2"}
	fp7 := &fakeProcess{id: "3"}

	_, err := pm1.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm1.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm1.AddProcess(context.Background(), fp3, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp4, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp5, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp6, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp7, true)
	test.That(t, err, test.ShouldBeNil)

	replaced, err := MergeAddProcessManagers(pm1, pm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replaced, test.ShouldHaveLength, 2)
	replacedM := make(map[string]ManagedProcess, 2)
	replacedM[replaced[0].ID()] = replaced[0]
	replacedM[replaced[1].ID()] = replaced[1]
	test.That(t, replacedM, test.ShouldResemble, map[string]ManagedProcess{
		fp2.ID(): fp2,
		fp3.ID(): fp3,
	})

	test.That(t, utils.NewStringSet(pm1.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2", "3", "4", "5"))

	proc, ok := pm1.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)
	proc, ok = pm1.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp6)
	proc, ok = pm1.ProcessByID("3")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp7)
	proc, ok = pm1.ProcessByID("4")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp4)
	proc, ok = pm1.ProcessByID("5")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp5)
}

func TestMergeRemoveProcessManagers(t *testing.T) {
	logger := golog.NewTestLogger(t)
	pm1 := NewProcessManager(logger)
	defer func() {
		test.That(t, pm1.Stop(), test.ShouldBeNil)
	}()
	pm2 := NewProcessManager(logger)
	defer func() {
		test.That(t, pm2.Stop(), test.ShouldBeNil)
	}()

	fp1 := &fakeProcess{id: "1"}
	fp2 := &fakeProcess{id: "2"}
	fp3 := &fakeProcess{id: "3"}
	fp4 := &fakeProcess{id: "4"}
	fp5 := &fakeProcess{id: "5"}
	fp6 := &fakeProcess{id: "2"}
	fp7 := &fakeProcess{id: "3"}

	_, err := pm1.AddProcess(context.Background(), fp1, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm1.AddProcess(context.Background(), fp2, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm1.AddProcess(context.Background(), fp3, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp4, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp5, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp6, true)
	test.That(t, err, test.ShouldBeNil)
	_, err = pm2.AddProcess(context.Background(), fp7, true)
	test.That(t, err, test.ShouldBeNil)

	removed := MergeRemoveProcessManagers(pm1, pm2)
	test.That(t, removed, test.ShouldResemble, []ManagedProcess{fp2, fp3})

	test.That(t, utils.NewStringSet(pm1.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1"))

	proc, ok := pm1.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc, test.ShouldEqual, fp1)
	_, ok = pm1.ProcessByID("2")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = pm1.ProcessByID("3")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = pm1.ProcessByID("4")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = pm1.ProcessByID("5")
	test.That(t, ok, test.ShouldBeFalse)
}
