// Package ros implements functionality that bridges the gap between `core` and ROS
package ros

import (
	"log"
	"os"

	"github.com/go-errors/errors"

	"github.com/starship-technologies/gobag/rosbag"
)

// ReadBag reads the contents of a rosbag into a gobag data structure
func ReadBag(filename string) (*rosbag.RosBag, error) {
	log.Printf("Working with bag file %v.", filename)

	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Errorf("Unable to open input file, error %w", err)
	}

	rb := rosbag.NewRosBag()
	err = rb.Read(f)
	if err != nil {
		return nil, errors.Errorf("Unable to create ros bag, error %w", err)
	}

	err = f.Close()
	if err != nil {
		return nil, err
	}
	log.Printf("Done with bag file %v.", filename)
	return rb, nil
}

// WriteTopicsJSON writes data from a rosbag into JSON files, filtered and sorted by topic.
func WriteTopicsJSON(rb *rosbag.RosBag, startTime int64, endTime int64, topicsFilter []string) error {
	log.Println("Starting WriteTopicsJSON")
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

	err := rb.ParseTopicsToJSON("", timeFilterFunc, topicFilterFunc, false)
	if err != nil {
		return errors.Errorf("Error while parsing bag to JSON, error %w", err)
	}

	return nil
}
