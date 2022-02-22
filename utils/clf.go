package utils

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// reader for CLF (CARMEN Logfile) log files
// see http://carmen.sourceforge.net/logger_playback.html
// This does not appear to be an extensible format so it's easier
// to write a reader/writer by just porting over the CARMEN C code.

//nolint
const clfHeader = `# CARMEN Logfile
# file format is one message per line
# message_name [message contents] ipc_timestamp ipc_hostname logger_timestamp
# message formats defined: PARAM SYNC ODOM RAWLASER1 RAWLASER2 RAWLASER3 RAWLASER4 ROBOTLASER1 ROBOTLASER2 FLASER RLASER LASER3 LASER4
# PARAM param_name param_value
# COMMENT text
# SYNC tagname
# ODOM x y theta tv rv accel
# TRUEPOS true_x true_y true_theta odom_x odom_y odom_theta
# RAWLASER1 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values]
# RAWLASER2 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values]
# RAWLASER3 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values]
# RAWLASER4 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values]
# POSITIONLASER laserid x y z phi(roll) theta(pitch) psi(yaw)
# ROBOTLASER1 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values] laser_pose_x laser_pose_y laser_pose_theta robot_pose_x robot_pose_y robot_pose_theta laser_tv laser_rv forward_safety_dist side_safety_dist turn_axis
# ROBOTLASER2 laser_type start_angle field_of_view angular_resolution maximum_range accuracy remission_mode num_readings [range_readings] num_remissions [remission values] laser_pose_x laser_pose_y laser_pose_theta robot_pose_x robot_pose_y robot_pose_theta laser_tv laser_rv forward_safety_dist side_safety_dist turn_axis
# NMEAGGA gpsnr utc latitude_dm lat_orient longitude_dm long_orient gps_quality num_satellites hdop sea_level alititude geo_sea_level geo_sep data_age
# NMEARMC gpsnr validity utc latitude_dm lat_orient longitude_dm long_orient speed course variation var_dir date
# SONAR cone_angle num_sonars [sonar_reading] [sonar_offsets x y theta]
# BUMPER num_bumpers [bumper_reading] [bumper_offsets x y]
# SCANMARK start_stop_indicator laserID
# IMU accelerationX accelerationY accelerationZ quaternion_q0 quaternion_q1 quaternion_q2 quaternion_q3 magneticfieldX magneticfieldY magneticfieldZ gyroX gyroY gyroZ
# VECTORMOVE distance theta
# ROBOTVELOCITY tv rv
# FOLLOWTRAJECTORY x y theta tv rv num readings [trajectory points: x y theta tv rv]
# BASEVELOCITY tv rv
#
# OLD LOG MESSAGES:
# (old) # FLASER num_readings [range_readings] x y theta odom_x odom_y odom_theta
# (old) # RLASER num_readings [range_readings] x y theta odom_x odom_y odom_theta
# (old) # LASER3 num_readings [range_readings]
# (old) # LASER4 num_readings [range_readings]
# (old) # REMISSIONFLASER num_readings [range_readings remission_value]
# (old) # REMISSIONRLASER num_readings [range_readings remission_value]
# (old) # REMISSIONLASER3 num_readings [range_readings remission_value]
# (old) # REMISSIONLASER4 num_readings [range_readings remission_value]`

// A CLFReader can read in CARMEN Logfiles.
type CLFReader struct {
	reader *bufio.Reader
}

// CLFMessage is a specific type of CLF message that always has
// a base message.
type CLFMessage interface {
	Base() CLFBaseMessage
	Type() CLFMessageType
}

// NewCLFReader returns a CLF reader based on the given reader.
func NewCLFReader(reader io.Reader) *CLFReader {
	return &CLFReader{reader: bufio.NewReader(reader)}
}

// Process reads over all messages and calls the given function for
// each message. If the function returns an error, execution stops
// with that error returned to the caller.
func (r *CLFReader) Process(f func(message CLFMessage) error) error {
	// discard directives
	for {
		line, eof, err := r.readLine()
		if err != nil || eof {
			return err
		}
		if line[0] != '#' {
			break
		}
	}

	for {
		line, eof, err := r.readLine()
		if err != nil || eof {
			return err
		}

		res, err := r.processLine(line)
		if err != nil {
			return err
		}

		err = f(res)
		if err != nil {
			return err
		}
	}
}

// CLFMessageType describes a specific type of message.
type CLFMessageType string

