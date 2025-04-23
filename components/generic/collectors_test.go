package generic_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	captureInterval = time.Millisecond
)

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestCollectors(t *testing.T) {
	start := time.Now()
	buf := tu.NewMockBuffer(t)
	params := data.CollectorParams{
		DataType:      data.CaptureTypeTabular,
		ComponentName: "generic",
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Target:        buf,
		Clock:         clock.New(),
	}

	gens := &inject.GenericComponent{}
	col, err := generic.NewCommandCollector(gens, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
		Metadata: &datasyncpb.SensorMetadata{},
		Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
			"readings": map[string]any{
				"hi": "bye",
			},
		})},
	}})
	buf.Close()
}
