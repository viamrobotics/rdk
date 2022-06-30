package rtk

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
)

func TestRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	cfig := config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "ntrip",
			"ntrip_addr": "some_ntrip_address",
			"ntrip_username": "skarpoor",
			"ntrip_password": "plswork",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"children": [
			  "gps1"
			]
		},
	}

	g := newRTKGPS(ctx, cfig, logger)

	// test NTRIPConnection Source
	

	// test serial connection source
	cfig := config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "serial",
			"correction_path": "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00",
			"children": [
			  "gps1"
			]
		},
	}
	s, err := newSerialCorrectionSource(ctx, )

	// test I2C correction source

	// test invalid source


}
