package wifi

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/registry"
	"go.viam.com/test"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	return make(registry.Dependencies)
}

func createWirelessInfoPath(t *testing.T) *os.File {
	t.Helper()

	dirPath := t.TempDir()
	file, err := ioutil.TempFile(dirPath, "wireless")
	test.That(t, err, test.ShouldBeNil)

	return file
}

func TestNewSensor(t *testing.T) {
	ctx := context.Background()
	deps := setupDependencies(t)
	logger := golog.NewLogger("testlog")

	file := createWirelessInfoPath(t)

	_, err := newWifi(ctx, deps, "fake-wifi", logger, "wrong-path")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = newWifi(ctx, deps, "fake-wifi", logger, file.Name())
	test.That(t, err, test.ShouldBeNil)
}

func TestReadings(t *testing.T) {
	ctx := context.Background()
	deps := setupDependencies(t)
	logger := golog.NewLogger("testlog")

	file := createWirelessInfoPath(t)
	_, err := file.WriteString(`Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
XXXXXXXXX: XXXX   58.  -52.  -256        X      X      X      X  XXXXX        X`)
	test.That(t, err, test.ShouldBeNil)

	sensor, err := newWifi(ctx, deps, "fake-wifi", logger, file.Name())
	test.That(t, err, test.ShouldBeNil)

	readings, err := sensor.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	expected := map[string]interface{}{
		"link":  int(58),
		"level": int(-52),
		"noise": int(-256),
	}
	test.That(t, readings, test.ShouldResemble, expected)
}
