// Package ros implements functionality that bridges the gap between `rdk` and ROS
package ros

// TimeStamp contains the timestamp expressed as:
// * TimeStamp.Secs: seconds since epoch
// * TimeStamp.Nsecs: nanoseconds since TimeStamp.Secs.
type TimeStamp struct {
	Secs  int
	Nsecs int
}

// MultiArrayDimension is a ROS std_msgs/MultiArrayDimension message.
type MultiArrayDimension struct {
	Label  string
	Size   uint32
	Stride uint32
}

// MultiArrayLayout is a ROS std_msgs/MultiArrayLayout message.
type MultiArrayLayout struct {
	Dim        []MultiArrayDimension
	DataOffset int `json:"data_offset"`
}

// ByteMultiArray is a ROS std_msgs/ByteMultiArray message.
type ByteMultiArray struct {
	Layout MultiArrayLayout
	Data   []byte
}

// MessageHeader is a ROS std_msgs/Header message.
type MessageHeader struct {
	Seq     int
	Stamp   TimeStamp
	FrameID string `json:"frame_id"`
}

// Quaternion is a ROS geometry_msgs/Quaternion message.
type Quaternion struct {
	X float64
	Y float64
	Z float64
	W float64
}

// Vector3 is a ROS geometry_msgs/Vector3 message.
type Vector3 struct {
	X float64
	Y float64
	Z float64
}

// L515Message reflects the JSON data format for rosbag Intel Realsense data.
type L515Message struct {
	Meta      TimeStamp
	ColorData ByteMultiArray
	DepthData ByteMultiArray
}

// ImuData contains the IMU data.
type ImuData struct {
	Header                       MessageHeader
	Orientation                  Quaternion
	OrientationCovariance        [9]int  `json:"orientation_covariance"`
	AngularVelocity              Vector3 `json:"angular_velocity"`
	AngularVelocityCovariance    [9]int  `json:"angular_velocity_covariance"`
	LinearAcceleration           Vector3 `json:"linear_acceleration"`
	LinearAccelerationCovariance [9]int  `json:"linear_acceleration_covariance"`
}

// ImuMessage reflects the JSON data format for rosbag imu data.
type ImuMessage struct {
	Meta TimeStamp
	Data ImuData
}
