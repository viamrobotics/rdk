package lidar

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/edaniels/wsapi"
)

const ModelNameWS = "lidarws"
const DeviceTypeWS = DeviceType("lidarws")

func init() {
	RegisterDeviceType(DeviceTypeWS, DeviceTypeRegistration{
		New: func(ctx context.Context, desc DeviceDescription) (Device, error) {
			return NewWSDevice(ctx, fmt.Sprintf("ws://%s:%d", desc.Host, desc.Port))
		},
	})
}

const (
	WSCommandInfo              = "info"
	WSCommandStart             = "start"
	WSCommandStop              = "stop"
	WSCommandClose             = "close"
	WSCommandScan              = "scan"
	WSCommandRange             = "range"
	WSCommandBounds            = "bounds"
	WSCommandAngularResolution = "angular_resolution"
)

type WSDevice struct {
	conn wsapi.Conn
}

func NewWSDevice(ctx context.Context, address string) (Device, error) {
	conn, err := wsapi.Dial(ctx, address)
	if err != nil {
		return nil, err
	}
	return &WSDevice{conn}, nil
}

func (wsd *WSDevice) Info(ctx context.Context) (map[string]interface{}, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandInfo)
	if err != nil {
		return nil, err
	}
	var info map[string]interface{}
	return info, resp.Unmarshal(&info)
}

func (wsd *WSDevice) Start(ctx context.Context) error {
	_, err := wsd.conn.SendCommand(ctx, WSCommandStart)
	return err
}

func (wsd *WSDevice) Stop(ctx context.Context) error {
	_, err := wsd.conn.SendCommand(ctx, WSCommandStop)
	return err
}

func (wsd *WSDevice) Close(ctx context.Context) error {
	defer wsd.conn.Close()
	_, err := wsd.conn.SendCommand(ctx, WSCommandClose)
	return err
}

// TODO(erd): send options
func (wsd *WSDevice) Scan(ctx context.Context, options ScanOptions) (Measurements, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandScan)
	if err != nil {
		return nil, err
	}
	var measurements Measurements
	return measurements, resp.Unmarshal(&measurements)
}

func (wsd *WSDevice) Range(ctx context.Context) (int, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandRange)
	if err != nil {
		return 0, err
	}
	var devRange int
	return devRange, resp.Unmarshal(&devRange)
}

func (wsd *WSDevice) Bounds(ctx context.Context) (image.Point, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandBounds)
	if err != nil {
		return image.Point{}, err
	}
	var bounds struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	return image.Point(bounds), resp.Unmarshal(&bounds)
}

func (wsd *WSDevice) AngularResolution(ctx context.Context) (float64, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandAngularResolution)
	if err != nil {
		return math.NaN(), err
	}
	var angRes float64
	return angRes, resp.Unmarshal(&angRes)
}
