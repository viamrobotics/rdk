// Package ros implements functionality that bridges the gap between `rdk` and ROS
package ros

import (
	"encoding/json"
	"io"
	"os"

	"github.com/edaniels/gobag/rosbag"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// ReadBag reads the contents of a rosbag into a gobag data structure.
func ReadBag(filename string) (*rosbag.RosBag, error) {
	//nolint:gosec
	f, err := os.Open(filename)
	defer utils.UncheckedErrorFunc(f.Close)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open input file")
	}

	rb := rosbag.NewRosBag()

	if err := rb.Read(f); err != nil {
		return nil, errors.Wrapf(err, "unable to create ros bag, error")
	}

	return rb, nil
}

// WriteTopicsJSON writes data from a rosbag into JSON files, filtered and sorted by topic.
func WriteTopicsJSON(rb *rosbag.RosBag, startTime, endTime int64, topicsFilter []string) error {
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

// AllMessagesForTopic returns all messages for a specific topic in the ros bag.
func AllMessagesForTopic(rb *rosbag.RosBag, topic string) ([]map[string]interface{}, error) {
	if err := rb.ParseTopicsToJSON(
		"",
		func(int64) bool { return true },
		func(t string) bool { return t == topic },
		false,
	); err != nil {
		return nil, errors.Wrapf(err, "error while parsing bag to JSON")
	}

	msgs := rb.TopicsAsJSON[topic]
	if msgs == nil {
		return nil, errors.Errorf("no messages for topic %s", topic)
	}

	all := []map[string]interface{}{}

	for {
		data, err := msgs.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		message := map[string]interface{}{}
		err = json.Unmarshal(data, &message)
		if err != nil {
			return nil, err
		}

		all = append(all, message)
	}

	return all, nil
}
