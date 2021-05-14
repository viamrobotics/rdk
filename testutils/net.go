package testutils

import (
	"context"
	"net"
	"time"

	"github.com/go-errors/errors"
	"go.uber.org/multierr"
)

var waitDur = 5 * time.Second

// WaitSuccessfulDial waits for a dial attempt to succeed.
func WaitSuccessfulDial(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitDur)
	var lastErr error
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return multierr.Combine(ctx.Err(), lastErr)
		default:
		}
		var conn net.Conn
		conn, lastErr = net.Dial("tcp", address)
		if lastErr == nil {
			return conn.Close()
		}
		lastErr = errors.Wrap(lastErr, 0)
	}
}
