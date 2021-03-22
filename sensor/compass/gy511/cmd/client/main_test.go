package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/sensor/compass/gy511"
	"go.viam.com/robotcore/serial"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	defaultSearchDevicesFunc := func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
		return nil, nil
	}
	searchDevicesFunc := defaultSearchDevicesFunc
	prevSearchDevicesFunc := serial.SearchDevices
	serial.SearchDevices = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
		return searchDevicesFunc(filter)
	}
	defaultOpenDeviceFunc := func(devicePath string) (io.ReadWriteCloser, error) {
		return nil, fmt.Errorf("cannot open %s", devicePath)
	}
	prevOpenDeviceFunc := serial.OpenDevice
	openDeviceFunc := defaultOpenDeviceFunc
	serial.OpenDevice = func(devicePath string) (io.ReadWriteCloser, error) {
		if openDeviceFunc == nil {
			return prevOpenDeviceFunc(devicePath)
		}
		return openDeviceFunc(devicePath)
	}
	reset := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		searchDevicesFunc = defaultSearchDevicesFunc
		openDeviceFunc = defaultOpenDeviceFunc
	}
	defer func() {
		serial.SearchDevices = prevSearchDevicesFunc
		serial.OpenDevice = prevOpenDeviceFunc
	}()

	failingDevice := gy511.NewRawDevice()
	failingDevice.SetHeading(5)
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "no suitable", reset, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", reset, nil, nil},
		{"bad calibrate flag", []string{"--calibrate=who"}, "parse", reset, nil, nil},
		{"error searching", nil, "whoops", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return nil, errors.New("whoops")
			}
		}, nil, nil},

		// reading
		{"bad device", nil, "directory", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return []serial.DeviceDescription{
					{Path: "/"},
				}, nil
			}
			openDeviceFunc = nil
		}, nil, nil},
		{"faulty device", nil, "whoops2; whoops3", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return []serial.DeviceDescription{
					{Path: "path"},
				}, nil
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
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return []serial.DeviceDescription{
					{Path: "path"},
				}, nil
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
				rd := gy511.NewRawDevice()
				rd.SetHeading(5)
				return rd, nil
			}
		}, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"normal device with calibrate", []string{"--calibrate"}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			reset(t, tLogger, exec)
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return []serial.DeviceDescription{
					{Path: "path"},
				}, nil
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
				rd := gy511.NewRawDevice()
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
			searchDevicesFunc = func(filter serial.SearchFilter) ([]serial.DeviceDescription, error) {
				return []serial.DeviceDescription{
					{Path: "path"},
				}, nil
			}
			openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
				return failingDevice, nil
			}
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			failingDevice.SetFailAfter(0)
			exec.WaitIters(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("readings").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("error reading heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