// known message  types.
const (
	CLFMessageTypeParam            = CLFMessageType("PARAM")
	CLFMessageTypeComment          = CLFMessageType("COMMENT")
	CLFMessageTypeSync             = CLFMessageType("SYNC")
	CLFMessageTypeOdometry         = CLFMessageType("ODOM")
	CLFMessageTypeTruePos          = CLFMessageType("TRUEPOS")
	CLFMessageTypeRawLaser1        = CLFMessageType("RAWLASER1")
	CLFMessageTypeRawLaser2        = CLFMessageType("RAWLASER2")
	CLFMessageTypeRawLaser3        = CLFMessageType("RAWLASER3")
	CLFMessageTypeRawLaser4        = CLFMessageType("RAWLASER4")
	CLFMessageTypePositionLaser    = CLFMessageType("POSITIONLASER")
	CLFMessageTypeRobotLaser1      = CLFMessageType("ROBOTLASER1")
	CLFMessageTypeRobotLaser2      = CLFMessageType("ROBOTLASER2")
	CLFMessageTypeNMEAGGA          = CLFMessageType("NMEAGGA")
	CLFMessageTypeNMEARMC          = CLFMessageType("NMEARMC")
	CLFMessageTypeSonar            = CLFMessageType("SONAR")
	CLFMessageTypeBumper           = CLFMessageType("BUMPER")
	CLFMessageTypeScanMark         = CLFMessageType("SCANMARK")
	CLFMessageTypeIMU              = CLFMessageType("IMU")
	CLFMessageTypeVectorMove       = CLFMessageType("VECTORMOVE")
	CLFMessageTypeRobotVelocity    = CLFMessageType("ROBOTVELOCITY")
	CLFMessageTypeFollowTrajectory = CLFMessageType("FOLLOWTRAJECTORY")
	CLFMessageTypeBaseVelocity     = CLFMessageType("BASEVELOCITY")
	CLFMessageTypeOld              = CLFMessageType("OLD")
	CLFMessageTypeFrontLaser       = CLFMessageType("FLASER")
	CLFMessageTypeRearLaser        = CLFMessageType("RLASER")
	CLFMessageTypeLaser3           = CLFMessageType("LASER3")
	CLFMessageTypeLaser4           = CLFMessageType("LASER4")
	CLFMessageTypeRemissionFLaser  = CLFMessageType("REMISSIONFLASER")
	CLFMessageTypeRemissionRLaser  = CLFMessageType("REMISSIONRLASER")
	CLFMessageTypeRemissionLaser3  = CLFMessageType("REMISSIONLASER3")
	CLFMessageTypeRemissionLaser4  = CLFMessageType("REMISSIONLASER4")
)

func (r *CLFReader) processLine(line string) (CLFMessage, error) {
	parts := strings.Split(line, " ")
	rest := parts[1:]
	messageType := CLFMessageType(parts[0])
	switch messageType {
	case CLFMessageTypeParam:
		return parseCLFParamMessage(rest)
	case CLFMessageTypeOdometry:
		return parseCLFPOdometryMessage(rest)
	case CLFMessageTypeFrontLaser, CLFMessageTypeRearLaser:
		return parseCLFOldLaserMessage(messageType, rest)
	case CLFMessageTypeComment,
		CLFMessageTypeSync,
		CLFMessageTypeTruePos,
		CLFMessageTypeRawLaser1,
		CLFMessageTypeRawLaser2,
		CLFMessageTypeRawLaser3,
		CLFMessageTypeRawLaser4,
		CLFMessageTypePositionLaser,
		CLFMessageTypeRobotLaser1,
		CLFMessageTypeRobotLaser2,
		CLFMessageTypeNMEAGGA,
		CLFMessageTypeNMEARMC,
		CLFMessageTypeSonar,
		CLFMessageTypeBumper,
		CLFMessageTypeScanMark,
		CLFMessageTypeIMU,
		CLFMessageTypeVectorMove,
		CLFMessageTypeRobotVelocity,
		CLFMessageTypeFollowTrajectory,
		CLFMessageTypeBaseVelocity,
		CLFMessageTypeOld,
		CLFMessageTypeLaser3,
		CLFMessageTypeLaser4,
		CLFMessageTypeRemissionFLaser,
		CLFMessageTypeRemissionRLaser,
		CLFMessageTypeRemissionLaser3,
		CLFMessageTypeRemissionLaser4:
		return nil, errors.Errorf("reading a %q is not yet implemented", parts[0])
	default:
		return nil, errors.Errorf("unknown message type %q", parts[0])
	}
}

func (r *CLFReader) readLine() (string, bool, error) {
	for {
		line, err := r.reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			return "", true, nil
		} else if err != nil {
			return "", false, err
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		return line, false, nil
	}
}

// CLFBaseMessage is used by all messages and contains basic
// information about the message.
type CLFBaseMessage struct {
	MessageType     CLFMessageType
	IPCTimestamp    float64
	IPCHostname     string
	LoggerTimestamp float64
}

// Base returns the base part of the message.
func (b CLFBaseMessage) Base() CLFBaseMessage {
	return b
}

// Type returns the type of message this is.
func (b CLFBaseMessage) Type() CLFMessageType {
	return b.MessageType
}

// CLFParamMessage conveys parameters being set for the whole CLF.
type CLFParamMessage struct {
	CLFMessage
	Name, Value string
}

