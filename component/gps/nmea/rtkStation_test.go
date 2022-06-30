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

//test NTRIPConnection Source
func TestNTRIP(t *testing.T) {
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
}

//test serial connection source

//test I2C correction source


//test invalid source

//check writing to all ports and i2c handles

//test close function
