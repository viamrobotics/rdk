package builtin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

func TestWriteStreamDiagnostics(t *testing.T) {
	diag := diagnostics.New(time.Minute)
	diag.RecordReceivedJointPositionTargetEvent()
	start := time.Now().Add(-time.Second)

	t.Run("unset path writes nothing", func(t *testing.T) {
		ms := &builtIn{logger: logging.NewTestLogger(t), conf: &Config{}}
		test.That(t, ms.writeStreamDiagnostics("arm", streaming.StreamOptions{}, diag, start, nil), test.ShouldBeNil)
		ms.conf = nil
		test.That(t, ms.writeStreamDiagnostics("arm", streaming.StreamOptions{}, diag, start, nil), test.ShouldBeNil)
	})

	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		ms := &builtIn{logger: logging.NewTestLogger(t), conf: &Config{StreamDiagnosticsFilePath: dir}}
		test.That(t, ms.writeStreamDiagnostics("arm", streaming.StreamOptions{}, diag, start, nil), test.ShouldBeNil)

		files, err := filepath.Glob(filepath.Join(dir, "tag=arm-stream-diagnostics", "tag=arm", "stream-diagnostics-arm-*.json"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldHaveLength, 1)
		bytes, err := os.ReadFile(files[0])
		test.That(t, err, test.ShouldBeNil)
		var doc map[string]any
		test.That(t, json.Unmarshal(bytes, &doc), test.ShouldBeNil)
		test.That(t, doc["arm"], test.ShouldEqual, "arm")
		for _, key := range []string{"stats", "options", "last_window_details", "start_time", "end_time"} {
			_, ok := doc[key]
			test.That(t, ok, test.ShouldBeTrue)
		}
		_, hasErr := doc["error"]
		test.That(t, hasErr, test.ShouldBeFalse)
	})

	t.Run("error session gets the err tag", func(t *testing.T) {
		dir := t.TempDir()
		ms := &builtIn{logger: logging.NewTestLogger(t), conf: &Config{StreamDiagnosticsFilePath: dir}}
		test.That(t, ms.writeStreamDiagnostics("arm", streaming.StreamOptions{}, diag, start, errors.New("boom")), test.ShouldBeNil)

		files, err := filepath.Glob(filepath.Join(dir, "tag=arm-stream-diagnostics-err", "tag=arm", "stream-diagnostics-arm-*-err.json"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldHaveLength, 1)
		bytes, err := os.ReadFile(files[0])
		test.That(t, err, test.ShouldBeNil)
		var doc map[string]any
		test.That(t, json.Unmarshal(bytes, &doc), test.ShouldBeNil)
		test.That(t, doc["error"], test.ShouldEqual, "boom")
	})
}
