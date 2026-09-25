package video

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getVideo method = iota
	doCommand
)

func (m method) String() string {
	switch m {
	case getVideo:
		return "GetVideo"
	case doCommand:
		return "DoCommand"
	}
	return "Unknown"
}

func newGetVideoCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	videoSvc, err := assertVideo(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		startTime := timeRequested
		endTime := startTime.Add(params.Interval)

		videoChan, err := videoSvc.GetVideo(ctx, startTime, endTime, "h264", "mp4", data.FromDMExtraMap)
		if err != nil {
			if data.IsNoCaptureToStoreError(err) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, getVideo.String(), err)
		}

		var payload []byte
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case chunk, ok := <-videoChan:
				if !ok {
					break loop
				}
				payload = append(payload, chunk.Data...)
			}
		}

		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			Payload:  payload,
			MimeType: "video/mp4",
		}}), nil
	})

	return data.NewCollector(cFunc, params)
}

func assertVideo(resource interface{}) (Service, error) {
	v, ok := resource.(Service)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return v, nil
}

func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	videoSvc, err := assertVideo(resource)
	if err != nil {
		return nil, err
	}
	cFunc := data.NewDoCommandCaptureFunc(videoSvc, params)
	return data.NewCollector(cFunc, params)
}
