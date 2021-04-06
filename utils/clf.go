package utils

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// reader for CLF (CARMEN Logfile) log files
// see http://carmen.sourceforge.net/logger_playback.html
/* Common example
# CARMEN Logfile
# file format is one message per line
# message_name [message contents] ipc_timestamp ipc_hostname logger_timestamp
# message formats defined    : PARAM SYNC ODOM RAWLASER1 RAWLASER2 RAWLASER3 RAWLASER4
#                              ROBOTLASER1 ROBOTLASER2
# old message formats defined: FLASER RLASER LASER3 LASER4
# PARAM param_name param_value
# SYNC tagname
# ODOM x y theta tv rv accel
# TRUEPOS true_x true_y true_theta odom_x odom_y odom_theta
# RAWLASER1 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] num_remissions [remission values]
# RAWLASER2 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] num_remissions [remission values]
# RAWLASER3 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] num_remissions [remission values]
# RAWLASER4 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] num_remissions [remission values]
# ROBOTLASER1 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] laser_pose_x laser_pose_y laser_pose_theta
#   robot_pose_x robot_pose_y robot_pose_theta
#   laser_tv laser_rv forward_safety_dist side_safety_dist
# ROBOTLASER2 laser_type start_angle field_of_view angular_resolution
#   maximum_range accuracy remission_mode
#   num_readings [range_readings] laser_pose_x laser_pose_y laser_pose_theta
#   robot_pose_x robot_pose_y robot_pose_theta
#   laser_tv laser_rv forward_safety_dist side_safety_dist
# NMEAGGA gpsnr utc latitude lat_orient longitude long_orient gps_quality
#   num_satellites hdop sea_level alititude geo_sea_level geo_sep data_age
# NMEARMC gpsnr validity utc latitude lat_orient longitude long_orient
#   speed course variation var_dir date
*/
/* Common messages (http://carmen.sourceforge.net/doc/binary__loggerplayback.html)
RAWLASER messages, which correspond to the raw laser information obtained by the laser driver
ODOM messages, which correspond to the odometry information provided by the base
ROBOTLASER messages, which is the merges message of odometry and laser data.
TRUEPOS message, which provides ground truth information (in case the simulator is used).
NMEAGGA message, which provides the position estimate of the gps
NMEARMC message, which provides the ground speed information of the gps
PARAM message, contain the parameters of the ini file as well as updated parameters
SYNC message
*/
type CLFReader struct {
	format       []string
	messageTypes map[string][]string
}

func (r *CLFReader) processMeta(line string) error {
	if strings.HasPrefix(line, "message formats defined") {
		// ignored this
		return nil
	}

	if len(r.format) == 0 {
		r.format = clfSplit(line)
		return nil
	}

	if r.messageTypes == nil {
		r.messageTypes = map[string][]string{}
	}

	pcs := clfSplit(line)
	r.messageTypes[pcs[0]] = pcs[1:]

	numArrays := 0
	for _, s := range pcs {
		if s[0] == '[' {
			numArrays++
		}
	}

	if numArrays > 1 {
		return fmt.Errorf("too many arrays (%d) in (%s)", numArrays, pcs)
	}

	return nil
}

func (r *CLFReader) combineFormats(sub []string) []string {
	n := []string{}

	for _, s := range r.format {
		if s == "[message contents]" {
			n = append(n, sub...)
		} else {
			n = append(n, s)
		}
	}

	return n
}

func (r *CLFReader) processPiece(p string) interface{} {
	for _, c := range p {
		if c == '.' || c == '-' || unicode.IsDigit(c) {
			continue
		}
		return p
	}
	x, err := strconv.ParseFloat(p, 64)
	if err == nil {
		return x
	}
	return p
}

func (r *CLFReader) processLine(line string) (map[string]interface{}, error) {
	if len(line) == 0 {
		return nil, nil
	}

	if line[0] == '#' {
		return nil, r.processMeta(line[2:])
	}

	pcs := strings.Split(line, " ")
	msgFormat := r.messageTypes[pcs[0]]
	if len(msgFormat) == 0 {
		return nil, fmt.Errorf("unknown type %s", pcs[0])
	}

	msgFormat = r.combineFormats(msgFormat)

	m := map[string]interface{}{}

	offset := 0
	for idx, key := range msgFormat {
		if key[0] == '[' {
			v := []interface{}{}

			for ; offset <= (len(pcs) - len(msgFormat)); offset++ {
				v = append(v, r.processPiece(pcs[idx+offset]))
			}

			key = key[1:]
			key = key[0 : len(key)-1]
			m[key] = v
			offset--
			continue
		}
		v := ""
		if idx < len(pcs) {
			v = pcs[idx+offset]
		}
		m[key] = r.processPiece(v)
	}

	return m, nil
}

func (r *CLFReader) Process(reader io.Reader, f func(data map[string]interface{}) error) error {
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadString('\n')
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)

		res, err := r.processLine(line)
		if err != nil {
			return err
		}

		if res == nil {
			continue
		}

		err = f(res)
		if err != nil {
			return err
		}
	}
}

func clfSplit(s string) []string {
	pcs := strings.Split(s, " ")

	n := []string{}
	inArray := false
	for _, s := range pcs {

		if inArray {
			n[len(n)-1] = n[len(n)-1] + " " + s
			if s[len(s)-1] == ']' {
				inArray = false
			}
			continue
		}

		if s[0] != '[' {
			n = append(n, s)
			continue
		}
		n = append(n, s)
		inArray = s[len(s)-1] != ']'
	}
	return n
}
