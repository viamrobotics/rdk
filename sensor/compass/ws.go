package compass

import (
	"context"
	"math"

	"github.com/edaniels/wsapi"
)

const (
	WSCommandHeading = "heading"
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

func (wsd *WSDevice) StartCalibration(ctx context.Context) error {
	return nil
}

func (wsd *WSDevice) StopCalibration(ctx context.Context) error {
	return nil
}

func (wsd *WSDevice) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := wsd.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (wsd *WSDevice) Heading(ctx context.Context) (float64, error) {
	resp, err := wsd.conn.SendCommand(ctx, WSCommandHeading)
	if err != nil {
		return math.NaN(), err
	}
	var heading float64
	return heading, resp.Unmarshal(&heading)
}

func (wsd *WSDevice) Close(ctx context.Context) error {
	wsd.conn.Close()
	return nil
}
