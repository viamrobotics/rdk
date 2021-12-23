// Package ros implements functionality that bridges the gap between `rdk` and ROS
package ros

import (
	"os"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/starship-technologies/gobag/rosbag"

	"go.viam.com/utils"
)

// ReadBag reads the contents of a rosbag into a gobag data structure
func ReadBag(filename string, logger golog.Logger) (*rosbag.RosBag, error) {
	logger.Debugw("working with bag file", "name", filename)

	f, err := os.Open(filename)
	defer utils.UncheckedErrorFunc(f.Close)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open input file")
	}

	rb := rosbag.NewRosBag()

	if err := rb.Read(f); err != nil {
		return nil, errors.Wrapf(err, "unable to create ros bag, error")
	}

	logger.Debugw("done with bag file", "name", filename)
	return rb, nil
}

// WriteTopicsJSON writes data from a rosbag into JSON files, filtered and sorted by topic.
func WriteTopicsJSON(rb *rosbag.RosBag, startTime, endTime int64, topicsFilter []string, logger golog.Logger) error {
	logger.Debugw("Starting WriteTopicsJSON")
	var timeFilterFunc func(int64) bool
	if startTime == 0 || endTime == 0 {
		timeFilterFunc = func(timestamp int64) bool {
			return true
		}

	} else {
		timeFilterFunc = func(timestamp int64) bool {
			return timestamp >= startTime && timestamp <= endTime
		}
	}

	var topicFilterFunc func(string) bool
	if len(topicsFilter) == 0 {
		topicFilterFunc = func(string) bool {
			return true
		}
	} else {
		topicsFilterMap := make(map[string]bool)
		for _, topic := range topicsFilter {
			topicsFilterMap[topic] = true
		}
		topicFilterFunc = func(topic string) bool {
			_, ok := topicsFilterMap[topic]
			return ok
		}
	}

	if err := rb.ParseTopicsToJSON("", timeFilterFunc, topicFilterFunc, false); err != nil {
		return errors.Wrapf(err, "error while parsing bag to JSON")
	}

	return nil
}
