// Package logging is a thread-safe way to log video device information to a file. On startup, this package creates a
// unique filename and uses that filename throughout the lifetime of the program to log information such as which video
// devices are V4L2 compatible and the current operating system.
package logging

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

var (
	// GLoggerCamComp is the global logger-to-file for camera components.
	GLoggerCamComp *Logger
	filePath       string
)

func init() {
	t := time.Now().UTC().Format(time.RFC3339)
	filePath = filepath.Join(config.ViamDotDir, "debug", "components", "camera", fmt.Sprintf("%s.txt", t))

	var err error
	if GLoggerCamComp, err = NewLogger(); err != nil && !errors.Is(err, UnsupportedError{}) {
		log.Println("cannot create new logger: ", err)
	}
}

// InfoMap is a map of information to be written to the log.
type InfoMap = map[string]string

// Logger is a thread-safe logger that manages a single log file in config.ViamDotDir.
type Logger struct {
	infoCh    chan info
	logger    logging.Logger
	isRunning atomic.Bool
	seenPath  map[string]bool
	seenMap   map[string]InfoMap
}

type info struct {
	title string
	m     InfoMap
}

const (
	// keep at most 3 log files in dir.
	maxFiles = 3
	linux    = "linux"
)

// UnsupportedError indicates this feature is not supported on the current platform.
type UnsupportedError struct{}

func (e UnsupportedError) Error() string {
	return "unsupported OS: cannot emit logs to file for camera component"
}

// NewLogger creates a new logger. Call Logger.Start to start logging.
func NewLogger() (*Logger, error) {
	// TODO: support non-Linux platforms
	if runtime.GOOS != linux {
		return nil, UnsupportedError{}
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "camera logger: cannot mkdir "+dir)
	}

	// remove enough entries to keep the number of files <= maxFiles
	for entries, err := os.ReadDir(dir); len(entries) >= maxFiles; entries, err = os.ReadDir(dir) {
		if err != nil {
			utils.UncheckedError(errors.Wrap(err, "camera logger: cannot read directory "+dir))
			break
		}

		// because entries are sorted by name (timestamp), earlier entries are removed first
		if err = os.Remove(filepath.Join(dir, entries[0].Name())); err != nil {
			utils.UncheckedError(errors.Wrap(err, "camera logger: cannot remove file "+filepath.Join(dir, entries[0].Name())))
			break
		}
	}

	cfg := logging.NewZapLoggerConfig()
	cfg.OutputPaths = []string{filePath}

	// only keep message
	cfg.EncoderConfig.TimeKey = ""
	cfg.EncoderConfig.LevelKey = ""
	cfg.EncoderConfig.NameKey = ""
	cfg.EncoderConfig.CallerKey = ""
	cfg.EncoderConfig.StacktraceKey = ""

	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		infoCh: make(chan info),
		logger: logging.FromZapCompatible(logger.Sugar().Named("camera_debugger")),
	}, nil
}

// Start creates and initializes the logging file and periodically emits logs to it. This method is thread-safe.
func (l *Logger) Start(ctx context.Context) error {
	// TODO: support non-Linux platforms
	if runtime.GOOS != linux {
		return UnsupportedError{}
	}

	if l == nil {
		return nil
	}

	if prevVal := l.isRunning.Swap(true); prevVal {
		return nil // already running; nothing to do
	}

	utils.PanicCapturingGo(func() {
		vsourceMetaLogger := logging.Global().Sublogger("videosource")
		vsourceMetaLogger.CInfo(ctx, "Starting videosource logger")
		defer vsourceMetaLogger.CInfo(ctx, "Terminating videosource logger")

		l.init()
		ticker := time.NewTicker(1 * time.Second)
		shouldReset := time.NewTimer(12 * time.Hour)
		for {
			select {
			case <-ctx.Done():
				return
			case <-shouldReset.C:
				l.init()
			default:
			}

			select {
			case <-ctx.Done():
				return
			case info := <-l.infoCh:
				l.write(info.title, info.m)
			case <-ticker.C:
				l.captureV4L2info()
			}
		}
	})
	return nil
}

