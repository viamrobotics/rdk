package datasync

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"os"
	"sync"
)

type LiveManager interface {
	Sync(md *v1.DataCaptureMetadata, in chan *v1.SensorData, spool *os.File)
	Close()
}

/**
Hmm I actually think the collector should be in charge of spooling.

Its contract can be "poll captureFunc every interval, and write result to OUTPUTS following some priority."
How to best expose the output channel? Get/Set like the target is currently done?
Maybe instead of target file use this[0] queue impl?
Maybe just use the queue instead of combo. That should be way simpler tbh.

[0]: https://github.com/joncrlsn/dque

Sync can just be rewritten to have both file and channel based sync methods. Or maybe one that takes both. Nah, probably
separate.

Then data manager can:
- Build collectors
- Call Sync on collector's output channel.
- Once an INTERVAL, update collector's target file and (if it's been written to), sync it
*/

type liveSyncer struct {
	backgroundWorkers sync.WaitGroup
	logger            golog.Logger
	client            v1.DataSyncServiceClient
}

func (s *liveSyncer) Sync(md *v1.DataCaptureMetadata, in chan *v1.SensorData, spool *os.File) {
	// TODO: support arbitrary files too... or don't? Not sure how that would work with streaming.
	// TODO: add md->spool to spools.

	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		stream, err := s.client.Upload(context.TODO())
		if err != nil {
			// log
			s.logger.Error(err)
		}
		// TODO: First send metadata packet.

		defer s.backgroundWorkers.Done()
		// TODO: use different background workers for subroutine
		s.backgroundWorkers.Add(1)
		uploadChannel := make(chan *v1.SensorData, 100)
		spoolChannel := make(chan *v1.SensorData, 100)
		defer close(uploadChannel)
		defer close(spoolChannel)
		goutils.PanicCapturingGo(func() {
			defer s.backgroundWorkers.Done()
			uploadFromChannel(stream, uploadChannel)
		})
		s.backgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer s.backgroundWorkers.Done()
			spoolFromChannel(uploadChannel, spool)
		})

		for {
			select {
			case x, ok := <-in:
				if ok {
					// Try to throw into upload channel. If not, spool.
					select {
					case uploadChannel <- x:
					// do nothing except throw it in channel
					default:
						// if channel is full, write to spool
						select {
						case spoolChannel <- x:
						// do nothing except spool
						default:
							// if spool is full too, just yeet it. Probably log or something.
						}
					}
				} else {
					// channel was closed, return
					return
				}
			}
		}
	})
}

func uploadFromChannel(stream v1.DataSyncService_UploadClient, in chan *v1.SensorData) {
	for {
		select {
		// TODO: add context case
		case x, ok := <-in:
			if ok {
				// Try to throw into upload channel. If not, spool.
				ur := &v1.UploadRequest{
					UploadPacket: &v1.UploadRequest_SensorContents{SensorContents: x},
				}
				if err := stream.Send(ur); err != nil {
					// TODO: handle error
				}
			} else {
				// channel was closed, return
				return
			}
		}
	}
}

func spoolFromChannel(in chan *v1.SensorData, out *os.File) {
	for {
		select {
		// TODO: add context case
		case x, ok := <-in:
			if ok {
				_, err := pbutil.WriteDelimited(out, x)
				if err != nil {
					// TODO: handle
				}
			} else {
				// channel was closed, return
				return
			}
		}
	}
}

func (s *liveSyncer) Close() {

}
