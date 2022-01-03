package main

import (
	"context"
	"io"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/sensor/compass/gy511"
	"go.viam.com/rdk/serial"
	"go.viam.com/rdk/testutils/inject"
)

func TestMainMain(t *testing.T) {
	defaultSearchFunc := func(filter serial.SearchFilter) []serial.Description {
		return nil
	}
	searchDevicesFunc := defaultSearchFunc
	prevSearchFunc := serial.Search
	serial.Search = func(filter serial.SearchFilter) []serial.Description {
		return searchDevicesFunc(filter)
	}
	defaultOpenFunc := func(devicePath string) (io.ReadWriteCloser, error) {
		return nil, errors.Errorf("cannot open %s", devicePath)
	}
	prevOpenFunc := serial.Open
	var injectedOpenDeviceFunc func(devicePath string) io.ReadWriteCloser
	openDeviceFunc := defaultOpenFunc
	serial.Open = func(devicePath string) (io.ReadWriteCloser, error) {
		if injectedOpenDeviceFunc != nil {
			return injectedOpenDeviceFunc(devicePath), nil
		}
		if openDeviceFunc == nil {
			return prevOpenFunc(devicePath)
		}
		return openDeviceFunc(devicePath)
	}
	reset := func(t *testing.T, tLogger golog.Logger, _ *testutils.ContextualMainExecution) {
		t.Helper()
		logger = tLogger
		searchDevicesFunc = defaultSearchFunc
		openDeviceFunc = defaultOpenFunc
		injectedOpenDeviceFunc = nil
	}
	defer func() {
		serial.Search = prevSearchFunc
		serial.Open = prevOpenFunc
	}()

	failingDevice := gy511.NewRawGY511()
	failingDevice.SetHeading(5)
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "no suitable", reset, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", reset, nil, nil},
		{"bad calibrate flag", []string{"--calibrate=who"}, "parse", reset, nil, nil},

		// reading
		{"bad device", nil, "directory", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			t.Helper()
			reset(t, tLogger, exec)
			searchDevicesFunc = func(_ serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "/"},
				}
			}
			injectedOpenDeviceFunc = nil
			openDeviceFunc = nil
		}, nil, nil},
		{"faulty device", nil, "whoops2; whoops3", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			t.Helper()
			reset(t, tLogger, exec)
			searchDevicesFunc = func(_ serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
				return &inject.ReadWriteCloser{
					ReadFunc: func(p []byte) (int, error) {
						return 0, errors.New("whoops1")
					},
					WriteFunc: func(p []byte) (int, error) {
						return 0, errors.New("whoops2")
					},
					CloseFunc: func() error {
						return errors.New("whoops3")
					},
				}
			}
		}, nil, nil},
		{"normal device", nil, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			t.Helper()
			reset(t, tLogger, exec)
			searchDevicesFunc = func(_ serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
				rd := gy511.NewRawGY511()
				rd.SetHeading(5)
				return rd
			}
		}, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			t.Helper()
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{
			"normal device with calibrate",
			[]string{"--calibrate"},
			"",
			func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
				t.Helper()
				reset(t, tLogger, exec)
				searchDevicesFunc = func(_ serial.SearchFilter) []serial.Description {
					return []serial.Description{
						{Path: "path"},
					}
				}
				injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
					rd := gy511.NewRawGY511()
					rd.SetHeading(5)
					return rd
				}
				exec.ExpectIters(t, 2)
			}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
				t.Helper()
				exec.QuitSignal(t)
				exec.WaitIters(t)
			}, func(t *testing.T, logs *observer.ObservedLogs) {
				t.Helper()
				test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
				test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			},
		},
		{"failing device", nil, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			t.Helper()
			reset(t, tLogger, exec)
			searchDevicesFunc = func(_ serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
				return failingDevice
			}
			failingDevice.SetFailAfter(4)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			t.Helper()
			exec.WaitIters(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			t.Helper()
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("error reading heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
