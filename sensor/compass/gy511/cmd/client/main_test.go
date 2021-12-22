package main

import (
	"context"
	"io"
	"testing"

	"github.com/pkg/errors"

	"go.uber.org/zap/zaptest/observer"

	"go.viam.com/utils/testutils"

	"go.viam.com/core/sensor/compass/gy511"
	"go.viam.com/core/serial"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
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
	openDeviceFunc := defaultOpenFunc
	serial.Open = func(devicePath string) (io.ReadWriteCloser, error) {
		if openDeviceFunc == nil {
			return prevOpenFunc(devicePath)
		}
		return openDeviceFunc(devicePath)
	}
	reset := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		searchDevicesFunc = defaultSearchFunc
		openDeviceFunc = defaultOpenFunc
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
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "/"},
				}
			}
			openDeviceFunc = nil
		}, nil, nil},
		{"faulty device", nil, "whoops2; whoops3", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
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
				}, nil
			}
		}, nil, nil},
		{"normal device", nil, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
				rd := gy511.NewRawGY511()
				rd.SetHeading(5)
				return rd, nil
			}
		}, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"normal device with calibrate", []string{"--calibrate"}, "",
			func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
				reset(t, tLogger, exec)
				searchDevicesFunc = func(filter serial.SearchFilter) []serial.Description {
					return []serial.Description{
						{Path: "path"},
					}
				}
				openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
					rd := gy511.NewRawGY511()
					rd.SetHeading(5)
					return rd, nil
				}
				exec.ExpectIters(t, 2)
			}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
				exec.QuitSignal(t)
				exec.WaitIters(t)
			}, func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
				test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			}},
		{"failing device", nil, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) []serial.Description {
				return []serial.Description{
					{Path: "path"},
				}
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
				return failingDevice, nil
			}
			failingDevice.SetFailAfter(4)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			exec.WaitIters(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("error reading heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