// Log emits the data stored in the given InfoMap with the given title to the log file. This method is thread-safe.
func (l *Logger) Log(title string, m InfoMap) error {
	// TODO: support non-Linux platforms
	if runtime.GOOS != linux {
		return UnsupportedError{}
	}

	if l == nil {
		return nil
	}

	if !l.isRunning.Load() {
		return errors.New("must start logger")
	}

	l.infoCh <- info{title, m}
	return nil
}

func (l *Logger) captureV4L2info() {
	v4l2Info := make(InfoMap)
	v4l2Compliance := make(InfoMap)
	err := filepath.Walk("/dev", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasPrefix(path, "/dev/video") {
			return nil
		}
		if l.seenPath[path] {
			return nil
		}

		// some devices may not have a symbolic link under /dev/v4l so we log info from all /dev/videoN paths we find.
		v4l2Info[path] = runCommand("v4l2-ctl", "--device", path, "--all")
		v4l2Compliance[path] = runCommand("v4l2-compliance", "--device", path)
		l.seenPath[path] = true
		return nil
	})
	l.logError(err, "cannot walk filepath")

	v4l2Path := make(InfoMap)
	err = filepath.Walk("/dev/v4l/by-path", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if l.seenPath[path] {
			return nil
		}
		v4l2Path["by-path"] = filepath.Base(path)
		l.seenPath[path] = true
		return nil
	})
	l.logError(err, "cannot walk filepath")

	v4l2ID := make(InfoMap)
	err = filepath.Walk("/dev/v4l/by-id", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if l.seenPath[path] {
			return nil
		}
		v4l2ID["by-id"] = filepath.Base(path)
		l.seenPath[path] = true
		return nil
	})
	l.logError(err, "cannot walk filepath")

	// Video capture and overlay devices' minor numbers range from [0,63]
	// https://www.kernel.org/doc/html/v4.16/media/uapi/v4l/diff-v4l.html
	for n := 0; n < 63; n++ {
		path := fmt.Sprintf("/dev/video%d", n)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// when this file is re-created we won't know whether it's the same device. Better to assume not.
			l.seenPath[path] = false
		}
	}

	l.write("v4l2 control", v4l2Info)
	l.write("v4l2 compliance", v4l2Compliance)
	l.write("v4l2 paths", v4l2Path)
	l.write("v4l2 ID", v4l2ID)
}

func (l *Logger) init() {
	err := os.Truncate(filePath, 0)
	l.logError(err, "cannot truncate file")

	l.seenPath = make(map[string]bool)
	l.seenMap = make(map[string]InfoMap)
	l.write("system information", InfoMap{
		"kernel":    runCommand("uname", "--kernel-name"),
		"machine":   runCommand("uname", "--machine"),
		"processor": runCommand("uname", "--processor"),
		"platform":  runCommand("uname", "--hardware-platform"),
		"OS":        runCommand("uname", "--operating-system"),
		"lscpu":     runCommand("lscpu"),
		"model":     runCommand("cat", "/proc/device-tree/model"),
	})
}

func runCommand(name string, args ...string) string {
	//nolint:errcheck
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

func (l *Logger) write(title string, m InfoMap) {
	if len(m) == 0 {
		return
	}
	if oldM, ok := l.seenMap[title]; ok && reflect.DeepEqual(oldM, m) {
		return // don't log the same info twice
	}

	l.seenMap[title] = m
	t := table.NewWriter()
	t.SetAllowedRowLength(120)
	t.SuppressEmptyColumns()
	t.SetStyle(table.StyleLight)
	t.SetTitle(strings.ToUpper(title))

	splitLine := func(line string) table.Row {
		var row table.Row
		for _, ele := range strings.Split(line, ":") {
			row = append(row, strings.TrimSpace(ele))
		}
		return row
	}

	for k, v := range m {
		lines := strings.Split(v, "\n")
		t.AppendRow(append(table.Row{k}, splitLine(lines[0])...))
		for i := 1; i < len(lines); i++ {
			line := lines[i]
			if strings.ReplaceAll(line, " ", "") == "" {
				continue
			}

			t.AppendRow(append(table.Row{""}, splitLine(line)...))
		}
		t.AppendSeparator()
	}

	t.AppendFooter(table.Row{time.Now().UTC().Format(time.RFC3339)})
	l.logger.Info(t.Render())
	l.logger.Info()
}

func (l *Logger) logError(err error, msg string) {
	if l != nil && err != nil {
		l.write("error", InfoMap{msg: err.Error()})
	}
}