func parseBaseMessage(messageType CLFMessageType, parts []string) (CLFBaseMessage, error) {
	if len(parts) < 2 {
		return CLFBaseMessage{}, errors.New("malformed message; expected timestamp/host info at end")
	}
	if len(parts) == 2 {
		// some weird unaccepted format that we will accept (see artifact_data/aces_samples.clf)
		loggerTimestamp, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return CLFBaseMessage{}, errors.Wrap(err, "error parsing logger_timestamp")
		}
		return CLFBaseMessage{
			IPCHostname:     parts[0],
			LoggerTimestamp: loggerTimestamp,
		}, nil
	}
	ipcTimestamp, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return CLFBaseMessage{}, errors.Wrap(err, "error parsing ipc_timestamp")
	}
	loggerTimestamp, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return CLFBaseMessage{}, errors.Wrap(err, "error parsing logger_timestamp")
	}
	return CLFBaseMessage{
		MessageType:     messageType,
		IPCTimestamp:    ipcTimestamp,
		IPCHostname:     parts[1],
		LoggerTimestamp: loggerTimestamp,
	}, nil
}

func makeFieldError(typeName CLFMessageType, numFields int) error {
	return errors.Errorf("expected %q to have %d fields", typeName, numFields)
}

func parseCLFParamMessage(parts []string) (*CLFParamMessage, error) {
	messageType := CLFMessageTypeParam
	numFields := 2
	if len(parts) < numFields {
		return nil, makeFieldError(messageType, numFields)
	}
	bm, err := parseBaseMessage(messageType, parts[numFields:])
	if err != nil {
		return nil, err
	}
	return &CLFParamMessage{
		CLFMessage: bm,
		Name:       parts[0],
		Value:      parts[1],
	}, nil
}

// CLFOdometryMessage represents odometry data.
type CLFOdometryMessage struct {
	CLFMessage
	X                     float64
	Y                     float64
	Theta                 float64
	TranslationalVelocity float64
	RotationalVelocity    float64
	Acceleration          float64
}

func parseCLFPOdometryMessage(parts []string) (*CLFOdometryMessage, error) {
	messageType := CLFMessageTypeOdometry
	numFields := 6
	if len(parts) < numFields {
		return nil, makeFieldError(messageType, numFields)
	}
	bm, err := parseBaseMessage(messageType, parts[numFields:])
	if err != nil {
		return nil, err
	}
	x, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing x")
	}
	y, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing y")
	}
	theta, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing theta")
	}
	tv, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing tv")
	}
	rv, err := strconv.ParseFloat(parts[4], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing rv")
	}
	accel, err := strconv.ParseFloat(parts[5], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing accel")
	}
	return &CLFOdometryMessage{
		CLFMessage:            bm,
		X:                     x,
		Y:                     y,
		Theta:                 theta,
		TranslationalVelocity: tv,
		RotationalVelocity:    rv,
		Acceleration:          accel,
	}, nil
}

// CLFOldLaserMessage represents legacy lidar scan data.
type CLFOldLaserMessage struct {
	CLFMessage
	RangeReadings []float64
	X             float64
	Y             float64
	Theta         float64
	OdomX         float64
	OdomY         float64
	OdomTheta     float64
}

func parseCLFOldLaserMessage(messageType CLFMessageType, parts []string) (*CLFOldLaserMessage, error) {
	numFields := 8
	if len(parts) < numFields {
		return nil, makeFieldError(messageType, numFields)
	}
	bm, err := parseBaseMessage(messageType, parts[len(parts)-3:])
	if err != nil {
		return nil, err
	}
	numReadings, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing num_readings")
	}
	numFields = int(numReadings) + 7
	if len(parts) < numFields {
		return nil, makeFieldError(messageType, numFields)
	}
	var readings []float64 // untrusted, don't allocate in advance
	for i := 0; i < int(numReadings); i++ {
		reading, err := strconv.ParseFloat(parts[1+i], 64)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing range_readings")
		}
		readings = append(readings, reading)
	}
	after := 1 + numReadings
	x, err := strconv.ParseFloat(parts[after], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing x")
	}
	y, err := strconv.ParseFloat(parts[after+1], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing y")
	}
	theta, err := strconv.ParseFloat(parts[after+2], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing theta")
	}
	odomX, err := strconv.ParseFloat(parts[after+3], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing odom_x")
	}
	odomY, err := strconv.ParseFloat(parts[after+4], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing odom_y")
	}
	odomTheta, err := strconv.ParseFloat(parts[after+5], 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing odom_theta")
	}
	return &CLFOldLaserMessage{
		CLFMessage:    bm,
		RangeReadings: readings,
		X:             x,
		Y:             y,
		Theta:         theta,
		OdomX:         odomX,
		OdomY:         odomY,
		OdomTheta:     odomTheta,
	}, nil
}
